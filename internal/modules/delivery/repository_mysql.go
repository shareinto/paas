package delivery

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateRelease(ctx context.Context, release Release) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO releases (id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, build_run_id, build_artifact_id, version, commit_sha, image_uri, image_digest, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		release.ID, release.TenantID, release.ProjectID, release.ApplicationID, release.PipelineID, release.PipelineName,
		release.PipelineDisplayName, release.BuildRunID, release.BuildArtifactID, release.Version, release.CommitSHA,
		release.ImageURI, release.ImageDigest, release.Status, release.CreatedAt)
	return database.ConflictOrUnavailable(err, "release already exists", "create release failed")
}

func (r *MySQLRepository) GetRelease(ctx context.Context, id shared.ID) (Release, error) {
	release, err := scanRelease(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, releaseSelect()+" WHERE id = ?", id))
	if err != nil {
		return Release{}, database.NotFound(err, "release not found")
	}
	return release, nil
}

func (r *MySQLRepository) FindReleaseByBuildRun(ctx context.Context, buildRunID shared.ID) (Release, error) {
	release, err := scanRelease(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, releaseSelect()+" WHERE build_run_id = ?", buildRunID))
	if err != nil {
		return Release{}, database.NotFound(err, "release not found")
	}
	return release, nil
}

func (r *MySQLRepository) CreateFreight(ctx context.Context, freight Freight) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO freights (id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, name, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		freight.ID, freight.TenantID, freight.ProjectID, freight.ApplicationID, freight.PipelineID,
		freight.PipelineName, freight.PipelineDisplayName, freight.Name, freight.Status, freight.CreatedAt)
	return database.ConflictOrUnavailable(err, "freight already exists", "create freight failed")
}

func (r *MySQLRepository) GetFreight(ctx context.Context, id shared.ID) (Freight, error) {
	freight, err := scanFreight(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, freightSelect()+" WHERE id = ?", id))
	if err != nil {
		return Freight{}, database.NotFound(err, "freight not found")
	}
	return freight, nil
}

func (r *MySQLRepository) ListFreightsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error) {
	return listByApplication(ctx, r.db, applicationID, page, "freights", freightSelect(), scanFreight)
}

func (r *MySQLRepository) CreateFreightItem(ctx context.Context, item FreightItem) error {
	if _, err := r.GetFreight(ctx, item.FreightID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO freight_items (id, tenant_id, project_id, freight_id, application_id, release_id, build_artifact_id, source_key, type, name, uri, digest, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.TenantID, item.ProjectID, item.FreightID, item.ApplicationID, item.ReleaseID,
		item.BuildArtifactID, item.SourceKey, item.Type, item.Name, item.URI, item.Digest, item.CreatedAt)
	return database.ConflictOrUnavailable(err, "freight item already exists", "create freight item failed")
}

func (r *MySQLRepository) ListFreightItems(ctx context.Context, freightID shared.ID) ([]FreightItem, error) {
	if _, err := r.GetFreight(ctx, freightID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, freightItemSelect()+" WHERE freight_id = ? ORDER BY created_at ASC, id ASC", freightID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list freight items failed")
	}
	defer rows.Close()
	items := []FreightItem{}
	for rows.Next() {
		item, err := scanFreightItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list freight items failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateDeliveryFlow(ctx context.Context, flow DeliveryFlow) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO delivery_flows (id, tenant_id, project_id, application_id, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		flow.ID, flow.TenantID, flow.ProjectID, flow.ApplicationID, flow.Name, flow.CreatedAt, flow.UpdatedAt)
	return database.ConflictOrUnavailable(err, "delivery flow already exists", "create delivery flow failed")
}

func (r *MySQLRepository) GetDeliveryFlow(ctx context.Context, id shared.ID) (DeliveryFlow, error) {
	flow, err := scanDeliveryFlow(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryFlowSelect()+" WHERE id = ?", id))
	if err != nil {
		return DeliveryFlow{}, database.NotFound(err, "delivery flow not found")
	}
	return flow, nil
}

func (r *MySQLRepository) FindDeliveryFlowByApplication(ctx context.Context, applicationID shared.ID) (DeliveryFlow, error) {
	flow, err := scanDeliveryFlow(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryFlowSelect()+" WHERE application_id = ?", applicationID))
	if err != nil {
		return DeliveryFlow{}, database.NotFound(err, "delivery flow not found")
	}
	return flow, nil
}

func (r *MySQLRepository) CreateDeliveryStage(ctx context.Context, stage DeliveryStage) error {
	if _, err := r.GetDeliveryFlow(ctx, stage.DeliveryFlowID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO delivery_stages (id, tenant_id, project_id, application_id, delivery_flow_id, environment_id, name, stage_order, requires_approval, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		stage.ID, stage.TenantID, stage.ProjectID, stage.ApplicationID, stage.DeliveryFlowID,
		stage.EnvironmentID, stage.Name, stage.Order, stage.RequiresApproval, stage.CreatedAt, stage.UpdatedAt)
	return database.ConflictOrUnavailable(err, "delivery stage already exists", "create delivery stage failed")
}

func (r *MySQLRepository) GetDeliveryStage(ctx context.Context, id shared.ID) (DeliveryStage, error) {
	stage, err := scanDeliveryStage(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryStageSelect()+" WHERE id = ?", id))
	if err != nil {
		return DeliveryStage{}, database.NotFound(err, "delivery stage not found")
	}
	return stage, nil
}

func (r *MySQLRepository) FindDeliveryStageByEnvironment(ctx context.Context, applicationID shared.ID, environmentID shared.ID) (DeliveryStage, error) {
	stage, err := scanDeliveryStage(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryStageSelect()+" WHERE application_id = ? AND environment_id = ?", applicationID, environmentID))
	if err != nil {
		return DeliveryStage{}, database.NotFound(err, "delivery stage not found")
	}
	return stage, nil
}

func (r *MySQLRepository) ListDeliveryStages(ctx context.Context, flowID shared.ID) ([]DeliveryStage, error) {
	if _, err := r.GetDeliveryFlow(ctx, flowID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, deliveryStageSelect()+" WHERE delivery_flow_id = ? ORDER BY stage_order ASC, created_at ASC", flowID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list delivery stages failed")
	}
	defer rows.Close()
	items := []DeliveryStage{}
	for rows.Next() {
		stage, err := scanDeliveryStage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, stage)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list delivery stages failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreatePromotion(ctx context.Context, promotion Promotion) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO promotions (id, tenant_id, project_id, application_id, freight_id, target_stage_id, target_environment_id, status, is_rollback, rollback_from_freight_id, created_by, approved_by, message, manifest_revision, created_at, updated_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		promotion.ID, promotion.TenantID, promotion.ProjectID, promotion.ApplicationID, promotion.FreightID,
		promotion.TargetStageID, promotion.TargetEnvironmentID, promotion.Status, promotion.IsRollback,
		promotion.RollbackFromFreightID, promotion.CreatedBy, promotion.ApprovedBy, promotion.Message,
		promotion.ManifestRevision, promotion.CreatedAt, promotion.UpdatedAt, promotion.CompletedAt)
	return database.ConflictOrUnavailable(err, "promotion already exists", "create promotion failed")
}

func (r *MySQLRepository) UpdatePromotion(ctx context.Context, promotion Promotion) error {
	prev, err := r.GetPromotion(ctx, promotion.ID)
	if err != nil {
		return err
	}
	if prev.ApplicationID != promotion.ApplicationID || prev.FreightID != promotion.FreightID {
		return shared.NewError(shared.CodeInvalidArgument, "promotion ownership cannot be changed")
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE promotions
SET target_stage_id = ?, target_environment_id = ?, status = ?, is_rollback = ?,
    rollback_from_freight_id = ?, created_by = ?, approved_by = ?, message = ?,
    manifest_revision = ?, updated_at = ?, completed_at = ?
WHERE id = ?`,
		promotion.TargetStageID, promotion.TargetEnvironmentID, promotion.Status, promotion.IsRollback,
		promotion.RollbackFromFreightID, promotion.CreatedBy, promotion.ApprovedBy, promotion.Message,
		promotion.ManifestRevision, promotion.UpdatedAt, promotion.CompletedAt, promotion.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update promotion failed")
	}
	return database.RequireAffected(result, "promotion not found")
}

func (r *MySQLRepository) GetPromotion(ctx context.Context, id shared.ID) (Promotion, error) {
	promotion, err := scanPromotion(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, promotionSelect()+" WHERE id = ?", id))
	if err != nil {
		return Promotion{}, database.NotFound(err, "promotion not found")
	}
	return promotion, nil
}

func (r *MySQLRepository) ListPromotionsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error) {
	return listByApplication(ctx, r.db, applicationID, page, "promotions", promotionSelect(), scanPromotion)
}

func (r *MySQLRepository) CreatePromotionApproval(ctx context.Context, approval PromotionApproval) error {
	if _, err := r.GetPromotion(ctx, approval.PromotionID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO promotion_approvals (id, tenant_id, project_id, promotion_id, approver_id, status, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		approval.ID, approval.TenantID, approval.ProjectID, approval.PromotionID, approval.ApproverID,
		approval.Status, approval.Comment, approval.CreatedAt, approval.UpdatedAt)
	return database.ConflictOrUnavailable(err, "promotion approval already exists", "create promotion approval failed")
}

func (r *MySQLRepository) UpdatePromotionApproval(ctx context.Context, approval PromotionApproval) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE promotion_approvals
SET approver_id = ?, status = ?, comment = ?, updated_at = ?
WHERE promotion_id = ?`,
		approval.ApproverID, approval.Status, approval.Comment, approval.UpdatedAt, approval.PromotionID)
	if err != nil {
		return database.WrapUnavailable(err, "update promotion approval failed")
	}
	return database.RequireAffected(result, "promotion approval not found")
}

func (r *MySQLRepository) GetPromotionApproval(ctx context.Context, promotionID shared.ID) (PromotionApproval, error) {
	approval, err := scanPromotionApproval(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, promotion_id, approver_id, status, comment, created_at, updated_at
FROM promotion_approvals WHERE promotion_id = ?`, promotionID))
	if err != nil {
		return PromotionApproval{}, database.NotFound(err, "promotion approval not found")
	}
	return approval, nil
}

type deliveryScanner interface {
	Scan(dest ...any) error
}

func releaseSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, build_run_id, build_artifact_id, version, commit_sha, image_uri, image_digest, status, created_at FROM releases"
}

func freightSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, name, status, created_at FROM freights"
}

func freightItemSelect() string {
	return "SELECT id, tenant_id, project_id, freight_id, application_id, release_id, build_artifact_id, source_key, type, name, uri, digest, created_at FROM freight_items"
}

func deliveryFlowSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, name, created_at, updated_at FROM delivery_flows"
}

func deliveryStageSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, delivery_flow_id, environment_id, name, stage_order, requires_approval, created_at, updated_at FROM delivery_stages"
}

func promotionSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, freight_id, target_stage_id, target_environment_id, status, is_rollback, rollback_from_freight_id, created_by, approved_by, message, manifest_revision, created_at, updated_at, completed_at FROM promotions"
}

func scanRelease(scanner deliveryScanner) (Release, error) {
	var v Release
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.PipelineID, &v.PipelineName, &v.PipelineDisplayName, &v.BuildRunID, &v.BuildArtifactID, &v.Version, &v.CommitSHA, &v.ImageURI, &v.ImageDigest, &v.Status, &v.CreatedAt)
	return v, err
}

func scanFreight(scanner deliveryScanner) (Freight, error) {
	var v Freight
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.PipelineID, &v.PipelineName, &v.PipelineDisplayName, &v.Name, &v.Status, &v.CreatedAt)
	return v, err
}

func scanFreightItem(scanner deliveryScanner) (FreightItem, error) {
	var v FreightItem
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.FreightID, &v.ApplicationID, &v.ReleaseID, &v.BuildArtifactID, &v.SourceKey, &v.Type, &v.Name, &v.URI, &v.Digest, &v.CreatedAt)
	return v, err
}

func scanDeliveryFlow(scanner deliveryScanner) (DeliveryFlow, error) {
	var v DeliveryFlow
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.Name, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanDeliveryStage(scanner deliveryScanner) (DeliveryStage, error) {
	var v DeliveryStage
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.DeliveryFlowID, &v.EnvironmentID, &v.Name, &v.Order, &v.RequiresApproval, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanPromotion(scanner deliveryScanner) (Promotion, error) {
	var v Promotion
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.FreightID, &v.TargetStageID, &v.TargetEnvironmentID, &v.Status, &v.IsRollback, &v.RollbackFromFreightID, &v.CreatedBy, &v.ApprovedBy, &v.Message, &v.ManifestRevision, &v.CreatedAt, &v.UpdatedAt, &v.CompletedAt)
	return v, err
}

func scanPromotionApproval(scanner deliveryScanner) (PromotionApproval, error) {
	var v PromotionApproval
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.PromotionID, &v.ApproverID, &v.Status, &v.Comment, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func listByApplication[T any](ctx context.Context, db *sql.DB, applicationID shared.ID, page shared.PageRequest, table string, selectSQL string, scan func(deliveryScanner) (T, error)) (shared.PageResult[T], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, db).QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE application_id = ?", applicationID).Scan(&total); err != nil {
		return shared.PageResult[T]{}, database.WrapUnavailable(err, "count "+table+" failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, db).QueryContext(ctx, selectSQL+" WHERE application_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?", applicationID, limit, offset)
	if err != nil {
		return shared.PageResult[T]{}, database.WrapUnavailable(err, "list "+table+" failed")
	}
	defer rows.Close()
	items := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return shared.PageResult[T]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[T]{}, database.WrapUnavailable(err, "list "+table+" failed")
	}
	return shared.NewPageResult(items, total, page), nil
}
