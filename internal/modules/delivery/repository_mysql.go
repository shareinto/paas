package delivery

import (
	"context"
	"database/sql"
	"encoding/json"

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
INSERT INTO releases (id, tenant_id, project_id, application_id, workload_id, pipeline_id, pipeline_name, pipeline_display_name, build_run_id, build_artifact_id, image_bundle_id, version, commit_sha, image_uri, image_repository, image_tag, image_digest, source_type, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		release.ID, release.TenantID, release.ProjectID, release.ApplicationID, release.WorkloadID, release.PipelineID, release.PipelineName,
		release.PipelineDisplayName, release.BuildRunID, release.BuildArtifactID, release.ImageBundleID, release.Version, release.CommitSHA,
		release.ImageURI, release.ImageRepository, release.ImageTag, release.ImageDigest, release.SourceType, release.Status, release.CreatedAt)
	return database.ConflictOrUnavailable(err, "release already exists", "create release failed")
}

func (r *MySQLRepository) CreateImageBundle(ctx context.Context, bundle ImageBundle) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO image_bundles (id, tenant_id, project_id, application_id, workload_id, build_run_id, commit_sha, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		bundle.ID, bundle.TenantID, bundle.ProjectID, bundle.ApplicationID, bundle.WorkloadID, bundle.BuildRunID, bundle.CommitSHA, bundle.CreatedAt)
	return database.ConflictOrUnavailable(err, "image bundle already exists", "create image bundle failed")
}

func (r *MySQLRepository) CreateImageBundleImage(ctx context.Context, image ImageBundleImage) error {
	labels, err := database.MarshalJSON(image.SelectorLabels)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO image_bundle_images (id, bundle_id, build_artifact_id, runtime_environment_id, runtime_environment_name, uri, image_repository, image_tag, digest, selector_labels_json, is_primary, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		image.ID, image.BundleID, image.BuildArtifactID, image.RuntimeEnvironmentID, image.RuntimeEnvironmentName, image.URI,
		image.ImageRepository, image.ImageTag, image.Digest, string(labels), image.IsPrimary, image.CreatedAt)
	return database.ConflictOrUnavailable(err, "image bundle image already exists", "create image bundle image failed")
}

func (r *MySQLRepository) GetImageBundle(ctx context.Context, id shared.ID) (ImageBundle, error) {
	bundle, err := scanImageBundle(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, imageBundleSelect()+" WHERE id = ?", id))
	if err != nil {
		return ImageBundle{}, database.NotFound(err, "image bundle not found")
	}
	return bundle, nil
}

func (r *MySQLRepository) ListImageBundleImages(ctx context.Context, bundleID shared.ID) ([]ImageBundleImage, error) {
	if _, err := r.GetImageBundle(ctx, bundleID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, imageBundleImageSelect()+" WHERE bundle_id = ? ORDER BY is_primary DESC, created_at ASC, id ASC", bundleID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list image bundle images failed")
	}
	defer rows.Close()
	items := []ImageBundleImage{}
	for rows.Next() {
		item, err := scanImageBundleImage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list image bundle images failed")
	}
	return items, nil
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

func (r *MySQLRepository) ListReleasesByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Release], error) {
	return listByApplication(ctx, r.db, applicationID, page, "releases", releaseSelect(), scanRelease)
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
INSERT INTO freight_items (id, tenant_id, project_id, freight_id, application_id, workload_id, release_id, build_artifact_id, image_bundle_id, source_type, source_key, type, name, uri, image_ref, image_repository, image_tag, digest, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.TenantID, item.ProjectID, item.FreightID, item.ApplicationID, item.WorkloadID, item.ReleaseID,
		item.BuildArtifactID, item.ImageBundleID, item.SourceType, item.SourceKey, item.Type, item.Name, item.URI, item.ImageRef, item.ImageRepository, item.ImageTag, item.Digest, item.CreatedAt)
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

func (r *MySQLRepository) CreateDeliveryFlowTemplate(ctx context.Context, template DeliveryFlowTemplate) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO delivery_flow_templates (id, tenant_id, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`,
		template.ID, template.TenantID, template.Name, template.CreatedAt, template.UpdatedAt)
	return database.ConflictOrUnavailable(err, "delivery flow template already exists", "create delivery flow template failed")
}

func (r *MySQLRepository) FindDeliveryFlowTemplateByTenant(ctx context.Context, tenantID shared.ID) (DeliveryFlowTemplate, error) {
	template, err := scanDeliveryFlowTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryFlowTemplateSelect()+" WHERE tenant_id = ?", tenantID))
	if err != nil {
		return DeliveryFlowTemplate{}, database.NotFound(err, "delivery flow template not found")
	}
	return template, nil
}

func (r *MySQLRepository) CreateDeliveryFlowTemplateStage(ctx context.Context, stage DeliveryFlowTemplateStage) error {
	approveRoles, err := json.Marshal(stage.ApproveRoles)
	if err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "approve roles is invalid", err)
	}
	verifyRoles, err := json.Marshal(stage.VerifyRoles)
	if err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "verify roles is invalid", err)
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO delivery_flow_template_stages (
  id, tenant_id, template_id, stage_key, display_name, color, stage_order, layout_column, layout_row, status,
  requires_approval, requires_verification, approve_roles_json, verify_roles_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), ?, ?)`,
		stage.ID, stage.TenantID, stage.TemplateID, stage.StageKey, stage.DisplayName, stage.Color, stage.Order,
		stage.LayoutColumn, stage.LayoutRow, stage.Status, stage.RequiresApproval, stage.RequiresVerification, string(approveRoles), string(verifyRoles),
		stage.CreatedAt, stage.UpdatedAt)
	return database.ConflictOrUnavailable(err, "delivery flow template stage already exists", "create delivery flow template stage failed")
}

func (r *MySQLRepository) UpdateDeliveryFlowTemplateStage(ctx context.Context, stage DeliveryFlowTemplateStage) error {
	approveRoles, err := json.Marshal(stage.ApproveRoles)
	if err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "approve roles is invalid", err)
	}
	verifyRoles, err := json.Marshal(stage.VerifyRoles)
	if err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "verify roles is invalid", err)
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE delivery_flow_template_stages
SET display_name = ?, color = ?, stage_order = ?, layout_column = ?, layout_row = ?, status = ?, requires_approval = ?,
    requires_verification = ?, approve_roles_json = CAST(? AS JSON), verify_roles_json = CAST(? AS JSON), updated_at = ?
WHERE tenant_id = ? AND stage_key = ?`,
		stage.DisplayName, stage.Color, stage.Order, stage.LayoutColumn, stage.LayoutRow, stage.Status, stage.RequiresApproval, stage.RequiresVerification,
		string(approveRoles), string(verifyRoles), stage.UpdatedAt, stage.TenantID, stage.StageKey)
	if err != nil {
		return database.WrapUnavailable(err, "update delivery flow template stage failed")
	}
	return database.RequireAffected(result, "delivery flow template stage not found")
}

func (r *MySQLRepository) DeleteDeliveryFlowTemplateStage(ctx context.Context, tenantID shared.ID, stageKey string) error {
	return database.NewTransactor(r.db).WithinTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, r.db)
		if _, err := exec.ExecContext(txCtx, "DELETE FROM stage_cluster_bindings WHERE tenant_id = ? AND stage_key = ?", tenantID, stageKey); err != nil {
			return database.WrapUnavailable(err, "delete delivery flow template stage failed")
		}
		if _, err := exec.ExecContext(txCtx, "DELETE FROM delivery_flow_template_edges WHERE tenant_id = ? AND (from_stage_key = ? OR to_stage_key = ?)", tenantID, stageKey, stageKey); err != nil {
			return database.WrapUnavailable(err, "delete delivery flow template stage failed")
		}
		result, err := exec.ExecContext(txCtx, "DELETE FROM delivery_flow_template_stages WHERE tenant_id = ? AND stage_key = ?", tenantID, stageKey)
		if err != nil {
			return database.WrapUnavailable(err, "delete delivery flow template stage failed")
		}
		return database.RequireAffected(result, "delivery flow template stage not found")
	})
}

func (r *MySQLRepository) FindDeliveryFlowTemplateStage(ctx context.Context, tenantID shared.ID, stageKey string) (DeliveryFlowTemplateStage, error) {
	stage, err := scanDeliveryFlowTemplateStage(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, deliveryFlowTemplateStageSelect()+" WHERE tenant_id = ? AND stage_key = ?", tenantID, stageKey))
	if err != nil {
		return DeliveryFlowTemplateStage{}, database.NotFound(err, "delivery flow template stage not found")
	}
	return stage, nil
}

func (r *MySQLRepository) ListDeliveryFlowTemplateStages(ctx context.Context, templateID shared.ID) ([]DeliveryFlowTemplateStage, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, deliveryFlowTemplateStageSelect()+" WHERE template_id = ? ORDER BY stage_order ASC, created_at ASC", templateID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list delivery flow template stages failed")
	}
	defer rows.Close()
	items := []DeliveryFlowTemplateStage{}
	for rows.Next() {
		stage, err := scanDeliveryFlowTemplateStage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, stage)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list delivery flow template stages failed")
	}
	return items, nil
}

func (r *MySQLRepository) ReplaceDeliveryFlowTemplateEdges(ctx context.Context, templateID shared.ID, edges []DeliveryFlowTemplateEdge) error {
	return database.NewTransactor(r.db).WithinTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, r.db)
		if _, err := exec.ExecContext(txCtx, "DELETE FROM delivery_flow_template_edges WHERE template_id = ?", templateID); err != nil {
			return database.WrapUnavailable(err, "replace delivery flow template edges failed")
		}
		for _, edge := range edges {
			_, err := exec.ExecContext(txCtx, `
INSERT INTO delivery_flow_template_edges (id, tenant_id, template_id, from_stage_key, to_stage_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
				edge.ID, edge.TenantID, edge.TemplateID, edge.FromStageKey, edge.ToStageKey, edge.CreatedAt, edge.UpdatedAt)
			if err != nil {
				return database.ConflictOrUnavailable(err, "delivery flow template edge already exists", "replace delivery flow template edges failed")
			}
		}
		return nil
	})
}

func (r *MySQLRepository) ListDeliveryFlowTemplateEdges(ctx context.Context, templateID shared.ID) ([]DeliveryFlowTemplateEdge, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, deliveryFlowTemplateEdgeSelect()+" WHERE template_id = ? ORDER BY from_stage_key ASC, to_stage_key ASC", templateID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list delivery flow template edges failed")
	}
	defer rows.Close()
	items := []DeliveryFlowTemplateEdge{}
	for rows.Next() {
		edge, err := scanDeliveryFlowTemplateEdge(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list delivery flow template edges failed")
	}
	return items, nil
}

func (r *MySQLRepository) ReplaceStageClusterBindings(ctx context.Context, tenantID shared.ID, stageKey string, bindings []StageClusterBinding) error {
	exec := database.ExecutorFromContext(ctx, r.db)
	if _, err := exec.ExecContext(ctx, "DELETE FROM stage_cluster_bindings WHERE tenant_id = ? AND stage_key = ?", tenantID, stageKey); err != nil {
		return database.WrapUnavailable(err, "replace stage cluster bindings failed")
	}
	for _, binding := range bindings {
		_, err := exec.ExecContext(ctx, `
INSERT INTO stage_cluster_bindings (id, tenant_id, stage_key, cluster_id, cluster_name, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			binding.ID, binding.TenantID, binding.StageKey, binding.ClusterID, binding.ClusterName, binding.Status, binding.CreatedAt, binding.UpdatedAt)
		if err != nil {
			return database.ConflictOrUnavailable(err, "stage cluster binding already exists", "replace stage cluster bindings failed")
		}
	}
	return nil
}

func (r *MySQLRepository) ListStageClusterBindings(ctx context.Context, tenantID shared.ID, stageKey string) ([]StageClusterBinding, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, stageClusterBindingSelect()+" WHERE tenant_id = ? AND stage_key = ? ORDER BY cluster_name ASC, cluster_id ASC", tenantID, stageKey)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list stage cluster bindings failed")
	}
	defer rows.Close()
	items := []StageClusterBinding{}
	for rows.Next() {
		binding, err := scanStageClusterBinding(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list stage cluster bindings failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateFreightApproval(ctx context.Context, approval FreightApproval) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO freight_approvals (id, tenant_id, project_id, application_id, freight_id, target_stage_key, approver_id, status, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		approval.ID, approval.TenantID, approval.ProjectID, approval.ApplicationID, approval.FreightID, approval.TargetStageKey,
		approval.ApproverID, approval.Status, approval.Comment, approval.CreatedAt, approval.UpdatedAt)
	return database.ConflictOrUnavailable(err, "freight approval already exists", "create freight approval failed")
}

func (r *MySQLRepository) FindFreightApproval(ctx context.Context, freightID shared.ID, targetStageKey string) (FreightApproval, error) {
	approval, err := scanFreightApproval(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, freightApprovalSelect()+" WHERE freight_id = ? AND target_stage_key = ?", freightID, targetStageKey))
	if err != nil {
		return FreightApproval{}, database.NotFound(err, "freight approval not found")
	}
	return approval, nil
}

func (r *MySQLRepository) CreateStageVerification(ctx context.Context, verification StageVerification) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO stage_verifications (id, tenant_id, project_id, application_id, stage_key, freight_id, verifier_id, status, comment, sync_status, health_status, agent_status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		verification.ID, verification.TenantID, verification.ProjectID, verification.ApplicationID, verification.StageKey, verification.FreightID,
		verification.VerifierID, verification.Status, verification.Comment, verification.SyncStatus, verification.HealthStatus, verification.AgentStatus,
		verification.CreatedAt, verification.UpdatedAt)
	return database.ConflictOrUnavailable(err, "stage verification already exists", "create stage verification failed")
}

func (r *MySQLRepository) FindStageVerification(ctx context.Context, applicationID shared.ID, stageKey string, freightID shared.ID) (StageVerification, error) {
	verification, err := scanStageVerification(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, stageVerificationSelect()+" WHERE application_id = ? AND stage_key = ? AND freight_id = ?", applicationID, stageKey, freightID))
	if err != nil {
		return StageVerification{}, database.NotFound(err, "stage verification not found")
	}
	return verification, nil
}

func (r *MySQLRepository) CreatePromotion(ctx context.Context, promotion Promotion) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO promotions (id, tenant_id, project_id, application_id, freight_id, target_stage_id, target_environment_id, target_stage_key, namespace_override, status, is_rollback, rollback_from_freight_id, created_by, approved_by, message, manifest_revision, created_at, updated_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		promotion.ID, promotion.TenantID, promotion.ProjectID, promotion.ApplicationID, promotion.FreightID,
		promotion.TargetStageID, promotion.TargetEnvironmentID, promotion.TargetStageKey, promotion.NamespaceOverride, promotion.Status, promotion.IsRollback,
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
    target_stage_key = ?, namespace_override = ?, rollback_from_freight_id = ?, created_by = ?, approved_by = ?, message = ?,
    manifest_revision = ?, updated_at = ?, completed_at = ?
WHERE id = ?`,
		promotion.TargetStageID, promotion.TargetEnvironmentID, promotion.Status, promotion.IsRollback,
		promotion.TargetStageKey, promotion.NamespaceOverride,
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
	return "SELECT id, tenant_id, project_id, application_id, workload_id, pipeline_id, pipeline_name, pipeline_display_name, build_run_id, build_artifact_id, image_bundle_id, version, commit_sha, image_uri, image_repository, image_tag, image_digest, source_type, status, created_at FROM releases"
}

func freightSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, name, status, created_at FROM freights"
}

func freightItemSelect() string {
	return "SELECT id, tenant_id, project_id, freight_id, application_id, workload_id, release_id, build_artifact_id, image_bundle_id, source_type, source_key, type, name, uri, image_ref, image_repository, image_tag, digest, created_at FROM freight_items"
}

func imageBundleSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, workload_id, build_run_id, commit_sha, created_at FROM image_bundles"
}

func imageBundleImageSelect() string {
	return "SELECT id, bundle_id, build_artifact_id, runtime_environment_id, runtime_environment_name, uri, image_repository, image_tag, digest, selector_labels_json, is_primary, created_at FROM image_bundle_images"
}

func deliveryFlowSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, name, created_at, updated_at FROM delivery_flows"
}

func deliveryStageSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, delivery_flow_id, environment_id, name, stage_order, requires_approval, created_at, updated_at FROM delivery_stages"
}

func deliveryFlowTemplateSelect() string {
	return "SELECT id, tenant_id, name, created_at, updated_at FROM delivery_flow_templates"
}

func deliveryFlowTemplateStageSelect() string {
	return "SELECT id, tenant_id, template_id, stage_key, display_name, color, stage_order, layout_column, layout_row, status, requires_approval, requires_verification, approve_roles_json, verify_roles_json, created_at, updated_at FROM delivery_flow_template_stages"
}

func deliveryFlowTemplateEdgeSelect() string {
	return "SELECT id, tenant_id, template_id, from_stage_key, to_stage_key, created_at, updated_at FROM delivery_flow_template_edges"
}

func stageClusterBindingSelect() string {
	return "SELECT id, tenant_id, stage_key, cluster_id, cluster_name, status, created_at, updated_at FROM stage_cluster_bindings"
}

func freightApprovalSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, freight_id, target_stage_key, approver_id, status, comment, created_at, updated_at FROM freight_approvals"
}

func stageVerificationSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, stage_key, freight_id, verifier_id, status, comment, sync_status, health_status, agent_status, created_at, updated_at FROM stage_verifications"
}

func promotionSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, freight_id, target_stage_id, target_environment_id, target_stage_key, namespace_override, status, is_rollback, rollback_from_freight_id, created_by, approved_by, message, manifest_revision, created_at, updated_at, completed_at FROM promotions"
}

func scanRelease(scanner deliveryScanner) (Release, error) {
	var v Release
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.WorkloadID, &v.PipelineID, &v.PipelineName, &v.PipelineDisplayName, &v.BuildRunID, &v.BuildArtifactID, &v.ImageBundleID, &v.Version, &v.CommitSHA, &v.ImageURI, &v.ImageRepository, &v.ImageTag, &v.ImageDigest, &v.SourceType, &v.Status, &v.CreatedAt)
	return v, err
}

func scanFreight(scanner deliveryScanner) (Freight, error) {
	var v Freight
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.PipelineID, &v.PipelineName, &v.PipelineDisplayName, &v.Name, &v.Status, &v.CreatedAt)
	return v, err
}

func scanFreightItem(scanner deliveryScanner) (FreightItem, error) {
	var v FreightItem
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.FreightID, &v.ApplicationID, &v.WorkloadID, &v.ReleaseID, &v.BuildArtifactID, &v.ImageBundleID, &v.SourceType, &v.SourceKey, &v.Type, &v.Name, &v.URI, &v.ImageRef, &v.ImageRepository, &v.ImageTag, &v.Digest, &v.CreatedAt)
	return v, err
}

func scanImageBundle(scanner deliveryScanner) (ImageBundle, error) {
	var v ImageBundle
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.WorkloadID, &v.BuildRunID, &v.CommitSHA, &v.CreatedAt)
	return v, err
}

func scanImageBundleImage(scanner deliveryScanner) (ImageBundleImage, error) {
	var v ImageBundleImage
	var labels []byte
	if err := scanner.Scan(&v.ID, &v.BundleID, &v.BuildArtifactID, &v.RuntimeEnvironmentID, &v.RuntimeEnvironmentName, &v.URI, &v.ImageRepository, &v.ImageTag, &v.Digest, &labels, &v.IsPrimary, &v.CreatedAt); err != nil {
		return ImageBundleImage{}, err
	}
	if err := database.UnmarshalJSON(labels, &v.SelectorLabels); err != nil {
		return ImageBundleImage{}, err
	}
	return v, nil
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

func scanDeliveryFlowTemplate(scanner deliveryScanner) (DeliveryFlowTemplate, error) {
	var v DeliveryFlowTemplate
	err := scanner.Scan(&v.ID, &v.TenantID, &v.Name, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanDeliveryFlowTemplateStage(scanner deliveryScanner) (DeliveryFlowTemplateStage, error) {
	var v DeliveryFlowTemplateStage
	var approveRoles, verifyRoles []byte
	err := scanner.Scan(&v.ID, &v.TenantID, &v.TemplateID, &v.StageKey, &v.DisplayName, &v.Color, &v.Order, &v.LayoutColumn, &v.LayoutRow, &v.Status, &v.RequiresApproval, &v.RequiresVerification, &approveRoles, &verifyRoles, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	if len(approveRoles) > 0 {
		if err := json.Unmarshal(approveRoles, &v.ApproveRoles); err != nil {
			return v, shared.WrapError(shared.CodeUnavailable, "scan approve roles failed", err)
		}
	}
	if len(verifyRoles) > 0 {
		if err := json.Unmarshal(verifyRoles, &v.VerifyRoles); err != nil {
			return v, shared.WrapError(shared.CodeUnavailable, "scan verify roles failed", err)
		}
	}
	return v, nil
}

func scanDeliveryFlowTemplateEdge(scanner deliveryScanner) (DeliveryFlowTemplateEdge, error) {
	var v DeliveryFlowTemplateEdge
	err := scanner.Scan(&v.ID, &v.TenantID, &v.TemplateID, &v.FromStageKey, &v.ToStageKey, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanStageClusterBinding(scanner deliveryScanner) (StageClusterBinding, error) {
	var v StageClusterBinding
	err := scanner.Scan(&v.ID, &v.TenantID, &v.StageKey, &v.ClusterID, &v.ClusterName, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanFreightApproval(scanner deliveryScanner) (FreightApproval, error) {
	var v FreightApproval
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.FreightID, &v.TargetStageKey, &v.ApproverID, &v.Status, &v.Comment, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanStageVerification(scanner deliveryScanner) (StageVerification, error) {
	var v StageVerification
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.StageKey, &v.FreightID, &v.VerifierID, &v.Status, &v.Comment, &v.SyncStatus, &v.HealthStatus, &v.AgentStatus, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanPromotion(scanner deliveryScanner) (Promotion, error) {
	var v Promotion
	err := scanner.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.ApplicationID, &v.FreightID, &v.TargetStageID, &v.TargetEnvironmentID, &v.TargetStageKey, &v.NamespaceOverride, &v.Status, &v.IsRollback, &v.RollbackFromFreightID, &v.CreatedBy, &v.ApprovedBy, &v.Message, &v.ManifestRevision, &v.CreatedAt, &v.UpdatedAt, &v.CompletedAt)
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
