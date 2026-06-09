package clusteragent

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type clusterAgentSnapshot struct {
	Clusters    []Cluster
	Heartbeats  []ClusterHeartbeat
	Snapshots   []ClusterResourceSnapshot
	Tasks       []ClusterTask
	TaskResults []ClusterTaskResult
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "cluster-agent")}
	var snapshot clusterAgentSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, v := range snapshot.Clusters {
		repo.clusters[v.ID] = v
		repo.clusterIDs = append(repo.clusterIDs, v.ID)
	}
	for _, v := range snapshot.Heartbeats {
		repo.heartbeats[v.ID] = v
	}
	for _, v := range snapshot.Snapshots {
		repo.snapshots[v.ID] = v
	}
	for _, v := range snapshot.Tasks {
		repo.tasks[v.ID] = v
		repo.taskIDs = append(repo.taskIDs, v.ID)
	}
	for _, v := range snapshot.TaskResults {
		repo.taskResults[v.ID] = v
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() clusterAgentSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := clusterAgentSnapshot{
		Clusters: make([]Cluster, 0, len(r.clusterIDs)), Heartbeats: make([]ClusterHeartbeat, 0, len(r.heartbeats)),
		Snapshots: make([]ClusterResourceSnapshot, 0, len(r.snapshots)), Tasks: make([]ClusterTask, 0, len(r.taskIDs)),
		TaskResults: make([]ClusterTaskResult, 0, len(r.taskResults)),
	}
	for _, id := range r.clusterIDs {
		out.Clusters = append(out.Clusters, r.clusters[id])
	}
	for _, v := range r.heartbeats {
		out.Heartbeats = append(out.Heartbeats, v)
	}
	for _, v := range r.snapshots {
		out.Snapshots = append(out.Snapshots, v)
	}
	for _, id := range r.taskIDs {
		out.Tasks = append(out.Tasks, r.tasks[id])
	}
	for _, v := range r.taskResults {
		out.TaskResults = append(out.TaskResults, v)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }
func (r *MySQLRepository) CreateCluster(ctx context.Context, cluster Cluster) error {
	if err := r.MemoryRepository.CreateCluster(ctx, cluster); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateCluster(ctx context.Context, cluster Cluster) error {
	if err := r.MemoryRepository.UpdateCluster(ctx, cluster); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateHeartbeat(ctx context.Context, heartbeat ClusterHeartbeat) error {
	if err := r.MemoryRepository.CreateHeartbeat(ctx, heartbeat); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateSnapshot(ctx context.Context, snapshot ClusterResourceSnapshot) error {
	if err := r.MemoryRepository.CreateSnapshot(ctx, snapshot); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateTask(ctx context.Context, task ClusterTask) error {
	if err := r.MemoryRepository.CreateTask(ctx, task); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateTask(ctx context.Context, task ClusterTask) error {
	if err := r.MemoryRepository.UpdateTask(ctx, task); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateTaskResult(ctx context.Context, result ClusterTaskResult) error {
	if err := r.MemoryRepository.CreateTaskResult(ctx, result); err != nil {
		return err
	}
	return r.persist(ctx)
}
