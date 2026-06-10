package appenv

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605300501,
		Name:    "application_environment_core",
		Up: `
CREATE TABLE applications (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_applications_project_name (project_id, name),
  KEY idx_applications_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE application_sources (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  source_key VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  source_repository_id VARCHAR(64) NOT NULL,
  jenkins_template_id VARCHAR(64) NOT NULL DEFAULT '',
  source_path VARCHAR(512) NOT NULL,
  build_command VARCHAR(1024) NOT NULL,
  runtime_base_image VARCHAR(512) NOT NULL,
  artifact_deploy_path VARCHAR(512) NOT NULL DEFAULT '',
  default_ref VARCHAR(128) NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_application_sources_application_key (application_id, source_key),
  KEY idx_application_sources_repo (source_repository_id),
  CONSTRAINT fk_application_sources_application FOREIGN KEY (application_id) REFERENCES applications(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environments (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_environments_application_name (application_id, name),
  KEY idx_environments_project_application (project_id, application_id),
  CONSTRAINT fk_environments_application FOREIGN KEY (application_id) REFERENCES applications(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_configs (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  config_key VARCHAR(128) NOT NULL,
  config_value TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_environment_configs_key (environment_id, config_key),
  CONSTRAINT fk_environment_configs_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_secrets (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  secret_key VARCHAR(128) NOT NULL,
  secret_ref VARCHAR(512) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_environment_secrets_key (environment_id, secret_key),
  CONSTRAINT fk_environment_secrets_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_routes (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  host VARCHAR(255) NOT NULL,
  path VARCHAR(255) NOT NULL DEFAULT '/',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_environment_routes_environment (environment_id),
  CONSTRAINT fk_environment_routes_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_cluster_bindings (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  cluster_id VARCHAR(64) NOT NULL,
  cluster_name VARCHAR(128) NOT NULL,
  namespace VARCHAR(128) NOT NULL,
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_environment_cluster_bindings_environment (environment_id),
  KEY idx_environment_cluster_bindings_cluster (cluster_id),
  CONSTRAINT fk_environment_cluster_bindings_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_states (
  environment_id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  status VARCHAR(64) NOT NULL,
  message VARCHAR(1024) NOT NULL DEFAULT '',
  last_reported_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_environment_states_application (application_id),
  CONSTRAINT fk_environment_states_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE environment_events (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  environment_id VARCHAR(64) NOT NULL,
  type VARCHAR(128) NOT NULL,
  status VARCHAR(64) NOT NULL,
  message VARCHAR(1024) NOT NULL DEFAULT '',
  occurred_at DATETIME(6) NOT NULL,
  KEY idx_environment_events_environment_time (environment_id, occurred_at),
  CONSTRAINT fk_environment_events_environment FOREIGN KEY (environment_id) REFERENCES environments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS environment_events;
DROP TABLE IF EXISTS environment_states;
DROP TABLE IF EXISTS environment_cluster_bindings;
DROP TABLE IF EXISTS environment_routes;
DROP TABLE IF EXISTS environment_secrets;
DROP TABLE IF EXISTS environment_configs;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS application_sources;
DROP TABLE IF EXISTS applications;
`,
	},
	{
		Version: 202606020101,
		Name:    "application_source_artifact_copy_command",
		Up: `
ALTER TABLE application_sources
  ADD COLUMN artifact_copy_command TEXT NULL AFTER build_command;
`,
		Down: `
ALTER TABLE application_sources
  DROP COLUMN artifact_copy_command;
`,
	},
	{
		Version: 202606020102,
		Name:    "application_source_runtime_environment",
		Up: `
	SELECT 1;
	`,
		Down: `
	SELECT 1;
	`,
	},
	{
		Version: 202606080101,
		Name:    "remove_start_command_and_target_path",
		Up: `
	SELECT IF(COUNT(*) > 0, 'ALTER TABLE application_sources DROP COLUMN start_command', 'SELECT 1') INTO @drop_start_command
	  FROM information_schema.columns
	  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'start_command';
	PREPARE drop_start_command_stmt FROM @drop_start_command;
	EXECUTE drop_start_command_stmt;
	DEALLOCATE PREPARE drop_start_command_stmt;
	SELECT IF(COUNT(*) > 0, 'ALTER TABLE application_sources DROP COLUMN target_path', 'SELECT 1') INTO @drop_target_path
	  FROM information_schema.columns
	  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'target_path';
	PREPARE drop_target_path_stmt FROM @drop_target_path;
	EXECUTE drop_target_path_stmt;
	DEALLOCATE PREPARE drop_target_path_stmt;
	`,
		Down: `
	SELECT IF(COUNT(*) = 0, 'ALTER TABLE application_sources ADD COLUMN start_command VARCHAR(1024) NOT NULL DEFAULT '''' AFTER runtime_base_image', 'SELECT 1') INTO @add_start_command
	  FROM information_schema.columns
	  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'start_command';
	PREPARE add_start_command_stmt FROM @add_start_command;
	EXECUTE add_start_command_stmt;
	DEALLOCATE PREPARE add_start_command_stmt;
	SELECT IF(COUNT(*) = 0, 'ALTER TABLE application_sources ADD COLUMN target_path VARCHAR(255) NOT NULL DEFAULT '''' AFTER default_ref', 'SELECT 1') INTO @add_target_path
	  FROM information_schema.columns
	  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'target_path';
	PREPARE add_target_path_stmt FROM @add_target_path;
	EXECUTE add_target_path_stmt;
			DEALLOCATE PREPARE add_target_path_stmt;
	`,
	},
	{
		Version: 202606080102,
		Name:    "remove_application_source_artifact_path",
		Up: `
SELECT IF(COUNT(*) > 0, 'ALTER TABLE application_sources DROP COLUMN artifact_path', 'SELECT 1') INTO @drop_artifact_path
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'artifact_path';
PREPARE drop_artifact_path_stmt FROM @drop_artifact_path;
EXECUTE drop_artifact_path_stmt;
DEALLOCATE PREPARE drop_artifact_path_stmt;
`,
		Down: `
SELECT IF(COUNT(*) = 0, 'ALTER TABLE application_sources ADD COLUMN artifact_path VARCHAR(512) NOT NULL DEFAULT '''' AFTER build_command', 'SELECT 1') INTO @add_artifact_path
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'artifact_path';
PREPARE add_artifact_path_stmt FROM @add_artifact_path;
EXECUTE add_artifact_path_stmt;
DEALLOCATE PREPARE add_artifact_path_stmt;
`,
	},
	{
		Version: 202606090300,
		Name:    "backfill_application_source_jenkins_template_id",
		Up: `
SELECT IF(COUNT(*) = 0, 'ALTER TABLE application_sources ADD COLUMN jenkins_template_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER source_repository_id', 'SELECT 1') INTO @add_source_jenkins_template_id
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'jenkins_template_id';
PREPARE add_source_jenkins_template_id_stmt FROM @add_source_jenkins_template_id;
EXECUTE add_source_jenkins_template_id_stmt;
DEALLOCATE PREPARE add_source_jenkins_template_id_stmt;
`,
		Down: `
SELECT 1;
`,
	},
	{
		Version: 202606090301,
		Name:    "application_runtime_environment_tables",
		Up: `
SELECT IF(COUNT(*) = 0, 'ALTER TABLE applications ADD COLUMN runtime_environment_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER description', 'SELECT 1') INTO @add_app_runtime_environment_id
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'applications' AND column_name = 'runtime_environment_id';
PREPARE add_app_runtime_environment_id_stmt FROM @add_app_runtime_environment_id;
EXECUTE add_app_runtime_environment_id_stmt;
DEALLOCATE PREPARE add_app_runtime_environment_id_stmt;

SELECT IF(COUNT(*) = 0, 'ALTER TABLE application_sources ADD COLUMN build_environment_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER jenkins_template_id', 'SELECT 1') INTO @add_source_build_environment_id
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = 'build_environment_id';
PREPARE add_source_build_environment_id_stmt FROM @add_source_build_environment_id;
EXECUTE add_source_build_environment_id_stmt;
DEALLOCATE PREPARE add_source_build_environment_id_stmt;

CREATE TABLE application_runtime_environments (
  application_id VARCHAR(64) NOT NULL,
  runtime_environment_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  runtime_base_image VARCHAR(512) NOT NULL DEFAULT '',
  artifact_deploy_path VARCHAR(512) NOT NULL DEFAULT '',
  dockerfile_path VARCHAR(512) NOT NULL DEFAULT '',
  position INT NOT NULL DEFAULT 0,
  PRIMARY KEY (application_id, runtime_environment_id),
  KEY idx_application_runtime_environments_runtime (runtime_environment_id),
  CONSTRAINT fk_application_runtime_environments_application FOREIGN KEY (application_id) REFERENCES applications(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS application_runtime_environments;
ALTER TABLE application_sources DROP COLUMN build_environment_id;
ALTER TABLE applications DROP COLUMN runtime_environment_id;
`,
	},
}
