package build

import "github.com/shareinto/paas/internal/platform/database"

const backfillBuildPipelineIdentityColumnsSQL = `
SET @build_pipelines_name_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'name'
);
SET @build_pipelines_name_ddl := IF(@build_pipelines_name_missing, 'ALTER TABLE build_pipelines ADD COLUMN name VARCHAR(64) NOT NULL DEFAULT '''' AFTER application_id', 'SELECT 1');
PREPARE build_pipelines_name_stmt FROM @build_pipelines_name_ddl;
EXECUTE build_pipelines_name_stmt;
DEALLOCATE PREPARE build_pipelines_name_stmt;

SET @build_pipelines_display_name_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'display_name'
);
SET @build_pipelines_display_name_ddl := IF(@build_pipelines_display_name_missing, 'ALTER TABLE build_pipelines ADD COLUMN display_name VARCHAR(128) NOT NULL DEFAULT '''' AFTER name', 'SELECT 1');
PREPARE build_pipelines_display_name_stmt FROM @build_pipelines_display_name_ddl;
EXECUTE build_pipelines_display_name_stmt;
DEALLOCATE PREPARE build_pipelines_display_name_stmt;

SET @build_pipelines_description_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'description'
);
SET @build_pipelines_description_ddl := IF(@build_pipelines_description_missing, 'ALTER TABLE build_pipelines ADD COLUMN description VARCHAR(1024) NOT NULL DEFAULT '''' AFTER display_name', 'SELECT 1');
PREPARE build_pipelines_description_stmt FROM @build_pipelines_description_ddl;
EXECUTE build_pipelines_description_stmt;
DEALLOCATE PREPARE build_pipelines_description_stmt;
`

var Migrations = []database.Migration{
	{
		Version: 202605300601,
		Name:    "build_core",
		Up: `
CREATE TABLE build_pipelines (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL DEFAULT '',
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(1024) NOT NULL DEFAULT '',
  provider VARCHAR(32) NOT NULL,
  external_job_name VARCHAR(255) NOT NULL,
  template_id VARCHAR(128) NOT NULL,
  config_hash VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  managed_by_platform TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_build_pipelines_application_status (application_id, status),
  KEY idx_build_pipelines_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_runs (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  pipeline_id VARCHAR(64) NOT NULL,
  pipeline_name VARCHAR(64) NOT NULL DEFAULT '',
  pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '',
  application_id VARCHAR(64) NOT NULL,
  source_repository_id VARCHAR(64) NOT NULL,
  git_ref VARCHAR(128) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  jenkins_queue_id VARCHAR(128) NOT NULL DEFAULT '',
  jenkins_build_number BIGINT NOT NULL DEFAULT 0,
  primary_artifact_id VARCHAR(64) NOT NULL DEFAULT '',
  log_offset BIGINT NOT NULL DEFAULT 0,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  requested_by VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  KEY idx_build_runs_application_created (application_id, created_at),
  KEY idx_build_runs_pipeline (pipeline_id),
  CONSTRAINT fk_build_runs_pipeline FOREIGN KEY (pipeline_id) REFERENCES build_pipelines(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_run_sources (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  build_run_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL,
  source_repository_id VARCHAR(64) NOT NULL,
  git_ref VARCHAR(128) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  source_path VARCHAR(512) NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_build_run_sources_key (build_run_id, source_key),
  KEY idx_build_run_sources_run (build_run_id),
  CONSTRAINT fk_build_run_sources_run FOREIGN KEY (build_run_id) REFERENCES build_runs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_pipeline_runtime_environments (
  pipeline_id VARCHAR(64) NOT NULL,
  runtime_environment_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  runtime_base_image VARCHAR(512) NOT NULL DEFAULT '',
  artifact_deploy_path VARCHAR(512) NOT NULL DEFAULT '',
  dockerfile_path VARCHAR(512) NOT NULL DEFAULT '',
  position INT NOT NULL DEFAULT 0,
  PRIMARY KEY (pipeline_id, runtime_environment_id),
  KEY idx_build_pipeline_runtime_environments_runtime (runtime_environment_id),
  CONSTRAINT fk_build_pipeline_runtime_environments_pipeline FOREIGN KEY (pipeline_id) REFERENCES build_pipelines(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_pipeline_sources (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  pipeline_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  source_repository_id VARCHAR(64) NOT NULL,
  build_environment_id VARCHAR(64) NOT NULL DEFAULT '',
  source_path VARCHAR(512) NOT NULL,
  build_spec JSON NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_build_pipeline_sources_key (pipeline_id, source_key),
  KEY idx_build_pipeline_sources_pipeline (pipeline_id),
  CONSTRAINT fk_build_pipeline_sources_pipeline FOREIGN KEY (pipeline_id) REFERENCES build_pipelines(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_artifacts (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  build_run_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL,
  uri VARCHAR(1024) NOT NULL,
  digest VARCHAR(255) NOT NULL DEFAULT '',
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  metadata JSON NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_build_artifacts_run (build_run_id),
  KEY idx_build_artifacts_application (application_id),
  CONSTRAINT fk_build_artifacts_run FOREIGN KEY (build_run_id) REFERENCES build_runs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS build_artifacts;
DROP TABLE IF EXISTS build_pipeline_runtime_environments;
DROP TABLE IF EXISTS build_pipeline_sources;
DROP TABLE IF EXISTS build_run_sources;
DROP TABLE IF EXISTS build_runs;
DROP TABLE IF EXISTS build_pipelines;
`,
	},
	{
		Version: 202606081430,
		Name:    "build_logs_append_only",
		Up: `
CREATE TABLE build_logs (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  build_run_id VARCHAR(64) NOT NULL,
  log_text LONGTEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_build_logs_run_id (build_run_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `DROP TABLE IF EXISTS build_logs;`,
	},
	{
		Version: 202606081431,
		Name:    "ensure_build_logs_longtext",
		Up:      `ALTER TABLE build_logs MODIFY COLUMN log_text LONGTEXT NOT NULL;`,
		Down:    `ALTER TABLE build_logs MODIFY COLUMN log_text TEXT NOT NULL;`,
	},
	{
		Version: 202606081432,
		Name:    "drop_build_logs_run_foreign_key",
		Up: `
SET @build_logs_fk_exists := (
  SELECT COUNT(*)
  FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'build_logs'
    AND CONSTRAINT_NAME = 'fk_build_logs_run'
    AND CONSTRAINT_TYPE = 'FOREIGN KEY'
);
SET @build_logs_fk_ddl := IF(@build_logs_fk_exists > 0, 'ALTER TABLE build_logs DROP FOREIGN KEY fk_build_logs_run', 'SELECT 1');
PREPARE build_logs_fk_stmt FROM @build_logs_fk_ddl;
EXECUTE build_logs_fk_stmt;
DEALLOCATE PREPARE build_logs_fk_stmt;
`,
		Down: `SELECT 1;`,
	},
	{
		Version: 202606090401,
		Name:    "build_configuration_tables",
		Up: `
CREATE TABLE build_environments (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(1024) NOT NULL DEFAULT '',
  build_image VARCHAR(512) NOT NULL,
  status VARCHAR(32) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_build_environments_name (name),
  KEY idx_build_environments_status_default (status, is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE runtime_environments (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(1024) NOT NULL DEFAULT '',
  runtime_base_image VARCHAR(512) NOT NULL,
  artifact_deploy_path VARCHAR(512) NOT NULL DEFAULT '',
  dockerfile_path VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_runtime_environments_name (name),
  KEY idx_runtime_environments_status_default (status, is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE build_templates (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  version INT NOT NULL,
  content LONGTEXT NOT NULL,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE jenkins_job_templates (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(1024) NOT NULL DEFAULT '',
  runtime_base_image VARCHAR(512) NOT NULL DEFAULT '',
  version INT NOT NULL,
  xml_content LONGTEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_jenkins_job_templates_name (name),
  KEY idx_jenkins_job_templates_status_default (status, is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS jenkins_job_templates;
DROP TABLE IF EXISTS build_templates;
DROP TABLE IF EXISTS runtime_environments;
DROP TABLE IF EXISTS build_environments;
`,
	},
	{
		Version: 202606090402,
		Name:    "backfill_build_core_pipeline_columns",
		Up: `
CREATE TABLE IF NOT EXISTS build_pipeline_sources (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  pipeline_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  source_repository_id VARCHAR(64) NOT NULL,
  build_environment_id VARCHAR(64) NOT NULL DEFAULT '',
  source_path VARCHAR(512) NOT NULL,
  build_spec JSON NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_build_pipeline_sources_key (pipeline_id, source_key),
  KEY idx_build_pipeline_sources_pipeline (pipeline_id),
  CONSTRAINT fk_build_pipeline_sources_pipeline FOREIGN KEY (pipeline_id) REFERENCES build_pipelines(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
` + backfillBuildPipelineIdentityColumnsSQL + `
SET @build_pipelines_template_id_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'template_id'
);
SET @build_pipelines_template_id_ddl := IF(@build_pipelines_template_id_missing, 'ALTER TABLE build_pipelines ADD COLUMN template_id VARCHAR(128) NOT NULL DEFAULT '''' AFTER external_job_name', 'SELECT 1');
PREPARE build_pipelines_template_id_stmt FROM @build_pipelines_template_id_ddl;
EXECUTE build_pipelines_template_id_stmt;
DEALLOCATE PREPARE build_pipelines_template_id_stmt;

SET @build_pipelines_config_hash_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'config_hash'
);
SET @build_pipelines_config_hash_ddl := IF(@build_pipelines_config_hash_missing, 'ALTER TABLE build_pipelines ADD COLUMN config_hash VARCHAR(64) NOT NULL DEFAULT '''' AFTER template_id', 'SELECT 1');
PREPARE build_pipelines_config_hash_stmt FROM @build_pipelines_config_hash_ddl;
EXECUTE build_pipelines_config_hash_stmt;
DEALLOCATE PREPARE build_pipelines_config_hash_stmt;

SET @build_pipelines_managed_by_platform_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipelines'
    AND column_name = 'managed_by_platform'
);
SET @build_pipelines_managed_by_platform_ddl := IF(@build_pipelines_managed_by_platform_missing, 'ALTER TABLE build_pipelines ADD COLUMN managed_by_platform TINYINT(1) NOT NULL DEFAULT 1 AFTER status', 'SELECT 1');
PREPARE build_pipelines_managed_by_platform_stmt FROM @build_pipelines_managed_by_platform_ddl;
EXECUTE build_pipelines_managed_by_platform_stmt;
DEALLOCATE PREPARE build_pipelines_managed_by_platform_stmt;

SET @build_runs_pipeline_name_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_runs'
    AND column_name = 'pipeline_name'
);
SET @build_runs_pipeline_name_ddl := IF(@build_runs_pipeline_name_missing, 'ALTER TABLE build_runs ADD COLUMN pipeline_name VARCHAR(64) NOT NULL DEFAULT '''' AFTER pipeline_id', 'SELECT 1');
PREPARE build_runs_pipeline_name_stmt FROM @build_runs_pipeline_name_ddl;
EXECUTE build_runs_pipeline_name_stmt;
DEALLOCATE PREPARE build_runs_pipeline_name_stmt;

SET @build_runs_pipeline_display_name_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_runs'
    AND column_name = 'pipeline_display_name'
);
SET @build_runs_pipeline_display_name_ddl := IF(@build_runs_pipeline_display_name_missing, 'ALTER TABLE build_runs ADD COLUMN pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '''' AFTER pipeline_name', 'SELECT 1');
PREPARE build_runs_pipeline_display_name_stmt FROM @build_runs_pipeline_display_name_ddl;
EXECUTE build_runs_pipeline_display_name_stmt;
DEALLOCATE PREPARE build_runs_pipeline_display_name_stmt;

SET @build_pipeline_sources_build_environment_id_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'build_pipeline_sources'
    AND column_name = 'build_environment_id'
);
SET @build_pipeline_sources_build_environment_id_ddl := IF(@build_pipeline_sources_build_environment_id_missing, 'ALTER TABLE build_pipeline_sources ADD COLUMN build_environment_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER source_repository_id', 'SELECT 1');
PREPARE build_pipeline_sources_build_environment_id_stmt FROM @build_pipeline_sources_build_environment_id_ddl;
EXECUTE build_pipeline_sources_build_environment_id_stmt;
DEALLOCATE PREPARE build_pipeline_sources_build_environment_id_stmt;
`,
		Down: `SELECT 1;`,
	},
	{
		Version: 202606100101,
		Name:    "backfill_build_pipeline_identity_columns",
		Up:      backfillBuildPipelineIdentityColumnsSQL,
		Down:    `SELECT 1;`,
	},
	{
		Version: 202606100102,
		Name:    "build_pipeline_runtime_environment_tables",
		Up: `
CREATE TABLE IF NOT EXISTS build_pipeline_runtime_environments (
  pipeline_id VARCHAR(64) NOT NULL,
  runtime_environment_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  runtime_base_image VARCHAR(512) NOT NULL DEFAULT '',
  artifact_deploy_path VARCHAR(512) NOT NULL DEFAULT '',
  dockerfile_path VARCHAR(512) NOT NULL DEFAULT '',
  position INT NOT NULL DEFAULT 0,
  PRIMARY KEY (pipeline_id, runtime_environment_id),
  KEY idx_build_pipeline_runtime_environments_runtime (runtime_environment_id),
  CONSTRAINT fk_build_pipeline_runtime_environments_pipeline FOREIGN KEY (pipeline_id) REFERENCES build_pipelines(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET @application_runtime_environments_exists := (
  SELECT COUNT(*) > 0
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 'application_runtime_environments'
);
SET @build_pipeline_runtime_backfill_sql := IF(@application_runtime_environments_exists,
  'INSERT IGNORE INTO build_pipeline_runtime_environments (pipeline_id, runtime_environment_id, name, runtime_base_image, artifact_deploy_path, dockerfile_path, position)
   SELECT p.id, are.runtime_environment_id, are.name, are.runtime_base_image, are.artifact_deploy_path, are.dockerfile_path, are.position
   FROM build_pipelines p
   JOIN application_runtime_environments are ON are.application_id = p.application_id',
  'SELECT 1');
PREPARE build_pipeline_runtime_backfill_stmt FROM @build_pipeline_runtime_backfill_sql;
EXECUTE build_pipeline_runtime_backfill_stmt;
DEALLOCATE PREPARE build_pipeline_runtime_backfill_stmt;
`,
		Down: `
DROP TABLE IF EXISTS build_pipeline_runtime_environments;
`,
	},
	{
		Version: 202606100103,
		Name:    "backfill_build_run_sources_table",
		Up: `
CREATE TABLE IF NOT EXISTS build_run_sources (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  build_run_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL,
  source_repository_id VARCHAR(64) NOT NULL,
  git_ref VARCHAR(128) NOT NULL,
  commit_sha VARCHAR(128) NOT NULL DEFAULT '',
  source_path VARCHAR(512) NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_build_run_sources_key (build_run_id, source_key),
  KEY idx_build_run_sources_run (build_run_id),
  CONSTRAINT fk_build_run_sources_run FOREIGN KEY (build_run_id) REFERENCES build_runs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `SELECT 1;`,
	},
}
