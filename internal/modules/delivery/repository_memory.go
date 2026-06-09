package delivery

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu                   sync.RWMutex
	releases             map[shared.ID]Release
	releaseByBuild       map[shared.ID]shared.ID
	freights             map[shared.ID]Freight
	freightsByApp        map[shared.ID][]shared.ID
	items                map[shared.ID]FreightItem
	itemsByFreight       map[shared.ID][]shared.ID
	flows                map[shared.ID]DeliveryFlow
	flowByApp            map[shared.ID]shared.ID
	stages               map[shared.ID]DeliveryStage
	stagesByFlow         map[shared.ID][]shared.ID
	stageByEnv           map[shared.ID]shared.ID
	promotions           map[shared.ID]Promotion
	promotionsByApp      map[shared.ID][]shared.ID
	approvalsByPromotion map[shared.ID]PromotionApproval
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		releases: map[shared.ID]Release{}, releaseByBuild: map[shared.ID]shared.ID{},
		freights: map[shared.ID]Freight{}, freightsByApp: map[shared.ID][]shared.ID{},
		items: map[shared.ID]FreightItem{}, itemsByFreight: map[shared.ID][]shared.ID{},
		flows: map[shared.ID]DeliveryFlow{}, flowByApp: map[shared.ID]shared.ID{},
		stages: map[shared.ID]DeliveryStage{}, stagesByFlow: map[shared.ID][]shared.ID{}, stageByEnv: map[shared.ID]shared.ID{},
		promotions: map[shared.ID]Promotion{}, promotionsByApp: map[shared.ID][]shared.ID{}, approvalsByPromotion: map[shared.ID]PromotionApproval{},
	}
}

func (r *MemoryRepository) CreateRelease(_ context.Context, release Release) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.releases[release.ID]; ok {
		return shared.NewError(shared.CodeConflict, "release already exists")
	}
	if _, ok := r.releaseByBuild[release.BuildRunID]; ok {
		return shared.NewError(shared.CodeConflict, "release already exists for build run")
	}
	r.releases[release.ID] = release
	r.releaseByBuild[release.BuildRunID] = release.ID
	return nil
}
func (r *MemoryRepository) GetRelease(_ context.Context, id shared.ID) (Release, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.releases[id]
	if !ok {
		return Release{}, shared.NewError(shared.CodeNotFound, "release not found")
	}
	return v, nil
}
func (r *MemoryRepository) FindReleaseByBuildRun(_ context.Context, buildRunID shared.ID) (Release, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.releaseByBuild[buildRunID]
	if !ok {
		return Release{}, shared.NewError(shared.CodeNotFound, "release not found")
	}
	return r.releases[id], nil
}

func (r *MemoryRepository) CreateFreight(_ context.Context, freight Freight) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.freights[freight.ID]; ok {
		return shared.NewError(shared.CodeConflict, "freight already exists")
	}
	r.freights[freight.ID] = freight
	r.freightsByApp[freight.ApplicationID] = append(r.freightsByApp[freight.ApplicationID], freight.ID)
	return nil
}
func (r *MemoryRepository) GetFreight(_ context.Context, id shared.ID) (Freight, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.freights[id]
	if !ok {
		return Freight{}, shared.NewError(shared.CodeNotFound, "freight not found")
	}
	return v, nil
}
func (r *MemoryRepository) ListFreightsByApplication(_ context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Freight, 0, len(r.freightsByApp[applicationID]))
	for _, id := range r.freightsByApp[applicationID] {
		items = append(items, r.freights[id])
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
func (r *MemoryRepository) CreateFreightItem(_ context.Context, item FreightItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.freights[item.FreightID]; !ok {
		return shared.NewError(shared.CodeNotFound, "freight not found")
	}
	if _, ok := r.items[item.ID]; ok {
		return shared.NewError(shared.CodeConflict, "freight item already exists")
	}
	r.items[item.ID] = item
	r.itemsByFreight[item.FreightID] = append(r.itemsByFreight[item.FreightID], item.ID)
	return nil
}
func (r *MemoryRepository) ListFreightItems(_ context.Context, freightID shared.ID) ([]FreightItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.freights[freightID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "freight not found")
	}
	out := make([]FreightItem, 0, len(r.itemsByFreight[freightID]))
	for _, id := range r.itemsByFreight[freightID] {
		out = append(out, r.items[id])
	}
	return out, nil
}

func (r *MemoryRepository) CreateDeliveryFlow(_ context.Context, flow DeliveryFlow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.flows[flow.ID]; ok {
		return shared.NewError(shared.CodeConflict, "delivery flow already exists")
	}
	if _, ok := r.flowByApp[flow.ApplicationID]; ok {
		return shared.NewError(shared.CodeConflict, "delivery flow already exists for application")
	}
	r.flows[flow.ID] = flow
	r.flowByApp[flow.ApplicationID] = flow.ID
	return nil
}
func (r *MemoryRepository) GetDeliveryFlow(_ context.Context, id shared.ID) (DeliveryFlow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.flows[id]
	if !ok {
		return DeliveryFlow{}, shared.NewError(shared.CodeNotFound, "delivery flow not found")
	}
	return v, nil
}
func (r *MemoryRepository) FindDeliveryFlowByApplication(_ context.Context, applicationID shared.ID) (DeliveryFlow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.flowByApp[applicationID]
	if !ok {
		return DeliveryFlow{}, shared.NewError(shared.CodeNotFound, "delivery flow not found")
	}
	return r.flows[id], nil
}
func (r *MemoryRepository) CreateDeliveryStage(_ context.Context, stage DeliveryStage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.flows[stage.DeliveryFlowID]; !ok {
		return shared.NewError(shared.CodeNotFound, "delivery flow not found")
	}
	if _, ok := r.stages[stage.ID]; ok {
		return shared.NewError(shared.CodeConflict, "delivery stage already exists")
	}
	if _, ok := r.stageByEnv[stage.EnvironmentID]; ok {
		return shared.NewError(shared.CodeConflict, "delivery stage already exists for environment")
	}
	r.stages[stage.ID] = stage
	r.stagesByFlow[stage.DeliveryFlowID] = append(r.stagesByFlow[stage.DeliveryFlowID], stage.ID)
	r.stageByEnv[stage.EnvironmentID] = stage.ID
	return nil
}
func (r *MemoryRepository) GetDeliveryStage(_ context.Context, id shared.ID) (DeliveryStage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.stages[id]
	if !ok {
		return DeliveryStage{}, shared.NewError(shared.CodeNotFound, "delivery stage not found")
	}
	return v, nil
}
func (r *MemoryRepository) FindDeliveryStageByEnvironment(_ context.Context, applicationID shared.ID, environmentID shared.ID) (DeliveryStage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.stageByEnv[environmentID]
	if !ok {
		return DeliveryStage{}, shared.NewError(shared.CodeNotFound, "delivery stage not found")
	}
	stage := r.stages[id]
	if stage.ApplicationID != applicationID {
		return DeliveryStage{}, shared.NewError(shared.CodeNotFound, "delivery stage not found")
	}
	return stage, nil
}
func (r *MemoryRepository) ListDeliveryStages(_ context.Context, flowID shared.ID) ([]DeliveryStage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.flows[flowID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "delivery flow not found")
	}
	out := make([]DeliveryStage, 0, len(r.stagesByFlow[flowID]))
	for _, id := range r.stagesByFlow[flowID] {
		out = append(out, r.stages[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}

func (r *MemoryRepository) CreatePromotion(_ context.Context, promotion Promotion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.promotions[promotion.ID]; ok {
		return shared.NewError(shared.CodeConflict, "promotion already exists")
	}
	r.promotions[promotion.ID] = promotion
	r.promotionsByApp[promotion.ApplicationID] = append(r.promotionsByApp[promotion.ApplicationID], promotion.ID)
	return nil
}
func (r *MemoryRepository) UpdatePromotion(_ context.Context, promotion Promotion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, ok := r.promotions[promotion.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "promotion not found")
	}
	if prev.ApplicationID != promotion.ApplicationID || prev.FreightID != promotion.FreightID {
		return shared.NewError(shared.CodeInvalidArgument, "promotion ownership cannot be changed")
	}
	r.promotions[promotion.ID] = promotion
	return nil
}
func (r *MemoryRepository) GetPromotion(_ context.Context, id shared.ID) (Promotion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.promotions[id]
	if !ok {
		return Promotion{}, shared.NewError(shared.CodeNotFound, "promotion not found")
	}
	return v, nil
}
func (r *MemoryRepository) ListPromotionsByApplication(_ context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Promotion, 0, len(r.promotionsByApp[applicationID]))
	for _, id := range r.promotionsByApp[applicationID] {
		items = append(items, r.promotions[id])
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
func (r *MemoryRepository) CreatePromotionApproval(_ context.Context, approval PromotionApproval) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.promotions[approval.PromotionID]; !ok {
		return shared.NewError(shared.CodeNotFound, "promotion not found")
	}
	if _, ok := r.approvalsByPromotion[approval.PromotionID]; ok {
		return shared.NewError(shared.CodeConflict, "promotion approval already exists")
	}
	r.approvalsByPromotion[approval.PromotionID] = approval
	return nil
}
func (r *MemoryRepository) UpdatePromotionApproval(_ context.Context, approval PromotionApproval) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.approvalsByPromotion[approval.PromotionID]; !ok {
		return shared.NewError(shared.CodeNotFound, "promotion approval not found")
	}
	r.approvalsByPromotion[approval.PromotionID] = approval
	return nil
}
func (r *MemoryRepository) GetPromotionApproval(_ context.Context, promotionID shared.ID) (PromotionApproval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.approvalsByPromotion[promotionID]
	if !ok {
		return PromotionApproval{}, shared.NewError(shared.CodeNotFound, "promotion approval not found")
	}
	return v, nil
}
