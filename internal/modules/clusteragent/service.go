package clusteragent

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo        Repository
	tenants     TenantQuery
	permission  PermissionChecker
	envUpdater  EnvironmentStateUpdater
	deployments DeploymentStatusUpdater
	audit       AuditLogger
	ids         shared.IDGenerator
	clock       shared.Clock
	timeout     time.Duration
}

type Options struct {
	Repository        Repository
	TenantQuery       TenantQuery
	PermissionChecker PermissionChecker
	EnvironmentState  EnvironmentStateUpdater
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
	return &Service{repo: opts.Repository, tenants: opts.TenantQuery, permission: opts.PermissionChecker, envUpdater: opts.EnvironmentState, deployments: opts.DeploymentStatus, audit: audit, ids: ids, clock: clock, timeout: timeout}
}

type RegisterClusterInput struct {
	Actor    identityaccess.Subject `json:"actor"`
	TenantID shared.ID              `json:"tenant_id"`
	Name     string                 `json:"name"`
	Region   string                 `json:"region"`
}

type RegisterClusterResult struct {
	Cluster    Cluster `json:"cluster"`
	AgentToken string  `json:"agent_token"`
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
	cluster, err := normalizeCluster(Cluster{ID: id, TenantID: input.TenantID, Name: input.Name, Region: input.Region, Status: ClusterReady, AgentTokenHash: string(hash), CreatedAt: now, UpdatedAt: now})
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
	if s.envUpdater != nil {
		if err := s.envUpdater.UpdateFromAgent(ctx, report); err != nil {
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
