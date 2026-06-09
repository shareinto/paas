package delivery

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type deliverySnapshot struct {
	Releases   []Release
	Freights   []Freight
	Items      []FreightItem
	Flows      []DeliveryFlow
	Stages     []DeliveryStage
	Promotions []Promotion
	Approvals  []PromotionApproval
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "release-delivery")}
	var snapshot deliverySnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, v := range snapshot.Releases {
		repo.releases[v.ID] = v
		repo.releaseByBuild[v.BuildRunID] = v.ID
	}
	for _, v := range snapshot.Freights {
		repo.freights[v.ID] = v
		repo.freightsByApp[v.ApplicationID] = append(repo.freightsByApp[v.ApplicationID], v.ID)
	}
	for _, v := range snapshot.Items {
		repo.items[v.ID] = v
		repo.itemsByFreight[v.FreightID] = append(repo.itemsByFreight[v.FreightID], v.ID)
	}
	for _, v := range snapshot.Flows {
		repo.flows[v.ID] = v
		repo.flowByApp[v.ApplicationID] = v.ID
	}
	for _, v := range snapshot.Stages {
		repo.stages[v.ID] = v
		repo.stagesByFlow[v.DeliveryFlowID] = append(repo.stagesByFlow[v.DeliveryFlowID], v.ID)
		repo.stageByEnv[v.EnvironmentID] = v.ID
	}
	for _, v := range snapshot.Promotions {
		repo.promotions[v.ID] = v
		repo.promotionsByApp[v.ApplicationID] = append(repo.promotionsByApp[v.ApplicationID], v.ID)
	}
	for _, v := range snapshot.Approvals {
		repo.approvalsByPromotion[v.PromotionID] = v
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() deliverySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := deliverySnapshot{
		Releases: make([]Release, 0, len(r.releases)), Freights: make([]Freight, 0, len(r.freights)),
		Items: make([]FreightItem, 0, len(r.items)), Flows: make([]DeliveryFlow, 0, len(r.flows)),
		Stages: make([]DeliveryStage, 0, len(r.stages)), Promotions: make([]Promotion, 0, len(r.promotions)),
		Approvals: make([]PromotionApproval, 0, len(r.approvalsByPromotion)),
	}
	for _, v := range r.releases {
		out.Releases = append(out.Releases, v)
	}
	for _, v := range r.freights {
		out.Freights = append(out.Freights, v)
	}
	for _, v := range r.items {
		out.Items = append(out.Items, v)
	}
	for _, v := range r.flows {
		out.Flows = append(out.Flows, v)
	}
	for _, v := range r.stages {
		out.Stages = append(out.Stages, v)
	}
	for _, v := range r.promotions {
		out.Promotions = append(out.Promotions, v)
	}
	for _, v := range r.approvalsByPromotion {
		out.Approvals = append(out.Approvals, v)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }
func (r *MySQLRepository) CreateRelease(ctx context.Context, release Release) error {
	if err := r.MemoryRepository.CreateRelease(ctx, release); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateFreight(ctx context.Context, freight Freight) error {
	if err := r.MemoryRepository.CreateFreight(ctx, freight); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateFreightItem(ctx context.Context, item FreightItem) error {
	if err := r.MemoryRepository.CreateFreightItem(ctx, item); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateDeliveryFlow(ctx context.Context, flow DeliveryFlow) error {
	if err := r.MemoryRepository.CreateDeliveryFlow(ctx, flow); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateDeliveryStage(ctx context.Context, stage DeliveryStage) error {
	if err := r.MemoryRepository.CreateDeliveryStage(ctx, stage); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreatePromotion(ctx context.Context, promotion Promotion) error {
	if err := r.MemoryRepository.CreatePromotion(ctx, promotion); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdatePromotion(ctx context.Context, promotion Promotion) error {
	if err := r.MemoryRepository.UpdatePromotion(ctx, promotion); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreatePromotionApproval(ctx context.Context, approval PromotionApproval) error {
	if err := r.MemoryRepository.CreatePromotionApproval(ctx, approval); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdatePromotionApproval(ctx context.Context, approval PromotionApproval) error {
	if err := r.MemoryRepository.UpdatePromotionApproval(ctx, approval); err != nil {
		return err
	}
	return r.persist(ctx)
}
