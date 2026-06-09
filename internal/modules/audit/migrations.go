package audit

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605301001,
		Name:    "audit_logs",
		Up: `
CREATE TABLE audit_logs (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  actor_id VARCHAR(64) NOT NULL DEFAULT '',
  actor_type VARCHAR(32) NOT NULL,
  subject_type VARCHAR(32) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  project_id VARCHAR(64) NOT NULL DEFAULT '',
  resource_type VARCHAR(64) NOT NULL,
  resource_id VARCHAR(64) NOT NULL DEFAULT '',
  action VARCHAR(128) NOT NULL,
  result VARCHAR(32) NOT NULL,
  summary VARCHAR(512) NOT NULL DEFAULT '',
  details JSON NULL,
  occurred_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_audit_tenant_time (tenant_id, occurred_at),
  KEY idx_audit_project_time (project_id, occurred_at),
  KEY idx_audit_actor_time (actor_id, occurred_at),
  KEY idx_audit_resource (resource_type, resource_id),
  KEY idx_audit_action_time (action, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `DROP TABLE IF EXISTS audit_logs;`,
	},
}
