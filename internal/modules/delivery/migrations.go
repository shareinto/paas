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
  pipeline_id VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_name VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '',
  build_run_id VARCHAR(64) NOT NULL,
  build_artifact_id VARCHAR(64) NOT NULL,
  version VARCHAR(128) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  image_uri VARCHAR(1024) NOT NULL,
  image_digest VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_releases_build_run (build_run_id),
  KEY idx_releases_application (application_id)
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
  release_id VARCHAR(64) NOT NULL,
  build_artifact_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  uri VARCHAR(1024) NOT NULL,
  digest VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
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
}}
