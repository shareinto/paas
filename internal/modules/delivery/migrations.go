package delivery

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{{
	Version: 202605300701,
	Name:    "release_delivery_core",
	Up: `
CREATE TABLE releases (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  workload_id VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_id VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_name VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '',
  build_run_id VARCHAR(64) NOT NULL,
  build_artifact_id VARCHAR(64) NOT NULL,
  image_bundle_id VARCHAR(64) NOT NULL DEFAULT '',
  version VARCHAR(128) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  image_uri VARCHAR(1024) NOT NULL,
  image_repository VARCHAR(1024) NOT NULL DEFAULT '',
  image_tag VARCHAR(255) NOT NULL DEFAULT '',
  image_digest VARCHAR(255) NOT NULL DEFAULT '',
  source_type VARCHAR(64) NOT NULL DEFAULT 'pipeline_artifact',
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_releases_build_run (build_run_id),
  KEY idx_releases_application (application_id),
  KEY idx_releases_workload_created (application_id, workload_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE freights (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  pipeline_id VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_name VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_freights_application_created (application_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE freight_items (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  freight_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  workload_id VARCHAR(64) NOT NULL DEFAULT '',
  release_id VARCHAR(64) NOT NULL,
  build_artifact_id VARCHAR(64) NOT NULL,
  image_bundle_id VARCHAR(64) NOT NULL DEFAULT '',
  source_type VARCHAR(64) NOT NULL DEFAULT 'pipeline_artifact',
  source_key VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  uri VARCHAR(1024) NOT NULL,
  image_ref VARCHAR(1024) NOT NULL DEFAULT '',
  image_repository VARCHAR(1024) NOT NULL DEFAULT '',
  image_tag VARCHAR(255) NOT NULL DEFAULT '',
  digest VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_freight_items_workload (freight_id, workload_id),
  KEY idx_freight_items_freight (freight_id),
  CONSTRAINT fk_freight_items_freight FOREIGN KEY (freight_id) REFERENCES freights(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_flows (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_flows_application (application_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_stages (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  delivery_flow_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL,
  stage_order INT NOT NULL,
  requires_approval TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_stages_environment (environment_id),
  KEY idx_delivery_stages_flow (delivery_flow_id),
  CONSTRAINT fk_delivery_stages_flow FOREIGN KEY (delivery_flow_id) REFERENCES delivery_flows(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE promotions (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  freight_id VARCHAR(64) NOT NULL,
  target_stage_id VARCHAR(64) NOT NULL,
  target_environment_id VARCHAR(64) NOT NULL,
  target_stage_key VARCHAR(64) NOT NULL DEFAULT '',
  namespace_override VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  is_rollback TINYINT(1) NOT NULL DEFAULT 0,
  rollback_from_freight_id VARCHAR(64) NOT NULL DEFAULT '',
  created_by VARCHAR(64) NOT NULL,
  approved_by VARCHAR(64) NOT NULL DEFAULT '',
  message VARCHAR(1024) NOT NULL DEFAULT '',
  manifest_revision VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_promotions_application_created (application_id, created_at),
  KEY idx_promotions_freight (freight_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE promotion_approvals (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  promotion_id VARCHAR(64) NOT NULL,
  approver_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  comment VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_promotion_approvals_promotion (promotion_id),
  CONSTRAINT fk_promotion_approvals_promotion FOREIGN KEY (promotion_id) REFERENCES promotions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
	Down: `
DROP TABLE IF EXISTS promotion_approvals;
DROP TABLE IF EXISTS promotions;
DROP TABLE IF EXISTS delivery_stages;
DROP TABLE IF EXISTS delivery_flows;
DROP TABLE IF EXISTS freight_items;
DROP TABLE IF EXISTS freights;
DROP TABLE IF EXISTS releases;
`,
}, {
	Version: 202606121403,
	Name:    "image_bundle_delivery",
	Up: `
CREATE TABLE IF NOT EXISTS image_bundles (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  workload_id VARCHAR(64) NOT NULL,
  build_run_id VARCHAR(64) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_image_bundles_build_run (build_run_id),
  KEY idx_image_bundles_workload_created (application_id, workload_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS image_bundle_images (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  bundle_id VARCHAR(64) NOT NULL,
  build_artifact_id VARCHAR(64) NOT NULL,
  runtime_environment_id VARCHAR(64) NOT NULL DEFAULT '',
  runtime_environment_name VARCHAR(128) NOT NULL DEFAULT '',
  uri VARCHAR(1024) NOT NULL,
  image_repository VARCHAR(1024) NOT NULL DEFAULT '',
  image_tag VARCHAR(255) NOT NULL DEFAULT '',
  digest VARCHAR(255) NOT NULL DEFAULT '',
  selector_labels_json JSON NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_image_bundle_images_artifact (bundle_id, build_artifact_id),
  KEY idx_image_bundle_images_bundle (bundle_id),
  CONSTRAINT fk_image_bundle_images_bundle FOREIGN KEY (bundle_id) REFERENCES image_bundles(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET @releases_bundle_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'releases' AND column_name = 'image_bundle_id'
);
SET @releases_bundle_ddl := IF(@releases_bundle_missing, 'ALTER TABLE releases ADD COLUMN image_bundle_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER build_artifact_id', 'SELECT 1');
PREPARE releases_bundle_stmt FROM @releases_bundle_ddl;
EXECUTE releases_bundle_stmt;
DEALLOCATE PREPARE releases_bundle_stmt;

SET @freight_items_bundle_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'image_bundle_id'
);
SET @freight_items_bundle_ddl := IF(@freight_items_bundle_missing, 'ALTER TABLE freight_items ADD COLUMN image_bundle_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER build_artifact_id', 'SELECT 1');
PREPARE freight_items_bundle_stmt FROM @freight_items_bundle_ddl;
EXECUTE freight_items_bundle_stmt;
DEALLOCATE PREPARE freight_items_bundle_stmt;
`,
	Down: `SELECT 1;`,
}, {
	Version: 202606100302,
	Name:    "release_freight_workload_v2_columns",
	Up: `
SET @releases_workload_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'releases' AND column_name = 'workload_id'
);
SET @releases_workload_ddl := IF(@releases_workload_missing, 'ALTER TABLE releases ADD COLUMN workload_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER application_id', 'SELECT 1');
PREPARE releases_workload_stmt FROM @releases_workload_ddl;
EXECUTE releases_workload_stmt;
DEALLOCATE PREPARE releases_workload_stmt;

SET @releases_repo_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'releases' AND column_name = 'image_repository'
);
SET @releases_repo_ddl := IF(@releases_repo_missing, 'ALTER TABLE releases ADD COLUMN image_repository VARCHAR(1024) NOT NULL DEFAULT '''' AFTER image_uri', 'SELECT 1');
PREPARE releases_repo_stmt FROM @releases_repo_ddl;
EXECUTE releases_repo_stmt;
DEALLOCATE PREPARE releases_repo_stmt;

SET @releases_tag_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'releases' AND column_name = 'image_tag'
);
SET @releases_tag_ddl := IF(@releases_tag_missing, 'ALTER TABLE releases ADD COLUMN image_tag VARCHAR(255) NOT NULL DEFAULT '''' AFTER image_repository', 'SELECT 1');
PREPARE releases_tag_stmt FROM @releases_tag_ddl;
EXECUTE releases_tag_stmt;
DEALLOCATE PREPARE releases_tag_stmt;

SET @releases_source_type_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'releases' AND column_name = 'source_type'
);
SET @releases_source_type_ddl := IF(@releases_source_type_missing, 'ALTER TABLE releases ADD COLUMN source_type VARCHAR(64) NOT NULL DEFAULT ''pipeline_artifact'' AFTER image_digest', 'SELECT 1');
PREPARE releases_source_type_stmt FROM @releases_source_type_ddl;
EXECUTE releases_source_type_stmt;
DEALLOCATE PREPARE releases_source_type_stmt;

SET @freight_items_workload_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'workload_id'
);
SET @freight_items_workload_ddl := IF(@freight_items_workload_missing, 'ALTER TABLE freight_items ADD COLUMN workload_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER application_id', 'SELECT 1');
PREPARE freight_items_workload_stmt FROM @freight_items_workload_ddl;
EXECUTE freight_items_workload_stmt;
DEALLOCATE PREPARE freight_items_workload_stmt;

SET @freight_items_source_type_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'source_type'
);
SET @freight_items_source_type_ddl := IF(@freight_items_source_type_missing, 'ALTER TABLE freight_items ADD COLUMN source_type VARCHAR(64) NOT NULL DEFAULT ''pipeline_artifact'' AFTER build_artifact_id', 'SELECT 1');
PREPARE freight_items_source_type_stmt FROM @freight_items_source_type_ddl;
EXECUTE freight_items_source_type_stmt;
DEALLOCATE PREPARE freight_items_source_type_stmt;

SET @freight_items_image_ref_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'image_ref'
);
SET @freight_items_image_ref_ddl := IF(@freight_items_image_ref_missing, 'ALTER TABLE freight_items ADD COLUMN image_ref VARCHAR(1024) NOT NULL DEFAULT '''' AFTER uri', 'SELECT 1');
PREPARE freight_items_image_ref_stmt FROM @freight_items_image_ref_ddl;
EXECUTE freight_items_image_ref_stmt;
DEALLOCATE PREPARE freight_items_image_ref_stmt;

SET @freight_items_repo_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'image_repository'
);
SET @freight_items_repo_ddl := IF(@freight_items_repo_missing, 'ALTER TABLE freight_items ADD COLUMN image_repository VARCHAR(1024) NOT NULL DEFAULT '''' AFTER image_ref', 'SELECT 1');
PREPARE freight_items_repo_stmt FROM @freight_items_repo_ddl;
EXECUTE freight_items_repo_stmt;
DEALLOCATE PREPARE freight_items_repo_stmt;

SET @freight_items_tag_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'freight_items' AND column_name = 'image_tag'
);
SET @freight_items_tag_ddl := IF(@freight_items_tag_missing, 'ALTER TABLE freight_items ADD COLUMN image_tag VARCHAR(255) NOT NULL DEFAULT '''' AFTER image_repository', 'SELECT 1');
PREPARE freight_items_tag_stmt FROM @freight_items_tag_ddl;
EXECUTE freight_items_tag_stmt;
DEALLOCATE PREPARE freight_items_tag_stmt;
`,
	Down: `SELECT 1;`,
}, {
	Version: 202606120900,
	Name:    "promotion_stage_cluster_target_columns",
	Up: `
SET @promotions_stage_key_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'promotions' AND column_name = 'target_stage_key'
);
SET @promotions_stage_key_ddl := IF(@promotions_stage_key_missing, 'ALTER TABLE promotions ADD COLUMN target_stage_key VARCHAR(64) NOT NULL DEFAULT '''' AFTER target_environment_id', 'SELECT 1');
PREPARE promotions_stage_key_stmt FROM @promotions_stage_key_ddl;
EXECUTE promotions_stage_key_stmt;
DEALLOCATE PREPARE promotions_stage_key_stmt;

SET @promotions_namespace_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'promotions' AND column_name = 'namespace_override'
);
SET @promotions_namespace_ddl := IF(@promotions_namespace_missing, 'ALTER TABLE promotions ADD COLUMN namespace_override VARCHAR(255) NOT NULL DEFAULT '''' AFTER target_stage_key', 'SELECT 1');
PREPARE promotions_namespace_stmt FROM @promotions_namespace_ddl;
EXECUTE promotions_namespace_stmt;
DEALLOCATE PREPARE promotions_namespace_stmt;
`,
	Down: `SELECT 1;`,
}, {
	Version: 202606120901,
	Name:    "stage_delivery_flow_templates",
	Up: `
CREATE TABLE delivery_flow_templates (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_flow_templates_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_flow_template_stages (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  template_id VARCHAR(64) NOT NULL,
  stage_key VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  color VARCHAR(32) NOT NULL,
  stage_order INT NOT NULL,
  layout_column INT NOT NULL DEFAULT 0,
  layout_row INT NOT NULL DEFAULT 0,
  status VARCHAR(64) NOT NULL,
  requires_approval TINYINT(1) NOT NULL DEFAULT 0,
  requires_verification TINYINT(1) NOT NULL DEFAULT 0,
  approve_roles_json JSON NOT NULL,
  verify_roles_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_flow_template_stages_key (tenant_id, stage_key),
  KEY idx_delivery_flow_template_stages_template (template_id),
  CONSTRAINT fk_delivery_flow_template_stages_template FOREIGN KEY (template_id) REFERENCES delivery_flow_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_flow_template_edges (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  template_id VARCHAR(64) NOT NULL,
  from_stage_key VARCHAR(64) NOT NULL,
  to_stage_key VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_flow_template_edges_pair (template_id, from_stage_key, to_stage_key),
  KEY idx_delivery_flow_template_edges_template (template_id),
  KEY idx_delivery_flow_template_edges_to (template_id, to_stage_key),
  CONSTRAINT fk_delivery_flow_template_edges_template FOREIGN KEY (template_id) REFERENCES delivery_flow_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE stage_cluster_bindings (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  stage_key VARCHAR(64) NOT NULL,
  cluster_id VARCHAR(64) NOT NULL,
  cluster_name VARCHAR(128) NOT NULL,
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_stage_cluster_bindings_cluster (tenant_id, stage_key, cluster_id),
  KEY idx_stage_cluster_bindings_stage (tenant_id, stage_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE freight_approvals (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  freight_id VARCHAR(64) NOT NULL,
  target_stage_key VARCHAR(64) NOT NULL,
  approver_id VARCHAR(64) NOT NULL,
  status VARCHAR(64) NOT NULL,
  comment VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_freight_approvals_target (freight_id, target_stage_key),
  KEY idx_freight_approvals_application (application_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE stage_verifications (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  stage_key VARCHAR(64) NOT NULL,
  freight_id VARCHAR(64) NOT NULL,
  verifier_id VARCHAR(64) NOT NULL,
  status VARCHAR(64) NOT NULL,
  comment VARCHAR(1024) NOT NULL DEFAULT '',
  sync_status VARCHAR(64) NOT NULL DEFAULT '',
  health_status VARCHAR(64) NOT NULL DEFAULT '',
  agent_status VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_stage_verifications_target (application_id, stage_key, freight_id),
  KEY idx_stage_verifications_application (application_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
	Down: `
DROP TABLE IF EXISTS stage_verifications;
DROP TABLE IF EXISTS freight_approvals;
DROP TABLE IF EXISTS stage_cluster_bindings;
DROP TABLE IF EXISTS delivery_flow_template_edges;
DROP TABLE IF EXISTS delivery_flow_template_stages;
DROP TABLE IF EXISTS delivery_flow_templates;
`,
}, {
	Version: 202606121650,
	Name:    "delivery_flow_template_dag_edges",
	Up: `
CREATE TABLE IF NOT EXISTS delivery_flow_template_edges (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  template_id VARCHAR(64) NOT NULL,
  from_stage_key VARCHAR(64) NOT NULL,
  to_stage_key VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_delivery_flow_template_edges_pair (template_id, from_stage_key, to_stage_key),
  KEY idx_delivery_flow_template_edges_template (template_id),
  KEY idx_delivery_flow_template_edges_to (template_id, to_stage_key),
  CONSTRAINT fk_delivery_flow_template_edges_template FOREIGN KEY (template_id) REFERENCES delivery_flow_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
	Down: `SELECT 1;`,
}, {
	Version: 202606122245,
	Name:    "delivery_flow_template_stage_layout_slots",
	Up: `
SET @layout_column_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'delivery_flow_template_stages' AND column_name = 'layout_column'
);
SET @layout_column_ddl := IF(@layout_column_missing, 'ALTER TABLE delivery_flow_template_stages ADD COLUMN layout_column INT NOT NULL DEFAULT 0 AFTER stage_order', 'SELECT 1');
PREPARE stmt FROM @layout_column_ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @layout_row_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'delivery_flow_template_stages' AND column_name = 'layout_row'
);
SET @layout_row_ddl := IF(@layout_row_missing, 'ALTER TABLE delivery_flow_template_stages ADD COLUMN layout_row INT NOT NULL DEFAULT 0 AFTER layout_column', 'SELECT 1');
PREPARE stmt FROM @layout_row_ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
`,
	Down: `SELECT 1;`,
}, {
	Version: 202606130130,
	Name:    "stage_cluster_bindings_single_cluster_per_stage",
	Up: `
SET @stage_binding_unique_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'stage_cluster_bindings' AND index_name = 'uk_stage_cluster_bindings_stage_unique'
);
SET @stage_binding_unique_ddl := IF(@stage_binding_unique_missing, 'ALTER TABLE stage_cluster_bindings ADD UNIQUE KEY uk_stage_cluster_bindings_stage_unique (tenant_id, stage_key)', 'SELECT 1');
PREPARE stmt FROM @stage_binding_unique_ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
`,
	Down: `
SET @stage_binding_unique_exists := (
  SELECT COUNT(*) > 0 FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'stage_cluster_bindings' AND index_name = 'uk_stage_cluster_bindings_stage_unique'
);
SET @stage_binding_unique_drop := IF(@stage_binding_unique_exists, 'ALTER TABLE stage_cluster_bindings DROP INDEX uk_stage_cluster_bindings_stage_unique', 'SELECT 1');
PREPARE stmt FROM @stage_binding_unique_drop; EXECUTE stmt; DEALLOCATE PREPARE stmt;
`,
}}
