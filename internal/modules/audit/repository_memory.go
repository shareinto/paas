package audit

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	logs   map[shared.ID]AuditLog
	order  []shared.ID
	sealed map[shared.ID]struct{}
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{logs: map[shared.ID]AuditLog{}, sealed: map[shared.ID]struct{}{}}
}

func (r *MemoryRepository) Append(_ context.Context, log AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.logs[log.ID]; ok {
		return shared.NewError(shared.CodeConflict, "audit log already exists")
	}
	r.logs[log.ID] = log
	r.order = append(r.order, log.ID)
	r.sealed[log.ID] = struct{}{}
	return nil
}

func (r *MemoryRepository) Get(_ context.Context, id shared.ID) (AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	log, ok := r.logs[id]
	if !ok {
		return AuditLog{}, shared.NewError(shared.CodeNotFound, "audit log not found")
	}
	return cloneLog(log), nil
}

func (r *MemoryRepository) List(_ context.Context, query Query, page shared.PageRequest) (shared.PageResult[AuditLog], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]AuditLog, 0, len(r.order))
	for _, id := range r.order {
		log := r.logs[id]
		if !matchQuery(log, query) {
			continue
		}
		items = append(items, cloneLog(log))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.After(items[j].OccurredAt) })
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

func matchQuery(log AuditLog, query Query) bool {
	if !query.TenantID.IsZero() && log.TenantID != query.TenantID {
		return false
	}
	if !query.ProjectID.IsZero() && log.ProjectID != query.ProjectID {
		return false
	}
	if !query.ActorID.IsZero() && log.ActorID != query.ActorID {
		return false
	}
	if query.ResourceType != "" && log.ResourceType != query.ResourceType {
		return false
	}
	if !query.ResourceID.IsZero() && log.ResourceID != query.ResourceID {
		return false
	}
	if query.Action != "" && log.Action != query.Action {
		return false
	}
	if query.From != nil && log.OccurredAt.Before(*query.From) {
		return false
	}
	if query.To != nil && log.OccurredAt.After(*query.To) {
		return false
	}
	return true
}

func cloneLog(log AuditLog) AuditLog {
	if len(log.Details) == 0 {
		return log
	}
	details := make(map[string]string, len(log.Details))
	for k, v := range log.Details {
		details[k] = v
	}
	log.Details = details
	return log
}
