package clusteragent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/testsupport"
)

type staticIDs struct{ ids []shared.ID }

func (s *staticIDs) NewID(string) (shared.ID, error) {
	id := s.ids[0]
	s.ids = s.ids[1:]
	return id, nil
}

type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

type reportRecorder struct{ reports []StatusReport }

func (r *reportRecorder) UpdateFromAgent(_ context.Context, report StatusReport) error {
	r.reports = append(r.reports, report)
	return nil
}

type failingReportUpdater struct{ err error }

func (u failingReportUpdater) UpdateFromAgent(context.Context, StatusReport) error {
	return u.err
}

type fakeTenantQuery struct {
	tenants map[shared.ID]struct{}
	err     error
}

func (q fakeTenantQuery) GetTenant(_ context.Context, id shared.ID) (TenantRef, error) {
	if q.err != nil {
		return TenantRef{}, q.err
	}
	if _, ok := q.tenants[id]; !ok {
		return TenantRef{}, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	return TenantRef{ID: id}, nil
}

type recordingClusterPermission struct {
	calls []clusterPermissionCall
	err   error
}

type clusterPermissionCall struct {
	subject  identityaccess.Subject
	resource identityaccess.ResourceScope
	action   identityaccess.Permission
}

func (p *recordingClusterPermission) Check(_ context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error {
	p.calls = append(p.calls, clusterPermissionCall{subject: subject, resource: resource, action: action})
	return p.err
}

func tenantQuery(ids ...shared.ID) fakeTenantQuery {
	tenants := make(map[shared.ID]struct{}, len(ids))
	for _, id := range ids {
		tenants[id] = struct{}{}
	}
	return fakeTenantQuery{tenants: tenants}
}

func clusterActor() identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_cluster_admin"}
}

type fakeRuntimeGateway struct {
	resources []RuntimeResource
	restarted []RuntimeResourceTarget
	logs      string
}

func (g *fakeRuntimeGateway) ListRuntimeResources(_ context.Context, clusterID shared.ID, _ shared.ID, _ string) ([]RuntimeResource, error) {
	out := make([]RuntimeResource, 0, len(g.resources))
	for _, resource := range g.resources {
		resource.ClusterID = clusterID
		out = append(out, resource)
	}
	return out, nil
}

func (g *fakeRuntimeGateway) WatchRuntimeResources(_ context.Context, clusterID shared.ID, _ shared.ID, _ string, onSnapshot func([]RuntimeResource) error, onStatus func(string) error) error {
	if onStatus != nil {
		_ = onStatus("connected")
	}
	resources, _ := g.ListRuntimeResources(context.Background(), clusterID, "", "")
	return onSnapshot(resources)
}

func (g *fakeRuntimeGateway) RestartRuntimeResource(_ context.Context, target RuntimeResourceTarget) error {
	g.restarted = append(g.restarted, target)
	return nil
}

func (g *fakeRuntimeGateway) StreamPodLogs(_ context.Context, _ RuntimeResourceTarget, _ RuntimeLogOptions, writer io.Writer) error {
	_, err := writer.Write([]byte(g.logs))
	return err
}

func (g *fakeRuntimeGateway) Terminal(context.Context, RuntimeResourceTarget, RuntimeTerminalOptions, <-chan []byte, chan<- []byte) error {
	return nil
}

type fakeStageResolver struct{ ref StageClusterRef }

func (r fakeStageResolver) ResolveStageCluster(context.Context, shared.ID, string) (StageClusterRef, error) {
	return r.ref, nil
}

type clusterRepoWithErrors struct {
	Repository
	listErr             error
	createHeartbeatErr  error
	updateClusterErr    error
	listPendingTasksErr error
	createTaskResultErr error
	updateTaskErr       error
	createClusterErr    error
	createSnapshotErr   error
}

func (r *clusterRepoWithErrors) CreateCluster(ctx context.Context, cluster Cluster) error {
	if r.createClusterErr != nil {
		return r.createClusterErr
	}
	return r.Repository.CreateCluster(ctx, cluster)
}

func (r *clusterRepoWithErrors) UpdateCluster(ctx context.Context, cluster Cluster) error {
	if r.updateClusterErr != nil {
		return r.updateClusterErr
	}
	return r.Repository.UpdateCluster(ctx, cluster)
}

func (r *clusterRepoWithErrors) ListClusters(ctx context.Context, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	if r.listErr != nil {
		return shared.PageResult[Cluster]{}, r.listErr
	}
	return r.Repository.ListClusters(ctx, page)
}

func (r *clusterRepoWithErrors) ListClustersByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	if r.listErr != nil {
		return shared.PageResult[Cluster]{}, r.listErr
	}
	return r.Repository.ListClustersByTenant(ctx, tenantID, page)
}

func (r *clusterRepoWithErrors) CreateHeartbeat(ctx context.Context, heartbeat ClusterHeartbeat) error {
	if r.createHeartbeatErr != nil {
		return r.createHeartbeatErr
	}
	return r.Repository.CreateHeartbeat(ctx, heartbeat)
}

func (r *clusterRepoWithErrors) CreateSnapshot(ctx context.Context, snapshot ClusterResourceSnapshot) error {
	if r.createSnapshotErr != nil {
		return r.createSnapshotErr
	}
	return r.Repository.CreateSnapshot(ctx, snapshot)
}

func (r *clusterRepoWithErrors) UpdateTask(ctx context.Context, task ClusterTask) error {
	if r.updateTaskErr != nil {
		return r.updateTaskErr
	}
	return r.Repository.UpdateTask(ctx, task)
}

func (r *clusterRepoWithErrors) ListPendingTasks(ctx context.Context, clusterID shared.ID, limit int) ([]ClusterTask, error) {
	if r.listPendingTasksErr != nil {
		return nil, r.listPendingTasksErr
	}
	return r.Repository.ListPendingTasks(ctx, clusterID, limit)
}

func (r *clusterRepoWithErrors) CreateTaskResult(ctx context.Context, result ClusterTaskResult) error {
	if r.createTaskResultErr != nil {
		return r.createTaskResultErr
	}
	return r.Repository.CreateTaskResult(ctx, result)
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func TestAgentTokenBindingHeartbeatTimeoutAndStatusForward(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	env := &reportRecorder{}
	deployments := &reportRecorder{}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "heartbeat_1", "snapshot_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), StageState: env, DeploymentStatus: deployments, IDGenerator: ids, Clock: clock, HeartbeatTimeout: time.Minute})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "生产集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.AgentToken == "" || registered.Cluster.AgentTokenHash != "" {
		t.Fatalf("token response should include plain token and hide hash")
	}
	if err := svc.Heartbeat(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterHeartbeat{AgentVersion: "v1"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, registered.AgentToken, StatusReport{Applications: []ApplicationStatus{{ApplicationID: "app_1"}}}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(env.reports) != 1 || len(deployments.reports) != 1 {
		t.Fatalf("status report not forwarded")
	}
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, "wrong", StatusReport{}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("wrong token should be unauthenticated: %v", err)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	changed, err := svc.MarkUnreachable(context.Background())
	if err != nil {
		t.Fatalf("mark unreachable: %v", err)
	}
	if len(changed) != 1 || changed[0].Status != ClusterUnreachable {
		t.Fatalf("expected unreachable cluster: %#v", changed)
	}
}

func TestReportStatusDoesNotPersistRuntimeResources(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	reportedAt := clock.now.Add(time.Minute)
	report := StatusReport{
		RuntimeResources: []RuntimeResourceStatus{
			{
				ApplicationID: "app_1",
				StageKey:      "dev",
				Group:         "apps",
				Version:       "v1",
				Kind:          "Deployment",
				Namespace:     "order-dev",
				Name:          "order-api",
				Status:        "Progressing",
				HealthStatus:  "Progressing",
				Message:       "正在滚动更新",
				Desired:       3,
				Ready:         1,
				Containers:    []RuntimeContainerStatus{{Name: "app", Image: "registry.local/order-api:v1", Ready: true, RestartCount: 1}},
				Events:        []RuntimeResourceEvent{{Type: "Warning", Reason: "FailedPull", Message: "拉取镜像失败", Count: 2, OccurredAt: reportedAt}},
			},
			{
				ApplicationID: "app_1",
				StageKey:      "test",
				Kind:          "Pod",
				Namespace:     "order-test",
				Name:          "order-api-abc",
				Status:        "Running",
			},
		},
		ReportedAt: reportedAt,
	}
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, registered.AgentToken, report); err != nil {
		t.Fatalf("report status: %v", err)
	}
	resources, err := repo.ListRuntimeResources(context.Background(), RuntimeResourceFilter{ApplicationID: "app_1", StageKey: "dev"})
	if err != nil {
		t.Fatalf("ListRuntimeResources(repo) error = %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("ReportStatus should not persist runtime resources, got %#v", resources)
	}
}

func TestRuntimeResourcesUseRealtimeGatewayWhenConfigured(t *testing.T) {
	repo := newTestRepository(t)
	permission := &recordingClusterPermission{}
	gateway := &fakeRuntimeGateway{resources: []RuntimeResource{
		{ID: "runtime_deploy", ApplicationID: "app_1", StageKey: "dev", Kind: "Deployment", Namespace: "order-dev", Name: "order-api", Status: "Healthy"},
		{ID: "runtime_pod", ApplicationID: "app_1", StageKey: "dev", Kind: "Pod", Namespace: "order-dev", Name: "order-api-abc", Status: "Running", Containers: []RuntimeContainerStatus{{Name: "app", Ready: true}}},
	}}
	svc := NewService(Options{
		Repository:        repo,
		PermissionChecker: permission,
		RuntimeGateway:    gateway,
		StageClusters:     fakeStageResolver{ref: StageClusterRef{ClusterID: "cluster_1", TenantID: "tenant_1"}},
		IDGenerator:       &staticIDs{ids: []shared.ID{"cluster_task_unused"}},
		Clock:             &mutableClock{now: time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)},
	})
	resources, err := svc.ListRuntimeResources(context.Background(), RuntimeResourceQuery{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_runtime"}, ApplicationID: "app_1", StageKey: "dev"})
	if err != nil {
		t.Fatalf("ListRuntimeResources() error = %v", err)
	}
	if len(resources) != 2 || resources[0].TenantID != "tenant_1" || resources[0].ClusterID != "cluster_1" {
		t.Fatalf("runtime gateway resources should be enriched with stage scope, got %+v", resources)
	}
	task, err := svc.RestartRuntimeResource(context.Background(), RuntimeResourceActionInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_operator"}, ApplicationID: "app_1", StageKey: "dev", ResourceID: "runtime_deploy"})
	if err != nil {
		t.Fatalf("RestartRuntimeResource() error = %v", err)
	}
	if task.Status != ClusterTaskSucceeded || len(gateway.restarted) != 1 || gateway.restarted[0].Kind != "Deployment" {
		t.Fatalf("restart should execute through runtime gateway, task=%+v restarted=%+v", task, gateway.restarted)
	}
	if permission.calls[len(permission.calls)-1].action != "runtime:restart" {
		t.Fatalf("restart should check runtime permission, calls=%+v", permission.calls)
	}
}

func TestRuntimeActionsLogsAndTerminalGates(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "cluster_task_1"}}
	permission := &recordingClusterPermission{}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), PermissionChecker: permission, IDGenerator: ids, Clock: clock})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	runtimeResources := []RuntimeResourceStatus{
		{ApplicationID: "app_1", StageKey: "dev", Group: "apps", Kind: "Deployment", Namespace: "order-dev", Name: "order-api", Status: "Healthy"},
		{ApplicationID: "app_1", StageKey: "dev", Kind: "Pod", Namespace: "order-dev", Name: "order-api-abc", Status: "Running", Containers: []RuntimeContainerStatus{{Name: "app", Ready: true}}},
	}
	if err := repo.ReplaceRuntimeResources(context.Background(), registered.Cluster.ID, "tenant_1", clock.now, runtimeResources); err != nil {
		t.Fatalf("seed runtime resources: %v", err)
	}
	resources, err := svc.ListRuntimeResources(context.Background(), RuntimeResourceQuery{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_runtime"}, ApplicationID: "app_1", StageKey: "dev"})
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	var deployment, pod RuntimeResource
	for _, resource := range resources {
		if resource.Kind == "Deployment" {
			deployment = resource
		}
		if resource.Kind == "Pod" {
			pod = resource
		}
	}
	task, err := svc.RestartRuntimeResource(context.Background(), RuntimeResourceActionInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_operator"}, ApplicationID: "app_1", StageKey: "dev", ResourceID: deployment.ID})
	if err != nil {
		t.Fatalf("RestartRuntimeResource() error = %v", err)
	}
	if task.Type != "runtime_restart" || task.ClusterID != registered.Cluster.ID || task.Payload["kind"] != "Deployment" || task.Payload["namespace"] != "order-dev" || task.Payload["name"] != "order-api" {
		t.Fatalf("unexpected restart task: %#v", task)
	}
	if got, err := repo.GetTask(context.Background(), task.ID); err != nil || got.Payload["stage_key"] != "dev" {
		t.Fatalf("restart task should be stored with audit-safe payload, got %#v err=%v", got, err)
	}
	if permission.calls[len(permission.calls)-1].action != "runtime:restart" {
		t.Fatalf("restart should check runtime:restart, calls=%+v", permission.calls)
	}
	logs, err := svc.GetPodLogs(context.Background(), RuntimeResourceActionInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_runtime"}, ApplicationID: "app_1", StageKey: "dev", ResourceID: pod.ID, Container: "app"})
	if err != nil {
		t.Fatalf("GetPodLogs() error = %v", err)
	}
	if logs.Capability != "pod_logs" || logs.Supported || logs.ResourceID != pod.ID {
		t.Fatalf("logs should return authorized unsupported capability response, got %#v", logs)
	}
	permission.err = shared.NewError(shared.CodePermissionDenied, "permission denied")
	if _, err := svc.OpenTerminal(context.Background(), RuntimeResourceActionInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_dev"}, ApplicationID: "app_1", StageKey: "dev", ResourceID: pod.ID, Container: "app"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("terminal should be permission-gated, got %v", err)
	}
}

func TestTaskPullAndResultAreScopedToCluster(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "cluster_task_1", "cluster_task_result_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "测试集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	task, err := svc.CreateTask(context.Background(), ClusterTask{ClusterID: registered.Cluster.ID, Type: "argocd_sync", TargetRef: "app-dev"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	tasks, err := svc.PullTasks(context.Background(), registered.Cluster.ID, registered.AgentToken, 10)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Status != ClusterTaskRunning {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	completed, err := svc.CompleteTask(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterTaskResult{TaskID: task.ID, Status: ClusterTaskSucceeded, Message: "ok"})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != ClusterTaskSucceeded || completed.CompletedAt == nil {
		t.Fatalf("unexpected completed task: %#v", completed)
	}
}

func TestClusterAgentHTTPHandlerCoversControlAndAgentAPIs(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "cluster_task_1", "heartbeat_1", "cluster_task_result_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	register := httptest.NewRecorder()
	mux.ServeHTTP(register, httptest.NewRequest(http.MethodPost, "/api/clusters", bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_cluster_admin"},"tenant_id":"tenant_1","name":"生产集群","region":"cn","labels":{"cloud":"aliyun"}}`)))
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", register.Code, register.Body.String())
	}
	var registered RegisterClusterResult
	if err := json.Unmarshal(register.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if registered.AgentToken == "" || registered.Cluster.AgentTokenHash != "" {
		t.Fatalf("token should be returned once and hash hidden: %#v", registered)
	}
	if registered.Cluster.Labels["cloud"] != "aliyun" {
		t.Fatalf("registered cluster should keep labels: %#v", registered.Cluster)
	}
	rotate := httptest.NewRecorder()
	mux.ServeHTTP(rotate, httptest.NewRequest(http.MethodPost, "/api/clusters/"+registered.Cluster.ID.String()+"/rotate-token", bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_cluster_admin"}}`)))
	if rotate.Code != http.StatusOK || bytes.Contains(rotate.Body.Bytes(), []byte("AgentTokenHash")) {
		t.Fatalf("rotate status=%d body=%s", rotate.Code, rotate.Body.String())
	}
	var rotated struct {
		AgentToken string `json:"agent_token"`
	}
	if err := json.Unmarshal(rotate.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotated token: %v", err)
	}
	if rotated.AgentToken == "" || rotated.AgentToken == registered.AgentToken {
		t.Fatalf("rotated token should be returned once and changed")
	}
	registered.AgentToken = rotated.AgentToken
	if _, err := svc.CreateTask(context.Background(), ClusterTask{ClusterID: registered.Cluster.ID, Type: "argocd_refresh", TargetRef: "order-dev"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	headers := func(req *http.Request) *http.Request {
		req.Header.Set("X-PaaS-Cluster-ID", registered.Cluster.ID.String())
		req.Header.Set("Authorization", "Bearer "+registered.AgentToken)
		return req
	}
	heartbeat := httptest.NewRecorder()
	mux.ServeHTTP(heartbeat, headers(httptest.NewRequest(http.MethodPost, "/api/agent/v1/heartbeat", bytes.NewBufferString(`{"agent_version":"v1"}`))))
	if heartbeat.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", heartbeat.Code, heartbeat.Body.String())
	}
	status := httptest.NewRecorder()
	mux.ServeHTTP(status, headers(httptest.NewRequest(http.MethodPost, "/api/agent/v1/status/report", bytes.NewBufferString(`{"applications":[{"application_id":"app_1"}]}`))))
	if status.Code != http.StatusOK {
		t.Fatalf("status report = %d body=%s", status.Code, status.Body.String())
	}
	runtimeStatus := httptest.NewRecorder()
	mux.ServeHTTP(runtimeStatus, headers(httptest.NewRequest(http.MethodPost, "/api/agent/v1/status/report", bytes.NewBufferString(`{"runtime_resources":[{"application_id":"app_1","stage_key":"dev","kind":"Deployment","namespace":"order-dev","name":"order-api","status":"Healthy","ready":2,"desired":2,"containers":[{"name":"app","image":"registry.local/order-api:v1","ready":true}]}]}`))))
	if runtimeStatus.Code != http.StatusOK {
		t.Fatalf("runtime status report = %d body=%s", runtimeStatus.Code, runtimeStatus.Body.String())
	}
	if err := repo.ReplaceRuntimeResources(context.Background(), registered.Cluster.ID, "tenant_1", clock.now, []RuntimeResourceStatus{{ApplicationID: "app_1", StageKey: "dev", Kind: "Deployment", Namespace: "order-dev", Name: "order-api", Status: "Healthy", Ready: 2, Desired: 2, Containers: []RuntimeContainerStatus{{Name: "app", Image: "registry.local/order-api:v1", Ready: true}}}}); err != nil {
		t.Fatalf("seed runtime resources: %v", err)
	}
	runtimeList := httptest.NewRecorder()
	mux.ServeHTTP(runtimeList, httptest.NewRequest(http.MethodGet, "/api/apps/app_1/stages/dev/runtime/resources?actor_id=usr_runtime", nil))
	if runtimeList.Code != http.StatusOK || !bytes.Contains(runtimeList.Body.Bytes(), []byte(`"kind":"Deployment"`)) || bytes.Contains(runtimeList.Body.Bytes(), []byte(registered.AgentToken)) {
		t.Fatalf("runtime list status=%d body=%s", runtimeList.Code, runtimeList.Body.String())
	}
	var runtimePayload struct {
		Items []RuntimeResource `json:"items"`
	}
	if err := json.Unmarshal(runtimeList.Body.Bytes(), &runtimePayload); err != nil {
		t.Fatalf("decode runtime list: %v", err)
	}
	if len(runtimePayload.Items) != 1 || runtimePayload.Items[0].ID == "" {
		t.Fatalf("unexpected runtime list: %#v", runtimePayload)
	}
	runtimeDetail := httptest.NewRecorder()
	mux.ServeHTTP(runtimeDetail, httptest.NewRequest(http.MethodGet, "/api/apps/app_1/stages/dev/runtime/resources/"+runtimePayload.Items[0].ID.String()+"?actor_id=usr_runtime", nil))
	if runtimeDetail.Code != http.StatusOK || !bytes.Contains(runtimeDetail.Body.Bytes(), []byte(`"name":"order-api"`)) {
		t.Fatalf("runtime detail status=%d body=%s", runtimeDetail.Code, runtimeDetail.Body.String())
	}
	events := httptest.NewRecorder()
	mux.ServeHTTP(events, headers(httptest.NewRequest(http.MethodPost, "/api/agent/v1/events/report", bytes.NewBufferString(`{"events":[{"type":"Warning","message":"重启"}]}`))))
	if events.Code != http.StatusOK {
		t.Fatalf("events report = %d body=%s", events.Code, events.Body.String())
	}
	pull := httptest.NewRecorder()
	mux.ServeHTTP(pull, headers(httptest.NewRequest(http.MethodGet, "/api/agent/v1/tasks?limit=1", nil)))
	if pull.Code != http.StatusOK || !bytes.Contains(pull.Body.Bytes(), []byte("cluster_task_1")) {
		t.Fatalf("pull tasks status=%d body=%s", pull.Code, pull.Body.String())
	}
	result := httptest.NewRecorder()
	mux.ServeHTTP(result, headers(httptest.NewRequest(http.MethodPost, "/api/agent/v1/tasks/result", bytes.NewBufferString(`{"task_id":"cluster_task_1","status":"succeeded","message":"ok"}`))))
	if result.Code != http.StatusOK {
		t.Fatalf("task result = %d body=%s", result.Code, result.Body.String())
	}
	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/clusters?tenant_id=tenant_1&page=1&page_size=10", nil))
	if list.Code != http.StatusOK || bytes.Contains(list.Body.Bytes(), []byte("AgentTokenHash")) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	if !bytes.Contains(list.Body.Bytes(), []byte(`"cloud":"aliyun"`)) {
		t.Fatalf("list should include cluster labels, body=%s", list.Body.String())
	}
	drain := httptest.NewRecorder()
	mux.ServeHTTP(drain, httptest.NewRequest(http.MethodPost, "/api/clusters/cluster_1/drain", bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_cluster_admin"}}`)))
	if drain.Code != http.StatusOK {
		t.Fatalf("drain status=%d body=%s", drain.Code, drain.Body.String())
	}
	disable := httptest.NewRecorder()
	mux.ServeHTTP(disable, httptest.NewRequest(http.MethodPost, "/api/clusters/cluster_1/disable", bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_cluster_admin"}}`)))
	if disable.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/clusters", bytes.NewBufferString(`{bad`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestClusterServiceValidationRotationAndScopedFailures(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_bad", "cluster_1", "cluster_2", "cluster_task_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	if _, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: " "}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("blank cluster name should fail, got %v", err)
	}
	first, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register first: %v", err)
	}
	second, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "生产集群"})
	if err != nil {
		t.Fatalf("register second: %v", err)
	}
	rotated, err := svc.RotateAgentToken(context.Background(), clusterActor(), first.Cluster.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated == first.AgentToken {
		t.Fatalf("rotated token should change")
	}
	if _, err := svc.Authenticate(context.Background(), first.Cluster.ID, first.AgentToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("old token should fail, got %v", err)
	}
	if _, err := svc.UpdateClusterStatus(context.Background(), clusterActor(), first.Cluster.ID, ClusterStatus("bad")); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid status should fail, got %v", err)
	}
	if _, err := svc.UpdateClusterStatus(context.Background(), clusterActor(), first.Cluster.ID, ClusterDisabled); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), first.Cluster.ID, rotated); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("disabled cluster should be denied, got %v", err)
	}
	task, err := svc.CreateTask(context.Background(), ClusterTask{ClusterID: first.Cluster.ID, Type: "argocd_sync"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := svc.ReportStatus(context.Background(), second.Cluster.ID, second.AgentToken, StatusReport{ClusterID: first.Cluster.ID}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("cross-cluster status should fail, got %v", err)
	}
	if _, err := svc.CompleteTask(context.Background(), second.Cluster.ID, second.AgentToken, ClusterTaskResult{TaskID: task.ID, Status: ClusterTaskSucceeded}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("cross-cluster task result should fail, got %v", err)
	}
	if _, err := svc.CompleteTask(context.Background(), second.Cluster.ID, second.AgentToken, ClusterTaskResult{TaskID: "missing", Status: ClusterTaskSucceeded}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing task should fail, got %v", err)
	}
	if _, err := svc.CreateTask(context.Background(), ClusterTask{ClusterID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing cluster task should fail, got %v", err)
	}
}

func TestClusterRepositoryConflictAndMissingBranches(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	cluster := Cluster{ID: "cluster_1", Name: "开发集群", Status: ClusterReady, CreatedAt: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	if err := repo.CreateCluster(ctx, cluster); err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := repo.CreateCluster(ctx, cluster); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate cluster should conflict, got %v", err)
	}
	if err := repo.UpdateCluster(ctx, Cluster{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing cluster should fail, got %v", err)
	}
	if _, err := repo.ListClusters(ctx, shared.PageRequest{Page: 2, PageSize: 10}); err != nil {
		t.Fatalf("list clusters page: %v", err)
	}
	task := ClusterTask{ID: "task_1", ClusterID: cluster.ID, Status: ClusterTaskPending}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := repo.CreateTask(ctx, task); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate task should conflict, got %v", err)
	}
	if err := repo.UpdateTask(ctx, ClusterTask{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing task should fail, got %v", err)
	}
	result := ClusterTaskResult{ID: "result_1", TaskID: task.ID}
	if err := repo.CreateTaskResult(ctx, result); err != nil {
		t.Fatalf("create result: %v", err)
	}
	if err := repo.CreateTaskResult(ctx, result); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate result should conflict, got %v", err)
	}
}

func TestClusterServiceStatusForwardingHeartbeatAndUnreachableBranches(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "heartbeat_1", "snapshot_1", "snapshot_2", "snapshot_3"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock, StageState: failingReportUpdater{err: shared.NewError(shared.CodeInternal, "stage failed")}, HeartbeatTimeout: time.Minute})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := svc.UpdateClusterStatus(context.Background(), clusterActor(), registered.Cluster.ID, ClusterUnreachable); err != nil {
		t.Fatalf("mark unreachable manually: %v", err)
	}
	observed := clock.now.Add(-time.Second)
	if err := svc.Heartbeat(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterHeartbeat{ObservedAt: observed}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	cluster, _ := repo.GetCluster(context.Background(), registered.Cluster.ID)
	if cluster.Status != ClusterReady || cluster.LastHeartbeatAt == nil || !cluster.LastHeartbeatAt.Equal(clock.now) {
		t.Fatalf("heartbeat should recover unreachable cluster and set last heartbeat: %#v", cluster)
	}
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, registered.AgentToken, StatusReport{}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("stage updater error should propagate, got %v", err)
	}
	svc.stageUpdater = nil
	svc.deployments = failingReportUpdater{err: shared.NewError(shared.CodeInternal, "deployment failed")}
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, registered.AgentToken, StatusReport{}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("deployment updater error should propagate, got %v", err)
	}
	svc.deployments = nil
	if err := svc.ReportStatus(context.Background(), registered.Cluster.ID, registered.AgentToken, StatusReport{}); err != nil {
		t.Fatalf("status without updaters: %v", err)
	}
	changed, err := svc.MarkUnreachable(context.Background())
	if err != nil {
		t.Fatalf("mark unreachable fresh heartbeat: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("fresh heartbeat should not be unreachable: %#v", changed)
	}
}

func TestClusterAgentHTTPHandlerAuthenticationFailures(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	headers := func(req *http.Request) *http.Request {
		req.Header.Set("X-PaaS-Cluster-ID", registered.Cluster.ID.String())
		req.Header.Set("Authorization", "Bearer wrong")
		return req
	}
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/agent/v1/heartbeat", `{"agent_version":"v1"}`},
		{http.MethodPost, "/api/agent/v1/status/report", `{}`},
		{http.MethodPost, "/api/agent/v1/events/report", `{}`},
		{http.MethodGet, "/api/agent/v1/tasks", `{}`},
		{http.MethodPost, "/api/agent/v1/tasks/result", `{"task_id":"missing","status":"succeeded"}`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, headers(httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestClusterServicePropagatesRepositoryErrors(t *testing.T) {
	base := newTestRepository(t)
	repo := &clusterRepoWithErrors{Repository: base}
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	ids := &staticIDs{ids: []shared.ID{"cluster_1", "heartbeat_1", "heartbeat_2", "snapshot_1", "cluster_task_1", "cluster_task_result_1", "cluster_task_result_2", "cluster_2"}}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), IDGenerator: ids, Clock: clock})
	registered, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	repo.listErr = shared.NewError(shared.CodeInternal, "list failed")
	if _, err := svc.ListClusters(context.Background(), clusterActor(), "tenant_1", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("list error should propagate, got %v", err)
	}
	if _, err := svc.MarkUnreachable(context.Background()); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("mark unreachable list error should propagate, got %v", err)
	}
	repo.listErr = nil

	repo.createHeartbeatErr = shared.NewError(shared.CodeInternal, "heartbeat failed")
	if err := svc.Heartbeat(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterHeartbeat{}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("heartbeat create error should propagate, got %v", err)
	}
	repo.createHeartbeatErr = nil
	repo.updateClusterErr = shared.NewError(shared.CodeInternal, "update failed")
	if err := svc.Heartbeat(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterHeartbeat{}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("heartbeat update error should propagate, got %v", err)
	}
	if _, err := svc.UpdateClusterStatus(context.Background(), clusterActor(), registered.Cluster.ID, ClusterReady); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("status update error should propagate, got %v", err)
	}
	repo.updateClusterErr = nil

	task, err := svc.CreateTask(context.Background(), ClusterTask{ClusterID: registered.Cluster.ID, Type: "argocd_sync"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	repo.listPendingTasksErr = shared.NewError(shared.CodeInternal, "tasks failed")
	if _, err := svc.PullTasks(context.Background(), registered.Cluster.ID, registered.AgentToken, 10); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("pending task error should propagate, got %v", err)
	}
	repo.listPendingTasksErr = nil
	repo.updateTaskErr = shared.NewError(shared.CodeInternal, "lease failed")
	if _, err := svc.PullTasks(context.Background(), registered.Cluster.ID, registered.AgentToken, 10); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("task lease update error should propagate, got %v", err)
	}
	repo.updateTaskErr = nil

	repo.createTaskResultErr = shared.NewError(shared.CodeInternal, "result failed")
	if _, err := svc.CompleteTask(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterTaskResult{TaskID: task.ID, Status: ClusterTaskSucceeded}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("task result create error should propagate, got %v", err)
	}
	repo.createTaskResultErr = nil
	repo.updateTaskErr = shared.NewError(shared.CodeInternal, "complete failed")
	if _, err := svc.CompleteTask(context.Background(), registered.Cluster.ID, registered.AgentToken, ClusterTaskResult{TaskID: task.ID, Status: ClusterTaskSucceeded}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("task update error should propagate, got %v", err)
	}

	repo.createClusterErr = shared.NewError(shared.CodeInternal, "create cluster failed")
	if _, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "生产集群"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("cluster create error should propagate, got %v", err)
	}
}

func TestClusterRegistrationRequiresTenantAndPermission(t *testing.T) {
	repo := newTestRepository(t)
	clock := &mutableClock{now: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)}
	permission := &recordingClusterPermission{}
	svc := NewService(Options{Repository: repo, TenantQuery: tenantQuery("tenant_1"), PermissionChecker: permission, IDGenerator: &staticIDs{ids: []shared.ID{"cluster_1"}}, Clock: clock})

	if _, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), Name: "开发集群"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing tenant should fail, got %v", err)
	}
	if _, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "missing", Name: "开发集群"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("unknown tenant should fail, got %v", err)
	}

	permission.err = shared.NewError(shared.CodePermissionDenied, "permission denied")
	if _, err := svc.RegisterCluster(context.Background(), RegisterClusterInput{Actor: clusterActor(), TenantID: "tenant_1", Name: "开发集群"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("permission denial should fail, got %v", err)
	}
	if len(permission.calls) != 1 || permission.calls[0].resource.Kind != identityaccess.ScopeTenant || permission.calls[0].resource.TenantID != "tenant_1" || permission.calls[0].action != "cluster:manage" {
		t.Fatalf("unexpected permission calls: %+v", permission.calls)
	}
}
