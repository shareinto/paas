package notification

import "github.com/shareinto/paas/internal/platform/database"

var Migrations = []database.Migration{
	{
		Version: 202605301101,
		Name:    "notification_core",
		Up: `
CREATE TABLE notification_templates (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  event_type VARCHAR(128) NOT NULL,
  title_template VARCHAR(255) NOT NULL,
  content_template TEXT NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_notification_templates_event (event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_channels (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  type VARCHAR(32) NOT NULL,
  target VARCHAR(1024) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notifications (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  project_id VARCHAR(64) NOT NULL DEFAULT '',
  event_type VARCHAR(128) NOT NULL,
  dedupe_key VARCHAR(128) NOT NULL,
  channel_id VARCHAR(64) NOT NULL,
  title VARCHAR(255) NOT NULL,
  content TEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  sent_at DATETIME(6) NULL,
  UNIQUE KEY uk_notifications_dedupe (dedupe_key),
  KEY idx_notifications_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS notification_templates;
`,
	},
}
