package build

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/testsupport"
)

func TestMigrationsBackfillBuildCorePipelineColumns(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	replacements := []string{
		"  template_id VARCHAR(128) NOT NULL,\n",
		"  config_hash VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  managed_by_platform TINYINT(1) NOT NULL DEFAULT 1,\n",
		"  pipeline_name VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  pipeline_display_name VARCHAR(128) NOT NULL DEFAULT '',\n",
		"  build_environment_id VARCHAR(64) NOT NULL DEFAULT '',\n",
	}
	for _, replacement := range replacements {
		oldCore.Up = strings.Replace(oldCore.Up, replacement, "", 1)
	}
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove build core columns from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	if err := migrator.Up(ctx, Migrations[1:]); err != nil {
		t.Fatalf("apply current follow-up migrations: %v", err)
	}

	for table, columns := range map[string][]string{
		"build_pipelines":        {"template_id", "config_hash", "managed_by_platform"},
		"build_runs":             {"pipeline_name", "pipeline_display_name"},
		"build_pipeline_sources": {"build_environment_id"},
	} {
		for _, column := range columns {
			assertMySQLColumnExists(t, ctx, db, table, column)
		}
	}

	repo, err := NewMySQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	now := time.Now().UTC()
	if err := repo.CreatePipeline(ctx, BuildPipeline{
		ID:                "pipeline_1",
		TenantID:          "tenant_1",
		ProjectID:         "project_1",
		ApplicationID:     "app_1",
		Name:              "main",
		DisplayName:       "主流水线",
		Provider:          "jenkins",
		ExternalJobName:   "paas/project/app/main",
		TemplateID:        "template_1",
		ConfigHash:        "hash_1",
		Status:            BuildPipelineStatusActive,
		ManagedByPlatform: true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	result, err := repo.ListPipelinesByApplication(ctx, "app_1", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPipelinesByApplication() error = %v", err)
	}
	if got := len(result.Items); got != 1 {
		t.Fatalf("ListPipelinesByApplication() returned %d items, want 1", got)
	}
}

func TestMigrationsBackfillBuildCorePipelineColumnsCreatesMissingPipelineSourcesTable(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	oldCore.Up = strings.Replace(oldCore.Up, `
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
`, "", 1)
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove build_pipeline_sources from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	backfill := findBuildMigration(t, "backfill_build_core_pipeline_columns")
	if err := migrator.Up(ctx, []database.Migration{backfill}); err != nil {
		t.Fatalf("apply backfill migration: %v", err)
	}
	assertMySQLColumnExists(t, ctx, db, "build_pipeline_sources", "build_environment_id")
}

type columnQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func assertMySQLColumnExists(t *testing.T, ctx context.Context, db columnQueryer, table string, column string) {
	t.Helper()
	var count int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	if err != nil {
		t.Fatalf("query column %s.%s: %v", table, column, err)
	}
	if count != 1 {
		t.Fatalf("column %s.%s count = %d, want 1", table, column, count)
	}
}

func findBuildMigration(t *testing.T, name string) database.Migration {
	t.Helper()
	for _, migration := range Migrations {
		if migration.Name == name {
			return migration
		}
	}
	t.Fatalf("migration %s not found", name)
	return database.Migration{}
}
