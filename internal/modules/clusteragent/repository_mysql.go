package clusteragent

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateCluster(ctx context.Context, cluster Cluster) error {
	labels, err := database.MarshalJSON(cluster.Labels)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO clusters (id, tenant_id, name, region, labels_json, server_version, status, agent_token_hash, last_heartbeat_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cluster.ID, cluster.TenantID, cluster.Name, cluster.Region, string(labels), cluster.ServerVersion, cluster.Status,
		cluster.AgentTokenHash, mysqlTimePtr(cluster.LastHeartbeatAt), mysqlTime(cluster.CreatedAt), mysqlTime(cluster.UpdatedAt))
	return database.ConflictOrUnavailable(err, "cluster already exists", "create cluster failed")
}

func (r *MySQLRepository) UpdateCluster(ctx context.Context, cluster Cluster) error {
	labels, err := database.MarshalJSON(cluster.Labels)
	if err != nil {
		return err
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE clusters
SET tenant_id = ?, name = ?, region = ?, labels_json = ?, server_version = ?, status = ?, agent_token_hash = ?,
    last_heartbeat_at = ?, updated_at = ?
WHERE id = ?`,
		cluster.TenantID, cluster.Name, cluster.Region, string(labels), cluster.ServerVersion, cluster.Status,
		cluster.AgentTokenHash, mysqlTimePtr(cluster.LastHeartbeatAt), mysqlTime(cluster.UpdatedAt), cluster.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update cluster failed")
	}
	return database.RequireAffected(result, "cluster not found")
}

func (r *MySQLRepository) GetCluster(ctx context.Context, id shared.ID) (Cluster, error) {
	cluster, err := scanCluster(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, name, region, labels_json, server_version, status, agent_token_hash, last_heartbeat_at, created_at, updated_at
FROM clusters WHERE id = ?`, id))
	if err != nil {
		return Cluster{}, database.NotFound(err, "cluster not found")
	}
	return cluster, nil
}

func (r *MySQLRepository) ListClusters(ctx context.Context, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM clusters").Scan(&total); err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "count clusters failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, name, region, labels_json, server_version, status, agent_token_hash, last_heartbeat_at, created_at, updated_at
FROM clusters ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "list clusters failed")
	}
	defer rows.Close()
	items := []Cluster{}
	for rows.Next() {
		cluster, err := scanCluster(rows)
		if err != nil {
			return shared.PageResult[Cluster]{}, err
		}
		items = append(items, cluster)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "list clusters failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) ListClustersByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM clusters WHERE tenant_id = ?", tenantID).Scan(&total); err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "count tenant clusters failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, name, region, labels_json, server_version, status, agent_token_hash, last_heartbeat_at, created_at, updated_at
FROM clusters WHERE tenant_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, tenantID, limit, offset)
	if err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "list tenant clusters failed")
	}
	defer rows.Close()
	items := []Cluster{}
	for rows.Next() {
		cluster, err := scanCluster(rows)
		if err != nil {
			return shared.PageResult[Cluster]{}, err
		}
		items = append(items, cluster)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Cluster]{}, database.WrapUnavailable(err, "list tenant clusters failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) CreateHeartbeat(ctx context.Context, heartbeat ClusterHeartbeat) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_heartbeats (id, cluster_id, agent_version, observed_at, message, control_plane_url)
VALUES (?, ?, ?, ?, ?, ?)`,
		heartbeat.ID, heartbeat.ClusterID, heartbeat.AgentVersion, mysqlTime(heartbeat.ObservedAt), heartbeat.Message, heartbeat.ControlPlaneURL)
	return database.ConflictOrUnavailable(err, "cluster heartbeat already exists", "create cluster heartbeat failed")
}

func (r *MySQLRepository) CreateSnapshot(ctx context.Context, snapshot ClusterResourceSnapshot) error {
	payload, err := database.MarshalJSON(snapshot.Payload)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_resource_snapshots (id, cluster_id, tenant_id, payload, reported_at)
VALUES (?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.ClusterID, snapshot.TenantID, string(payload), mysqlTime(snapshot.ReportedAt))
	return database.ConflictOrUnavailable(err, "cluster resource snapshot already exists", "create cluster resource snapshot failed")
}

func (r *MySQLRepository) ReplaceRuntimeResources(ctx context.Context, clusterID shared.ID, tenantID shared.ID, reportedAt time.Time, resources []RuntimeResourceStatus) error {
	if len(resources) == 0 {
		return nil
	}
	tx := database.NewTransactor(r.db)
	return tx.WithinTx(ctx, func(txCtx context.Context) error {
		groups := map[string]RuntimeResourceFilter{}
		for _, status := range resources {
			status = normalizeRuntimeResourceStatus(status)
			if status.ApplicationID.IsZero() || status.StageKey == "" {
				continue
			}
			key := runtimeResourceGroupKey(status.ApplicationID, status.StageKey)
			groups[key] = RuntimeResourceFilter{ApplicationID: status.ApplicationID, StageKey: status.StageKey}
		}
		for _, group := range groups {
			if _, err := database.ExecutorFromContext(txCtx, r.db).ExecContext(txCtx, `
DELETE e FROM cluster_runtime_resource_events e
JOIN cluster_runtime_resources r ON r.id = e.resource_id
WHERE r.cluster_id = ? AND r.application_id = ? AND r.stage_key = ?`,
				clusterID, group.ApplicationID, group.StageKey); err != nil {
				return database.WrapUnavailable(err, "delete runtime resource events failed")
			}
			if _, err := database.ExecutorFromContext(txCtx, r.db).ExecContext(txCtx, `
DELETE FROM cluster_runtime_resources
WHERE cluster_id = ? AND application_id = ? AND stage_key = ?`,
				clusterID, group.ApplicationID, group.StageKey); err != nil {
				return database.WrapUnavailable(err, "delete runtime resources failed")
			}
		}
		for _, status := range resources {
			status = normalizeRuntimeResourceStatus(status)
			if status.ApplicationID.IsZero() || status.StageKey == "" || status.Kind == "" || status.Name == "" {
				continue
			}
			resource := RuntimeResource{
				ID:              stableRuntimeResourceID(clusterID, status),
				ClusterID:       clusterID,
				TenantID:        tenantID,
				ApplicationID:   status.ApplicationID,
				StageKey:        status.StageKey,
				Group:           status.Group,
				Version:         status.Version,
				Kind:            status.Kind,
				Namespace:       status.Namespace,
				Name:            status.Name,
				ParentKind:      status.ParentKind,
				ParentNamespace: status.ParentNamespace,
				ParentName:      status.ParentName,
				Status:          status.Status,
				HealthStatus:    status.HealthStatus,
				Message:         status.Message,
				Desired:         status.Desired,
				Ready:           status.Ready,
				Containers:      status.Containers,
				Events:          status.Events,
				ReportedAt:      reportedAt,
				UpdatedAt:       reportedAt,
			}
			if err := r.insertRuntimeResource(txCtx, resource); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *MySQLRepository) insertRuntimeResource(ctx context.Context, resource RuntimeResource) error {
	containers, err := database.MarshalJSON(resource.Containers)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_runtime_resources (
  id, cluster_id, tenant_id, application_id, stage_key, group_name, version, kind, namespace, name,
  parent_kind, parent_namespace, parent_name, status, health_status, message, desired, ready, containers_json, reported_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		resource.ID, resource.ClusterID, resource.TenantID, resource.ApplicationID, resource.StageKey, resource.Group, resource.Version, resource.Kind, resource.Namespace, resource.Name,
		resource.ParentKind, resource.ParentNamespace, resource.ParentName, resource.Status, resource.HealthStatus, resource.Message, resource.Desired, resource.Ready, string(containers), mysqlTime(resource.ReportedAt), mysqlTime(resource.UpdatedAt))
	if err != nil {
		return database.ConflictOrUnavailable(err, "runtime resource already exists", "create runtime resource failed")
	}
	for i, event := range resource.Events {
		eventID := shared.ID(fmt.Sprintf("%s_event_%d", resource.ID, i))
		_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_runtime_resource_events (
  id, resource_id, cluster_id, tenant_id, application_id, stage_key, type, reason, message, count, occurred_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			eventID, resource.ID, resource.ClusterID, resource.TenantID, resource.ApplicationID, resource.StageKey,
			strings.TrimSpace(event.Type), strings.TrimSpace(event.Reason), strings.TrimSpace(event.Message), event.Count, mysqlTimePtr(nilIfZero(event.OccurredAt)))
		if err != nil {
			return database.ConflictOrUnavailable(err, "runtime resource event already exists", "create runtime resource event failed")
		}
	}
	return nil
}

func (r *MySQLRepository) ListRuntimeResources(ctx context.Context, filter RuntimeResourceFilter) ([]RuntimeResource, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, cluster_id, tenant_id, application_id, stage_key, group_name, version, kind, namespace, name,
       parent_kind, parent_namespace, parent_name, status, health_status, message, desired, ready, containers_json, reported_at, updated_at
FROM cluster_runtime_resources
WHERE application_id = ? AND stage_key = ?
ORDER BY FIELD(kind, 'Application', 'Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Pod', 'Event'), kind ASC, namespace ASC, name ASC`,
		filter.ApplicationID, strings.TrimSpace(filter.StageKey))
	if err != nil {
		return nil, database.WrapUnavailable(err, "list runtime resources failed")
	}
	defer rows.Close()
	items := []RuntimeResource{}
	for rows.Next() {
		resource, err := scanRuntimeResource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list runtime resources failed")
	}
	return r.loadRuntimeResourceEvents(ctx, items)
}

func (r *MySQLRepository) GetRuntimeResource(ctx context.Context, id shared.ID) (RuntimeResource, error) {
	resource, err := scanRuntimeResource(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, cluster_id, tenant_id, application_id, stage_key, group_name, version, kind, namespace, name,
       parent_kind, parent_namespace, parent_name, status, health_status, message, desired, ready, containers_json, reported_at, updated_at
FROM cluster_runtime_resources WHERE id = ?`, id))
	if err != nil {
		return RuntimeResource{}, database.NotFound(err, "runtime resource not found")
	}
	resources, err := r.loadRuntimeResourceEvents(ctx, []RuntimeResource{resource})
	if err != nil {
		return RuntimeResource{}, err
	}
	return resources[0], nil
}

func (r *MySQLRepository) CreateTask(ctx context.Context, task ClusterTask) error {
	payload, err := database.MarshalJSON(task.Payload)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_tasks (id, cluster_id, type, target_ref, payload, status, result_message, created_at, updated_at, leased_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.ClusterID, task.Type, task.TargetRef, string(payload), task.Status, task.ResultMessage,
		mysqlTime(task.CreatedAt), mysqlTime(task.UpdatedAt), mysqlTimePtr(task.LeasedAt), mysqlTimePtr(task.CompletedAt))
	return database.ConflictOrUnavailable(err, "cluster task already exists", "create cluster task failed")
}

func (r *MySQLRepository) UpdateTask(ctx context.Context, task ClusterTask) error {
	payload, err := database.MarshalJSON(task.Payload)
	if err != nil {
		return err
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE cluster_tasks
SET cluster_id = ?, type = ?, target_ref = ?, payload = ?, status = ?, result_message = ?,
    updated_at = ?, leased_at = ?, completed_at = ?
WHERE id = ?`,
		task.ClusterID, task.Type, task.TargetRef, string(payload), task.Status, task.ResultMessage,
		mysqlTime(task.UpdatedAt), mysqlTimePtr(task.LeasedAt), mysqlTimePtr(task.CompletedAt), task.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update cluster task failed")
	}
	return database.RequireAffected(result, "cluster task not found")
}

func (r *MySQLRepository) GetTask(ctx context.Context, id shared.ID) (ClusterTask, error) {
	task, err := scanTask(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, cluster_id, type, target_ref, payload, status, result_message, created_at, updated_at, leased_at, completed_at
FROM cluster_tasks WHERE id = ?`, id))
	if err != nil {
		return ClusterTask{}, database.NotFound(err, "cluster task not found")
	}
	return task, nil
}

func (r *MySQLRepository) ListPendingTasks(ctx context.Context, clusterID shared.ID, limit int) ([]ClusterTask, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, cluster_id, type, target_ref, payload, status, result_message, created_at, updated_at, leased_at, completed_at
FROM cluster_tasks
WHERE cluster_id = ? AND status = ?
ORDER BY created_at ASC, id ASC LIMIT ?`, clusterID, ClusterTaskPending, limit)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list pending cluster tasks failed")
	}
	defer rows.Close()
	items := []ClusterTask{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, task)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list pending cluster tasks failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateTaskResult(ctx context.Context, result ClusterTaskResult) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO cluster_task_results (id, cluster_id, task_id, status, message, reported_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		result.ID, result.ClusterID, result.TaskID, result.Status, result.Message, mysqlTime(result.ReportedAt))
	return database.ConflictOrUnavailable(err, "cluster task result already exists", "create cluster task result failed")
}

type clusterScanner interface {
	Scan(dest ...any) error
}

func scanCluster(scanner clusterScanner) (Cluster, error) {
	var cluster Cluster
	var labels []byte
	if err := scanner.Scan(&cluster.ID, &cluster.TenantID, &cluster.Name, &cluster.Region, &labels, &cluster.ServerVersion, &cluster.Status, &cluster.AgentTokenHash, &cluster.LastHeartbeatAt, &cluster.CreatedAt, &cluster.UpdatedAt); err != nil {
		return Cluster{}, err
	}
	if err := database.UnmarshalJSON(labels, &cluster.Labels); err != nil {
		return Cluster{}, err
	}
	return cluster, nil
}

func scanTask(scanner clusterScanner) (ClusterTask, error) {
	var task ClusterTask
	var payload []byte
	if err := scanner.Scan(&task.ID, &task.ClusterID, &task.Type, &task.TargetRef, &payload, &task.Status, &task.ResultMessage, &task.CreatedAt, &task.UpdatedAt, &task.LeasedAt, &task.CompletedAt); err != nil {
		return ClusterTask{}, err
	}
	if err := database.UnmarshalJSON(payload, &task.Payload); err != nil {
		return ClusterTask{}, err
	}
	return task, nil
}

func scanRuntimeResource(scanner clusterScanner) (RuntimeResource, error) {
	var resource RuntimeResource
	var containers []byte
	if err := scanner.Scan(
		&resource.ID, &resource.ClusterID, &resource.TenantID, &resource.ApplicationID, &resource.StageKey,
		&resource.Group, &resource.Version, &resource.Kind, &resource.Namespace, &resource.Name,
		&resource.ParentKind, &resource.ParentNamespace, &resource.ParentName, &resource.Status, &resource.HealthStatus, &resource.Message,
		&resource.Desired, &resource.Ready, &containers, &resource.ReportedAt, &resource.UpdatedAt,
	); err != nil {
		return RuntimeResource{}, err
	}
	if err := database.UnmarshalJSON(containers, &resource.Containers); err != nil {
		return RuntimeResource{}, err
	}
	return resource, nil
}

func (r *MySQLRepository) loadRuntimeResourceEvents(ctx context.Context, resources []RuntimeResource) ([]RuntimeResource, error) {
	for i := range resources {
		rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT type, reason, message, count, occurred_at
FROM cluster_runtime_resource_events
WHERE resource_id = ?
ORDER BY occurred_at DESC, id ASC`, resources[i].ID)
		if err != nil {
			return nil, database.WrapUnavailable(err, "list runtime resource events failed")
		}
		for rows.Next() {
			var event RuntimeResourceEvent
			var occurredAt sql.NullTime
			if err := rows.Scan(&event.Type, &event.Reason, &event.Message, &event.Count, &occurredAt); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if occurredAt.Valid {
				event.OccurredAt = occurredAt.Time
			}
			resources[i].Events = append(resources[i].Events, event)
		}
		if err := rows.Close(); err != nil {
			return nil, database.WrapUnavailable(err, "list runtime resource events failed")
		}
	}
	return resources, nil
}

func normalizeRuntimeResourceStatus(status RuntimeResourceStatus) RuntimeResourceStatus {
	status.StageKey = strings.TrimSpace(status.StageKey)
	status.Group = strings.TrimSpace(status.Group)
	status.Version = strings.TrimSpace(status.Version)
	status.Kind = strings.TrimSpace(status.Kind)
	status.Namespace = strings.TrimSpace(status.Namespace)
	status.Name = strings.TrimSpace(status.Name)
	status.ParentKind = strings.TrimSpace(status.ParentKind)
	status.ParentNamespace = strings.TrimSpace(status.ParentNamespace)
	status.ParentName = strings.TrimSpace(status.ParentName)
	status.Status = strings.TrimSpace(status.Status)
	status.HealthStatus = strings.TrimSpace(status.HealthStatus)
	status.Message = strings.TrimSpace(status.Message)
	return status
}

func runtimeResourceGroupKey(applicationID shared.ID, stageKey string) string {
	return string(applicationID) + "\x00" + stageKey
}

func stableRuntimeResourceID(clusterID shared.ID, status RuntimeResourceStatus) shared.ID {
	sum := sha1.Sum([]byte(strings.Join([]string{
		string(clusterID),
		string(status.ApplicationID),
		status.StageKey,
		status.Group,
		status.Kind,
		status.Namespace,
		status.Name,
	}, "\x00")))
	return shared.ID("runtime_" + hex.EncodeToString(sum[:]))
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func mysqlTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func nilIfZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
