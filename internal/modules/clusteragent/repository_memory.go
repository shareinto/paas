package clusteragent

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	clusters    map[shared.ID]Cluster
	clusterIDs  []shared.ID
	heartbeats  map[shared.ID]ClusterHeartbeat
	snapshots   map[shared.ID]ClusterResourceSnapshot
	tasks       map[shared.ID]ClusterTask
	taskIDs     []shared.ID
	taskResults map[shared.ID]ClusterTaskResult
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		clusters: map[shared.ID]Cluster{}, heartbeats: map[shared.ID]ClusterHeartbeat{}, snapshots: map[shared.ID]ClusterResourceSnapshot{},
		tasks: map[shared.ID]ClusterTask{}, taskResults: map[shared.ID]ClusterTaskResult{},
	}
}

func (r *MemoryRepository) CreateCluster(_ context.Context, cluster Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[cluster.ID]; ok {
		return shared.NewError(shared.CodeConflict, "cluster already exists")
	}
	r.clusters[cluster.ID] = cluster
	r.clusterIDs = append(r.clusterIDs, cluster.ID)
	return nil
}

func (r *MemoryRepository) UpdateCluster(_ context.Context, cluster Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[cluster.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "cluster not found")
	}
	r.clusters[cluster.ID] = cluster
	return nil
}

func (r *MemoryRepository) GetCluster(_ context.Context, id shared.ID) (Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cluster, ok := r.clusters[id]
	if !ok {
		return Cluster{}, shared.NewError(shared.CodeNotFound, "cluster not found")
	}
	return cluster, nil
}

func (r *MemoryRepository) ListClusters(_ context.Context, page shared.PageRequest) (shared.PageResult[Cluster], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Cluster, 0, len(r.clusterIDs))
	for _, id := range r.clusterIDs {
		items = append(items, r.clusters[id])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) CreateHeartbeat(_ context.Context, heartbeat ClusterHeartbeat) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.heartbeats[heartbeat.ID] = heartbeat
	return nil
}

func (r *MemoryRepository) CreateSnapshot(_ context.Context, snapshot ClusterResourceSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots[snapshot.ID] = snapshot
	return nil
}

func (r *MemoryRepository) CreateTask(_ context.Context, task ClusterTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[task.ID]; ok {
		return shared.NewError(shared.CodeConflict, "cluster task already exists")
	}
	r.tasks[task.ID] = task
	r.taskIDs = append(r.taskIDs, task.ID)
	return nil
}

func (r *MemoryRepository) UpdateTask(_ context.Context, task ClusterTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[task.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "cluster task not found")
	}
	r.tasks[task.ID] = task
	return nil
}

func (r *MemoryRepository) GetTask(_ context.Context, id shared.ID) (ClusterTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return ClusterTask{}, shared.NewError(shared.CodeNotFound, "cluster task not found")
	}
	return task, nil
}

func (r *MemoryRepository) ListPendingTasks(_ context.Context, clusterID shared.ID, limit int) ([]ClusterTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	out := make([]ClusterTask, 0, limit)
	for _, id := range r.taskIDs {
		task := r.tasks[id]
		if task.ClusterID == clusterID && task.Status == ClusterTaskPending {
			out = append(out, task)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (r *MemoryRepository) CreateTaskResult(_ context.Context, result ClusterTaskResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.taskResults[result.ID]; ok {
		return shared.NewError(shared.CodeConflict, "cluster task result already exists")
	}
	r.taskResults[result.ID] = result
	return nil
}
