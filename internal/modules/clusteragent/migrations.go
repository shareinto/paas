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
  server_version VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  agent_token_hash VARCHAR(255) NOT NULL,
  last_heartbeat_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
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
}
