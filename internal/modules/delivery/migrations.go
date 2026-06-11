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
}}
