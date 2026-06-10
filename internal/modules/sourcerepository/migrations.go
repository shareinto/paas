package sourcerepository

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605300401,
		Name:    "source_repository_core",
		Up: `
CREATE TABLE source_repositories (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(512) NOT NULL DEFAULT '',
  git_provider VARCHAR(32) NOT NULL,
  git_project_id VARCHAR(128) NOT NULL DEFAULT '',
  http_url VARCHAR(512) NOT NULL DEFAULT '',
  ssh_url VARCHAR(512) NOT NULL DEFAULT '',
  default_branch VARCHAR(128) NOT NULL DEFAULT 'main',
  status VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_source_repositories_project_name (project_id, name),
  KEY idx_source_repositories_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE repository_migrations (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  source_repository_id VARCHAR(64) NOT NULL,
  source_url VARCHAR(1024) NOT NULL,
  status VARCHAR(64) NOT NULL,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  requested_by VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_repository_migrations_repo (source_repository_id),
  KEY idx_repository_migrations_project_status (project_id, status),
  CONSTRAINT fk_repository_migrations_source_repo FOREIGN KEY (source_repository_id) REFERENCES source_repositories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE repository_permission_sync_jobs (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  source_repository_id VARCHAR(64) NOT NULL,
  status VARCHAR(64) NOT NULL,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  requested_by VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_repository_permission_sync_jobs_repo (source_repository_id),
  KEY idx_repository_permission_sync_jobs_project_status (project_id, status),
  CONSTRAINT fk_repository_permission_sync_jobs_source_repo FOREIGN KEY (source_repository_id) REFERENCES source_repositories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS repository_permission_sync_jobs;
DROP TABLE IF EXISTS repository_migrations;
DROP TABLE IF EXISTS source_repositories;
`,
	},
	{
		Version: 202606090101,
		Name:    "source_repository_associations",
		Up: `
CREATE TABLE source_repository_associations (
  source_repository_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(64) NOT NULL,
  application_name VARCHAR(64) NOT NULL,
  application_display_name VARCHAR(128) NOT NULL DEFAULT '',
  PRIMARY KEY (source_repository_id, application_id),
  KEY idx_source_repository_associations_name (source_repository_id, application_name),
  CONSTRAINT fk_source_repository_associations_source_repo FOREIGN KEY (source_repository_id) REFERENCES source_repositories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `DROP TABLE IF EXISTS source_repository_associations;`,
	},
}
