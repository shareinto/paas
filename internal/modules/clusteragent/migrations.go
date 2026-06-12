package clusteragent

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605301201,
		Name:    "cluster_agent_core",
		Up: `
CREATE TABLE clusters (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  region VARCHAR(128) NOT NULL DEFAULT '',
  labels_json JSON NULL,
  server_version VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  agent_token_hash VARCHAR(255) NOT NULL,
  last_heartbeat_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_clusters_tenant_status_created (tenant_id, status, created_at),
  KEY idx_clusters_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE cluster_heartbeats (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  cluster_id VARCHAR(64) NOT NULL,
  agent_version VARCHAR(64) NOT NULL DEFAULT '',
  observed_at DATETIME(6) NOT NULL,
  message VARCHAR(512) NOT NULL DEFAULT '',
  control_plane_url VARCHAR(1024) NOT NULL DEFAULT '',
  KEY idx_cluster_heartbeats_cluster_time (cluster_id, observed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE cluster_resource_snapshots (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  cluster_id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  payload JSON NOT NULL,
  reported_at DATETIME(6) NOT NULL,
  KEY idx_cluster_snapshots_cluster_time (cluster_id, reported_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE cluster_tasks (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  cluster_id VARCHAR(64) NOT NULL,
  type VARCHAR(64) NOT NULL,
  target_ref VARCHAR(255) NOT NULL DEFAULT '',
  payload JSON NULL,
  status VARCHAR(32) NOT NULL,
  result_message VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  leased_at DATETIME(6) NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_cluster_tasks_pending (cluster_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE cluster_task_results (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  cluster_id VARCHAR(64) NOT NULL,
  task_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  message VARCHAR(1024) NOT NULL DEFAULT '',
  reported_at DATETIME(6) NOT NULL,
  KEY idx_cluster_task_results_task (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS cluster_task_results;
DROP TABLE IF EXISTS cluster_tasks;
DROP TABLE IF EXISTS cluster_resource_snapshots;
DROP TABLE IF EXISTS cluster_heartbeats;
DROP TABLE IF EXISTS clusters;
`,
	},
	{
		Version: 202606121402,
		Name:    "cluster_selector_labels",
		Up: `
SET @clusters_labels_missing := (
  SELECT COUNT(*) = 0 FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'clusters' AND column_name = 'labels_json'
);
SET @clusters_labels_ddl := IF(@clusters_labels_missing, 'ALTER TABLE clusters ADD COLUMN labels_json JSON NULL AFTER region', 'SELECT 1');
PREPARE clusters_labels_stmt FROM @clusters_labels_ddl;
EXECUTE clusters_labels_stmt;
DEALLOCATE PREPARE clusters_labels_stmt;
`,
		Down: `SELECT 1;`,
	},
	{
		Version: 202606111201,
		Name:    "cluster_tenant_status_index",
		Up: `
SET @clusters_tenant_index_missing := (
  SELECT COUNT(*) = 0
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'clusters'
    AND index_name = 'idx_clusters_tenant_status_created'
);
SET @clusters_tenant_index_ddl := IF(@clusters_tenant_index_missing, 'ALTER TABLE clusters ADD KEY idx_clusters_tenant_status_created (tenant_id, status, created_at)', 'SELECT 1');
PREPARE clusters_tenant_index_stmt FROM @clusters_tenant_index_ddl;
EXECUTE clusters_tenant_index_stmt;
DEALLOCATE PREPARE clusters_tenant_index_stmt;
`,
		Down: `SELECT 1;`,
	},
}
