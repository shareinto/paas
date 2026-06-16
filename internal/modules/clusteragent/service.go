package clusteragent

import (
	"context"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo         Repository
	tenants      TenantQuery
	permission   PermissionChecker
	runtime      RuntimeGateway
	stages       StageClusterResolver
	stageUpdater StageStateUpdater
	deployments  DeploymentStatusUpdater
	audit        AuditLogger
	ids          shared.IDGenerator
	clock        shared.Clock
	timeout      time.Duration
}

type Options struct {
	Repository        Repository
	TenantQuery       TenantQuery
	PermissionChecker PermissionChecker
	RuntimeGateway    RuntimeGateway
	StageClusters     StageClusterResolver
	StageState        StageStateUpdater
	DeploymentStatus  DeploymentStatusUpdater
	Audit             AuditLogger
	IDGenerator       shared.IDGenerator
	Clock             shared.Clock
	HeartbeatTimeout  time.Duration
}

func NewService(opts Options) *Service {
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	timeout := opts.HeartbeatTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	return &Service{repo: opts.Repository, tenants: opts.TenantQuery, permission: opts.PermissionChecker, runtime: opts.RuntimeGateway, stages: opts.StageClusters, stageUpdater: opts.StageState, deployments: opts.DeploymentStatus, audit: audit, ids: ids, clock: clock, timeout: timeout}
}

type RegisterClusterInput struct {
	Actor    identityaccess.Subject `json:"actor"`
	TenantID shared.ID              `json:"tenant_id"`
	Name     string                 `json:"name"`
	Region   string                 `json:"region"`
	Labels   map[string]string      `json:"labels"`
}

type RegisterClusterResult struct {
	Cluster    Cluster `json:"cluster"`
	AgentToken string  `json:"agent_token"`
}

type RuntimeResourceQuery struct {
	Actor         identityaccess.Subject `json:"actor"`
	ApplicationID shared.ID              `json:"application_id"`
	StageKey      string                 `json:"stage_key"`
	ResourceID    shared.ID              `json:"resource_id,omitempty"`
}

type RuntimeResourceActionInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	ApplicationID shared.ID              `json:"application_id"`
	StageKey      string                 `json:"stage_key"`
	ResourceID    shared.ID              `json:"resource_id"`
	Namespace     string                 `json:"namespace,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Container     string                 `json:"container,omitempty"`
}

type RuntimeCapabilityResponse struct {
	Capability string    `json:"capability"`
	Supported  bool      `json:"supported"`
	ResourceID shared.ID `json:"resource_id"`
	Message    string    `json:"message"`
}

type RuntimeStreamWriter interface {
	WriteRuntimeSnapshot(resources []RuntimeResource) error
	WriteRuntimeStatus(status string) error
}

func (s *Service) RegisterCluster(ctx context.Context, input RegisterClusterInput) (RegisterClusterResult, error) {
	if input.TenantID.IsZero() {
		return RegisterClusterResult{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id is required")
	}
	if s.tenants != nil {
		if _, err := s.tenants.GetTenant(ctx, input.TenantID); err != nil {
			return RegisterClusterResult{}, err
		}
	}
	if err := s.check(ctx, input.Actor, input.TenantID, "cluster:manage"); err != nil {
		return RegisterClusterResult{}, err
	}
	id, err := s.ids.NewID("cluster")
	if err != nil {
		return RegisterClusterResult{}, err
	}
	token, err := newAgentToken()
	if err != nil {
		return RegisterClusterResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return RegisterClusterResult{}, err
	}
	now := s.clock.Now()
	cluster, err := normalizeCluster(Cluster{ID: id, TenantID: input.TenantID, Name: input.Name, Region: input.Region, Labels: input.Labels, Status: ClusterReady, AgentTokenHash: string(hash), CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return RegisterClusterResult{}, err
	}
	if err := s.repo.CreateCluster(ctx, cluster); err != nil {
		return RegisterClusterResult{}, err
	}
	cluster.AgentTokenHash = ""
	return RegisterClusterResult{Cluster: cluster, AgentToken: token}, nil
}

func (s *Service) RotateAgentToken(ctx context.Context, actor identityaccess.Subject, clusterID shared.ID) (string, error) {
	cluster, err := s.repo.GetCluster(ctx, clusterID)
	if err != nil {
		return "", err
	}
	if err := s.check(ctx, actor, cluster.TenantID, "cluster:manage"); err != nil {
		return "", err
	}
	token, err := newAgentToken()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	cluster.AgentTokenHash = string(hash)
	cluster.UpdatedAt = s.clock.Now()
	return token, s.repo.UpdateCluster(ctx, cluster)
}

func (s *Service) Authenticate(ctx context.Context, clusterID shared.ID, token string) (Cluster, error) {
	cluster, err := s.repo.GetCluster(ctx, clusterID)
	if err != nil {
		return Cluster{}, err
	}
	if cluster.Status == ClusterDisabled {
		return Cluster{}, shared.NewError(shared.CodePermissionDenied, "cluster is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cluster.AgentTokenHash), []byte(token)); err != nil {
		return Cluster{}, shared.NewError(shared.CodeUnauthenticated, "invalid agent token")
	}
	return cluster, nil
}

func (s *Service) UpdateClusterStatus(ctx context.Context, actor identityaccess.Subject, id shared.ID, status ClusterStatus) (Cluster, error) {
	cluster, err := s.repo.GetCluster(ctx, id)
	if err != nil {
		return Cluster{}, err
	}
	if err := s.check(ctx, actor, cluster.TenantID, "cluster:manage"); err != nil {
		return Cluster{}, err
	}
	if status != ClusterReady && status != ClusterDegraded && status != ClusterDraining && status != ClusterDisabled && status != ClusterUnreachable {
		return Cluster{}, shared.NewError(shared.CodeInvalidArgument, "unsupported cluster status")
	}
	cluster.Status = status
	cluster.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateCluster(ctx, cluster); err != nil {
		return Cluster{}, err
	}
	if status == ClusterDisabled || status == ClusterDraining {
		action := "cluster.draining"
		summary := "标记集群进入排空"
		if status == ClusterDisabled {
			action = "cluster.disable"
			summary = "禁用集群"
		}
		_ = s.audit.Log(ctx, AuditEvent{TenantID: cluster.TenantID, Action: action, ResourceType: "cluster", ResourceID: cluster.ID, Result: "succeeded", Summary: summary, OccurredAt: cluster.UpdatedAt})
	}
	return cluster, nil
}

func (s *Service) ListClusters(ctx context.Context, actor identityaccess.Subject, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	if tenantID.IsZero() {
		return shared.PageResult[Cluster]{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id is required")
	}
	if s.tenants != nil {
		if _, err := s.tenants.GetTenant(ctx, tenantID); err != nil {
			return shared.PageResult[Cluster]{}, err
		}
	}
	if err := s.check(ctx, actor, tenantID, "cluster:read"); err != nil {
		return shared.PageResult[Cluster]{}, err
	}
	result, err := s.repo.ListClustersByTenant(ctx, tenantID, page)
	if err != nil {
		return result, err
	}
	for i := range result.Items {
		result.Items[i].AgentTokenHash = ""
	}
	return result, nil
}

func (s *Service) GetCluster(ctx context.Context, id shared.ID) (Cluster, error) {
	cluster, err := s.repo.GetCluster(ctx, id)
	if err != nil {
		return Cluster{}, err
	}
	cluster.AgentTokenHash = ""
	return cluster, nil
}

func (s *Service) check(ctx context.Context, actor identityaccess.Subject, tenantID shared.ID, action identityaccess.Permission) error {
	if s.permission == nil {
		return nil
	}
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	return s.permission.Check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeTenant, TenantID: tenantID}, action)
}

func (s *Service) Heartbeat(ctx context.Context, clusterID shared.ID, token string, heartbeat ClusterHeartbeat) error {
	cluster, err := s.Authenticate(ctx, clusterID, token)
	if err != nil {
		return err
	}
	id, err := s.ids.NewID("heartbeat")
	if err != nil {
		return err
	}
	now := s.clock.Now()
	heartbeat.ID = id
	heartbeat.ClusterID = cluster.ID
	if heartbeat.ObservedAt.IsZero() {
		heartbeat.ObservedAt = now
	}
	if err := s.repo.CreateHeartbeat(ctx, heartbeat); err != nil {
		return err
	}
	cluster.LastHeartbeatAt = &now
	if cluster.Status == ClusterUnreachable {
		cluster.Status = ClusterReady
	}
	cluster.UpdatedAt = now
	return s.repo.UpdateCluster(ctx, cluster)
}

func (s *Service) ReportStatus(ctx context.Context, clusterID shared.ID, token string, report StatusReport) error {
	cluster, err := s.Authenticate(ctx, clusterID, token)
	if err != nil {
		return err
	}
	if report.ClusterID != "" && report.ClusterID != cluster.ID {
		return shared.NewError(shared.CodePermissionDenied, "agent cannot report another cluster")
	}
	report.ClusterID = cluster.ID
	if report.ReportedAt.IsZero() {
		report.ReportedAt = s.clock.Now()
	}
	id, err := s.ids.NewID("snapshot")
	if err != nil {
		return err
	}
	if err := s.repo.CreateSnapshot(ctx, ClusterResourceSnapshot{ID: id, ClusterID: cluster.ID, TenantID: cluster.TenantID, Payload: report, ReportedAt: report.ReportedAt}); err != nil {
		return err
	}
	if s.runtime == nil {
		if err := s.repo.ReplaceRuntimeResources(ctx, cluster.ID, cluster.TenantID, report.ReportedAt, report.RuntimeResources); err != nil {
			return err
		}
	}
	if s.stageUpdater != nil {
		if err := s.stageUpdater.UpdateFromAgent(ctx, report); err != nil {
			return err
		}
	}
	if s.deployments != nil {
		if err := s.deployments.UpdateFromAgent(ctx, report); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListRuntimeResources(ctx context.Context, query RuntimeResourceQuery) ([]RuntimeResource, error) {
	stage, err := s.resolveRuntimeStage(ctx, query.Actor, query.ApplicationID, query.StageKey, "runtime:read")
	if err != nil {
		return nil, err
	}
	if s.runtime != nil {
		resources, err := s.runtime.ListRuntimeResources(ctx, stage.ClusterID, query.ApplicationID, query.StageKey)
		if err != nil {
			return nil, err
		}
		enrichRuntimeResources(resources, stage)
		return resources, nil
	}
	return s.repo.ListRuntimeResources(ctx, RuntimeResourceFilter{ApplicationID: query.ApplicationID, StageKey: query.StageKey})
}

func (s *Service) GetRuntimeResource(ctx context.Context, query RuntimeResourceQuery) (RuntimeResource, error) {
	resources, err := s.ListRuntimeResources(ctx, query)
	if err != nil {
		return RuntimeResource{}, err
	}
	for _, resource := range resources {
		if resource.ID == query.ResourceID {
			return resource, nil
		}
	}
	return RuntimeResource{}, shared.NewError(shared.CodeNotFound, "runtime resource not found")
}

func (s *Service) WatchRuntimeResources(ctx context.Context, query RuntimeResourceQuery, writer RuntimeStreamWriter) error {
	if writer == nil {
		return shared.NewError(shared.CodeInvalidArgument, "runtime stream writer is required")
	}
	stage, err := s.resolveRuntimeStage(ctx, query.Actor, query.ApplicationID, query.StageKey, "runtime:read")
	if err != nil {
		return err
	}
	if s.runtime == nil {
		resources, err := s.repo.ListRuntimeResources(ctx, RuntimeResourceFilter{ApplicationID: query.ApplicationID, StageKey: query.StageKey})
		if err != nil {
			return err
		}
		return writer.WriteRuntimeSnapshot(resources)
	}
	return s.runtime.WatchRuntimeResources(ctx, stage.ClusterID, query.ApplicationID, query.StageKey, func(resources []RuntimeResource) error {
		enrichRuntimeResources(resources, stage)
		return writer.WriteRuntimeSnapshot(resources)
	}, func(status string) error {
		return writer.WriteRuntimeStatus(status)
	})
}

func (s *Service) RestartRuntimeResource(ctx context.Context, input RuntimeResourceActionInput) (ClusterTask, error) {
	resource, err := s.runtimeActionResource(ctx, input)
	if err != nil {
		return ClusterTask{}, err
	}
	if !isRestartableRuntimeKind(resource.Kind) {
		return ClusterTask{}, shared.NewError(shared.CodeFailedPrecondition, "runtime resource does not support restart")
	}
	if err := s.check(ctx, input.Actor, resource.TenantID, "runtime:restart"); err != nil {
		return ClusterTask{}, err
	}
	if s.runtime != nil {
		if err := s.runtime.RestartRuntimeResource(ctx, runtimeTargetFromResource(resource)); err != nil {
			return ClusterTask{}, err
		}
		now := s.clock.Now()
		task := ClusterTask{ID: stableRuntimeActionID("runtime_restart", resource), ClusterID: resource.ClusterID, Type: "runtime_restart", TargetRef: resource.Kind + "/" + resource.Namespace + "/" + resource.Name, Payload: map[string]string{"application_id": string(resource.ApplicationID), "stage_key": resource.StageKey, "kind": resource.Kind, "namespace": resource.Namespace, "name": resource.Name}, Status: ClusterTaskSucceeded, CreatedAt: now, UpdatedAt: now, CompletedAt: &now, ResultMessage: "实时重启请求已发送"}
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, TenantID: resource.TenantID, Action: "runtime.restart", ResourceType: "runtime_resource", ResourceID: resource.ID, Result: "succeeded", Summary: "执行运行时重启", OccurredAt: now})
		return task, nil
	}
	task := ClusterTask{
		ClusterID: resource.ClusterID,
		Type:      "runtime_restart",
		TargetRef: resource.Kind + "/" + resource.Namespace + "/" + resource.Name,
		Payload: map[string]string{
			"application_id": string(resource.ApplicationID),
			"stage_key":      resource.StageKey,
			"kind":           resource.Kind,
			"namespace":      resource.Namespace,
			"name":           resource.Name,
		},
	}
	created, err := s.CreateTask(ctx, task)
	if err != nil {
		return ClusterTask{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, TenantID: resource.TenantID, Action: "runtime.restart", ResourceType: "runtime_resource", ResourceID: resource.ID, Result: "succeeded", Summary: "创建运行时重启任务", OccurredAt: created.CreatedAt})
	return created, nil
}

func (s *Service) GetPodLogs(ctx context.Context, input RuntimeResourceActionInput) (RuntimeCapabilityResponse, error) {
	resource, err := s.runtimeActionResource(ctx, input)
	if err != nil {
		return RuntimeCapabilityResponse{}, err
	}
	if resource.Kind != "Pod" {
		return RuntimeCapabilityResponse{}, shared.NewError(shared.CodeFailedPrecondition, "logs are only available for pods")
	}
	if err := s.check(ctx, input.Actor, resource.TenantID, "runtime:read"); err != nil {
		return RuntimeCapabilityResponse{}, err
	}
	return RuntimeCapabilityResponse{Capability: "pod_logs", Supported: s.runtime != nil, ResourceID: resource.ID, Message: ""}, nil
}

func (s *Service) StreamPodLogs(ctx context.Context, input RuntimeResourceActionInput, writer interface{ Write([]byte) (int, error) }) error {
	resource, err := s.runtimeActionResource(ctx, input)
	if err != nil {
		return err
	}
	if resource.Kind != "Pod" {
		return shared.NewError(shared.CodeFailedPrecondition, "logs are only available for pods")
	}
	if err := s.check(ctx, input.Actor, resource.TenantID, "runtime:read"); err != nil {
		return err
	}
	if s.runtime == nil {
		return shared.NewError(shared.CodeUnavailable, "agent_offline")
	}
	return s.runtime.StreamPodLogs(ctx, runtimeTargetFromResource(resource), RuntimeLogOptions{Container: strings.TrimSpace(input.Container), TailLines: 500, Follow: true}, writer)
}

func (s *Service) OpenTerminal(ctx context.Context, input RuntimeResourceActionInput) (RuntimeCapabilityResponse, error) {
	resource, err := s.runtimeActionResource(ctx, input)
	if err != nil {
		return RuntimeCapabilityResponse{}, err
	}
	if resource.Kind != "Pod" {
		return RuntimeCapabilityResponse{}, shared.NewError(shared.CodeFailedPrecondition, "terminal is only available for pods")
	}
	if err := s.check(ctx, input.Actor, resource.TenantID, "runtime:terminal"); err != nil {
		return RuntimeCapabilityResponse{}, err
	}
	return RuntimeCapabilityResponse{Capability: "pod_terminal", Supported: s.runtime != nil, ResourceID: resource.ID, Message: ""}, nil
}

func (s *Service) Terminal(ctx context.Context, input RuntimeResourceActionInput, in <-chan []byte, out chan<- []byte) error {
	resource, err := s.runtimeActionResource(ctx, input)
	if err != nil {
		return err
	}
	if resource.Kind != "Pod" {
		return shared.NewError(shared.CodeFailedPrecondition, "terminal is only available for pods")
	}
	if err := s.check(ctx, input.Actor, resource.TenantID, "runtime:terminal"); err != nil {
		return err
	}
	if s.runtime == nil {
		return shared.NewError(shared.CodeUnavailable, "agent_offline")
	}
	return s.runtime.Terminal(ctx, runtimeTargetFromResource(resource), RuntimeTerminalOptions{Container: strings.TrimSpace(input.Container), Command: "/bin/sh"}, in, out)
}

func (s *Service) runtimeActionResource(ctx context.Context, input RuntimeResourceActionInput) (RuntimeResource, error) {
	resources, err := s.ListRuntimeResources(ctx, RuntimeResourceQuery{Actor: input.Actor, ApplicationID: input.ApplicationID, StageKey: input.StageKey})
	if err != nil {
		return RuntimeResource{}, err
	}
	for _, resource := range resources {
		if input.ResourceID != "" && resource.ID == input.ResourceID {
			return resource, nil
		}
		if input.ResourceID == "" && resource.Kind == "Pod" && resource.Namespace == strings.TrimSpace(input.Namespace) && resource.Name == strings.TrimSpace(input.Name) {
			return resource, nil
		}
	}
	return RuntimeResource{}, shared.NewError(shared.CodeNotFound, "runtime resource not found")
}

func isRestartableRuntimeKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return true
	default:
		return false
	}
}

func (s *Service) resolveRuntimeStage(ctx context.Context, actor identityaccess.Subject, applicationID shared.ID, stageKey string, action identityaccess.Permission) (StageClusterRef, error) {
	if applicationID.IsZero() {
		return StageClusterRef{}, shared.NewError(shared.CodeInvalidArgument, "application_id is required")
	}
	stageKey = strings.TrimSpace(stageKey)
	if stageKey == "" {
		return StageClusterRef{}, shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
	}
	if s.stages == nil {
		resources, err := s.repo.ListRuntimeResources(ctx, RuntimeResourceFilter{ApplicationID: applicationID, StageKey: stageKey})
		if err != nil {
			return StageClusterRef{}, err
		}
		if len(resources) == 0 {
			return StageClusterRef{}, shared.NewError(shared.CodeNotFound, "runtime resource not found")
		}
		if err := s.check(ctx, actor, resources[0].TenantID, action); err != nil {
			return StageClusterRef{}, err
		}
		return StageClusterRef{ClusterID: resources[0].ClusterID, TenantID: resources[0].TenantID}, nil
	}
	stage, err := s.stages.ResolveStageCluster(ctx, applicationID, stageKey)
	if err != nil {
		return StageClusterRef{}, err
	}
	if stage.ClusterID.IsZero() {
		return StageClusterRef{}, shared.NewError(shared.CodeFailedPrecondition, "stage has no bound cluster")
	}
	if err := s.check(ctx, actor, stage.TenantID, action); err != nil {
		return StageClusterRef{}, err
	}
	return stage, nil
}

func runtimeTargetFromResource(resource RuntimeResource) RuntimeResourceTarget {
	return RuntimeResourceTarget{
		ClusterID: resource.ClusterID, TenantID: resource.TenantID, ApplicationID: resource.ApplicationID, StageKey: resource.StageKey,
		Group: resource.Group, Version: resource.Version, Kind: resource.Kind, Namespace: resource.Namespace, Name: resource.Name,
		ParentKind: resource.ParentKind, ParentNamespace: resource.ParentNamespace, ParentName: resource.ParentName,
	}
}

func enrichRuntimeResources(resources []RuntimeResource, stage StageClusterRef) {
	for i := range resources {
		if resources[i].ClusterID.IsZero() {
			resources[i].ClusterID = stage.ClusterID
		}
		if resources[i].TenantID.IsZero() {
			resources[i].TenantID = stage.TenantID
		}
	}
}

func stableRuntimeActionID(prefix string, resource RuntimeResource) shared.ID {
	value := strings.ToLower(strings.TrimSpace(prefix + "_" + string(resource.ID)))
	if value == "_" {
		return shared.ID(prefix)
	}
	return shared.ID(strings.ReplaceAll(value, "/", "_"))
}

func (s *Service) CreateTask(ctx context.Context, task ClusterTask) (ClusterTask, error) {
	if _, err := s.repo.GetCluster(ctx, task.ClusterID); err != nil {
		return ClusterTask{}, err
	}
	id, err := s.ids.NewID("cluster_task")
	if err != nil {
		return ClusterTask{}, err
	}
	now := s.clock.Now()
	task.ID = id
	task.Status = ClusterTaskPending
	task.CreatedAt = now
	task.UpdatedAt = now
	return task, s.repo.CreateTask(ctx, task)
}

func (s *Service) PullTasks(ctx context.Context, clusterID shared.ID, token string, limit int) ([]ClusterTask, error) {
	if _, err := s.Authenticate(ctx, clusterID, token); err != nil {
		return nil, err
	}
	tasks, err := s.repo.ListPendingTasks(ctx, clusterID, limit)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for i := range tasks {
		tasks[i].Status = ClusterTaskRunning
		tasks[i].LeasedAt = &now
		tasks[i].UpdatedAt = now
		if err := s.repo.UpdateTask(ctx, tasks[i]); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (s *Service) CompleteTask(ctx context.Context, clusterID shared.ID, token string, result ClusterTaskResult) (ClusterTask, error) {
	if _, err := s.Authenticate(ctx, clusterID, token); err != nil {
		return ClusterTask{}, err
	}
	task, err := s.repo.GetTask(ctx, result.TaskID)
	if err != nil {
		return ClusterTask{}, err
	}
	if task.ClusterID != clusterID {
		return ClusterTask{}, shared.NewError(shared.CodePermissionDenied, "agent cannot complete another cluster task")
	}
	if result.Status != ClusterTaskSucceeded && result.Status != ClusterTaskFailed && result.Status != ClusterTaskCanceled {
		return ClusterTask{}, shared.NewError(shared.CodeInvalidArgument, "invalid task result status")
	}
	id, err := s.ids.NewID("cluster_task_result")
	if err != nil {
		return ClusterTask{}, err
	}
	now := s.clock.Now()
	result.ID = id
	result.ClusterID = clusterID
	result.ReportedAt = now
	task.Status = result.Status
	task.ResultMessage = result.Message
	task.UpdatedAt = now
	task.CompletedAt = &now
	if err := s.repo.CreateTaskResult(ctx, result); err != nil {
		return ClusterTask{}, err
	}
	return task, s.repo.UpdateTask(ctx, task)
}

func (s *Service) MarkUnreachable(ctx context.Context) ([]Cluster, error) {
	result, err := s.repo.ListClusters(ctx, shared.PageRequest{Page: 1, PageSize: 10000})
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	changed := make([]Cluster, 0)
	for _, cluster := range result.Items {
		if cluster.Status == ClusterDisabled || cluster.Status == ClusterUnreachable {
			continue
		}
		if cluster.LastHeartbeatAt == nil || now.Sub(*cluster.LastHeartbeatAt) > s.timeout {
			cluster.Status = ClusterUnreachable
			cluster.UpdatedAt = now
			if err := s.repo.UpdateCluster(ctx, cluster); err != nil {
				return nil, err
			}
			_ = s.audit.Log(ctx, AuditEvent{TenantID: cluster.TenantID, Action: "cluster.unreachable", ResourceType: "cluster", ResourceID: cluster.ID, Result: "failed", Summary: "Agent 心跳超时，集群离线", OccurredAt: now})
			cluster.AgentTokenHash = ""
			changed = append(changed, cluster)
		}
	}
	return changed, nil
}
