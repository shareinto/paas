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
		"  name VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  display_name VARCHAR(128) NOT NULL DEFAULT '',\n",
		"  description VARCHAR(1024) NOT NULL DEFAULT '',\n",
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
		"build_pipelines":        {"name", "display_name", "description", "template_id", "config_hash", "managed_by_platform"},
		"build_runs":             {"pipeline_name", "pipeline_display_name", "workload_id"},
		"build_artifacts":        {"workload_id"},
		"build_pipeline_sources": {"build_environment_id"},
	} {
		for _, column := range columns {
			assertMySQLColumnExists(t, ctx, db, table, column)
		}
	}
	assertMySQLColumnNotExists(t, ctx, db, "build_pipelines", "workload_id")

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

func TestMigrationsBackfillBuildRunSourcesTableWhenCoreWasAppliedWithoutIt(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	oldCore.Up = strings.Replace(oldCore.Up, `
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
`, "", 1)
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove build_run_sources from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	backfill := findBuildMigration(t, "backfill_build_run_sources_table")
	if err := migrator.Up(ctx, []database.Migration{backfill}); err != nil {
		t.Fatalf("apply build_run_sources backfill migration: %v", err)
	}
	assertMySQLColumnExists(t, ctx, db, "build_run_sources", "source_key")

	repo, err := NewMySQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	now := time.Now().UTC()
	pipeline := BuildPipeline{
		ID:                "pipeline_1",
		TenantID:          "tenant_1",
		ProjectID:         "project_1",
		ApplicationID:     "app_1",
		Name:              "main",
		DisplayName:       "主流水线",
		Provider:          "jenkins",
		ExternalJobName:   "paas/project/app/main",
		TemplateID:        "template_1",
		Status:            BuildPipelineStatusActive,
		ManagedByPlatform: true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.CreatePipeline(ctx, pipeline); err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	run := BuildRun{
		ID:                  "build_run_1",
		TenantID:            pipeline.TenantID,
		ProjectID:           pipeline.ProjectID,
		PipelineID:          pipeline.ID,
		PipelineName:        pipeline.Name,
		PipelineDisplayName: pipeline.DisplayName,
		ApplicationID:       pipeline.ApplicationID,
		SourceRepositoryID:  "repo_1",
		GitRef:              "main",
		Status:              BuildRunQueued,
		RequestedBy:         "usr_builder",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := repo.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := repo.CreateRunSource(ctx, BuildRunSource{
		ID:                 "build_run_source_1",
		TenantID:           pipeline.TenantID,
		ProjectID:          pipeline.ProjectID,
		BuildRunID:         run.ID,
		ApplicationID:      pipeline.ApplicationID,
		SourceKey:          "main",
		SourceRepositoryID: "repo_1",
		GitRef:             "main",
		SourcePath:         ".",
		IsPrimary:          true,
		CreatedAt:          now,
	}); err != nil {
		t.Fatalf("CreateRunSource() error = %v", err)
	}
	sources, err := repo.ListRunSources(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunSources() error = %v", err)
	}
	if len(sources) != 1 || sources[0].SourceKey != "main" {
		t.Fatalf("unexpected run sources: %+v", sources)
	}
}

func TestMigrationsBackfillBuildArtifactSourceKeyForExistingTables(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	oldCore.Up = strings.Replace(oldCore.Up, "  source_key VARCHAR(64) NOT NULL DEFAULT '',\n", "", 1)
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove build_artifacts.source_key from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	if err := migrator.Up(ctx, Migrations[1:]); err != nil {
		t.Fatalf("apply current follow-up migrations: %v", err)
	}
	assertMySQLColumnExists(t, ctx, db, "build_artifacts", "source_key")

	repo, err := NewMySQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	now := time.Now().UTC()
	pipeline := BuildPipeline{
		ID:                "pipeline_1",
		TenantID:          "tenant_1",
		ProjectID:         "project_1",
		ApplicationID:     "app_1",
		Name:              "main",
		DisplayName:       "主流水线",
		Provider:          "jenkins",
		ExternalJobName:   "paas/project/app/main",
		TemplateID:        "template_1",
		Status:            BuildPipelineStatusActive,
		ManagedByPlatform: true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.CreatePipeline(ctx, pipeline); err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	run := BuildRun{
		ID:                  "build_run_1",
		TenantID:            pipeline.TenantID,
		ProjectID:           pipeline.ProjectID,
		PipelineID:          pipeline.ID,
		PipelineName:        pipeline.Name,
		PipelineDisplayName: pipeline.DisplayName,
		ApplicationID:       pipeline.ApplicationID,
		SourceRepositoryID:  "repo_1",
		GitRef:              "main",
		Status:              BuildRunQueued,
		RequestedBy:         "usr_builder",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := repo.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := repo.CreateArtifact(ctx, BuildArtifact{
		ID:            "artifact_1",
		TenantID:      pipeline.TenantID,
		ProjectID:     pipeline.ProjectID,
		BuildRunID:    run.ID,
		ApplicationID: pipeline.ApplicationID,
		SourceKey:     "main",
		Type:          BuildArtifactImage,
		Name:          "主镜像",
		URI:           "registry.example/log-receiver:main",
		IsPrimary:     true,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	artifacts, err := repo.ListArtifactsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListArtifactsByRun() error = %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].SourceKey != "main" {
		t.Fatalf("unexpected artifacts: %+v", artifacts)
	}
}

func TestMigrationsBackfillBuildPipelineIdentityColumnsAfterCoreBackfillWasApplied(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	for _, replacement := range []string{
		"  name VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  display_name VARCHAR(128) NOT NULL DEFAULT '',\n",
		"  description VARCHAR(1024) NOT NULL DEFAULT '',\n",
	} {
		oldCore.Up = strings.Replace(oldCore.Up, replacement, "", 1)
	}
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove build pipeline identity columns from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)", int64(202606090402), "backfill_build_core_pipeline_columns", time.Now().UTC()); err != nil {
		t.Fatalf("mark old backfill migration applied: %v", err)
	}

	if err := migrator.Up(ctx, Migrations[1:]); err != nil {
		t.Fatalf("apply current follow-up migrations: %v", err)
	}

	for _, column := range []string{"name", "display_name", "description"} {
		assertMySQLColumnExists(t, ctx, db, "build_pipelines", column)
	}
	repo, err := NewMySQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	result, err := repo.ListPipelinesByApplication(ctx, "app_1", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPipelinesByApplication() error = %v", err)
	}
	if got := len(result.Items); got != 0 {
		t.Fatalf("ListPipelinesByApplication() returned %d items, want 0", got)
	}
}

func TestMigrationsDropBuildPipelineWorkloadID(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	if err := migrator.Up(ctx, []database.Migration{Migrations[0]}); err != nil {
		t.Fatalf("apply core migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE build_pipelines ADD COLUMN workload_id VARCHAR(64) NOT NULL DEFAULT '' AFTER application_id"); err != nil {
		t.Fatalf("add legacy workload_id: %v", err)
	}
	assertMySQLColumnExists(t, ctx, db, "build_pipelines", "workload_id")

	drop := findBuildMigration(t, "drop_build_pipeline_workload_id")
	if err := migrator.Up(ctx, []database.Migration{drop}); err != nil {
		t.Fatalf("apply drop migration: %v", err)
	}
	assertMySQLColumnNotExists(t, ctx, db, "build_pipelines", "workload_id")
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

func assertMySQLColumnNotExists(t *testing.T, ctx context.Context, db columnQueryer, table string, column string) {
	t.Helper()
	var count int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	if err != nil {
		t.Fatalf("query column %s.%s: %v", table, column, err)
	}
	if count != 0 {
		t.Fatalf("column %s.%s count = %d, want 0", table, column, count)
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
