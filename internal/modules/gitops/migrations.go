package gitops

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605301301,
		Name:    "gitops_deployment_core",
		Up: `
CREATE TABLE deployment_templates (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  project_id VARCHAR(64) NOT NULL DEFAULT '',
  application_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  scope VARCHAR(32) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  current_version INT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_deployment_templates_application (application_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE deployment_template_revisions (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  template_id VARCHAR(64) NOT NULL,
  version INT NOT NULL,
  content MEDIUMTEXT NOT NULL,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_template_revision (template_id, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE manifest_revisions (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  deployment_id VARCHAR(64) NOT NULL,
  promotion_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  stage_key VARCHAR(64) NOT NULL DEFAULT '',
  template_revision_id VARCHAR(64) NOT NULL,
  path VARCHAR(512) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  merge_request_id VARCHAR(128) NOT NULL DEFAULT '',
  change_type VARCHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_manifest_revisions_promotion (promotion_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE deployments (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  stage_key VARCHAR(64) NOT NULL DEFAULT '',
  cluster_binding_id VARCHAR(64) NOT NULL,
  promotion_id VARCHAR(64) NOT NULL,
  freight_id VARCHAR(64) NOT NULL,
  manifest_revision_id VARCHAR(64) NOT NULL,
  image_repository VARCHAR(1024) NOT NULL,
  image_tag VARCHAR(255) NOT NULL DEFAULT '',
  image_digest VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  message VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_deployments_application_created (application_id, created_at),
  KEY idx_deployments_application_stage (application_id, stage_key),
  KEY idx_deployments_promotion (promotion_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE deployment_events (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  deployment_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  message VARCHAR(1024) NOT NULL DEFAULT '',
  occurred_at DATETIME(6) NOT NULL,
  KEY idx_deployment_events_deployment_time (deployment_id, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS deployment_events;
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS manifest_revisions;
DROP TABLE IF EXISTS deployment_template_revisions;
DROP TABLE IF EXISTS deployment_templates;
`,
	},
	{
		Version: 202606090201,
		Name:    "gitops_lookup_constraints",
		Up: `
ALTER TABLE deployment_templates
  ADD UNIQUE KEY uk_deployment_templates_platform_name (scope, name),
  ADD UNIQUE KEY uk_deployment_templates_application (scope, application_id);
ALTER TABLE deployments
  ADD UNIQUE KEY uk_deployments_promotion (promotion_id);
`,
		Down: `
ALTER TABLE deployments DROP INDEX uk_deployments_promotion;
ALTER TABLE deployment_templates DROP INDEX uk_deployment_templates_application;
ALTER TABLE deployment_templates DROP INDEX uk_deployment_templates_platform_name;
`,
	},
	{
		Version: 202606110301,
		Name:    "gitops_deployment_workload_summary",
		Up: `
SET @deployments_workload_summary_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'deployments'
    AND column_name = 'workload_summary'
);
SET @deployments_workload_summary_ddl := IF(@deployments_workload_summary_missing, 'ALTER TABLE deployments ADD COLUMN workload_summary VARCHAR(2048) NULL AFTER image_digest', 'SELECT 1');
PREPARE deployments_workload_summary_stmt FROM @deployments_workload_summary_ddl;
EXECUTE deployments_workload_summary_stmt;
DEALLOCATE PREPARE deployments_workload_summary_stmt;

UPDATE deployments SET workload_summary = '' WHERE workload_summary IS NULL;
ALTER TABLE deployments MODIFY COLUMN workload_summary VARCHAR(2048) NOT NULL DEFAULT '';
`,
		Down: `
SET @deployments_workload_summary_exists := (
  SELECT COUNT(*) > 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'deployments'
    AND column_name = 'workload_summary'
);
SET @drop_deployments_workload_summary_ddl := IF(@deployments_workload_summary_exists, 'ALTER TABLE deployments DROP COLUMN workload_summary', 'SELECT 1');
PREPARE drop_deployments_workload_summary_stmt FROM @drop_deployments_workload_summary_ddl;
EXECUTE drop_deployments_workload_summary_stmt;
DEALLOCATE PREPARE drop_deployments_workload_summary_stmt;
`,
	},
	{
		Version: 202606120903,
		Name:    "gitops_multi_cluster_promotion_targets",
		Up: `
SET @deployments_promotion_unique_exists := (
  SELECT COUNT(*) > 0
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'deployments'
    AND index_name = 'uk_deployments_promotion'
);
SET @deployments_promotion_unique_drop := IF(@deployments_promotion_unique_exists, 'ALTER TABLE deployments DROP INDEX uk_deployments_promotion', 'SELECT 1');
PREPARE deployments_promotion_unique_stmt FROM @deployments_promotion_unique_drop;
EXECUTE deployments_promotion_unique_stmt;
DEALLOCATE PREPARE deployments_promotion_unique_stmt;
`,
		Down: `SELECT 1;`,
	},
}
