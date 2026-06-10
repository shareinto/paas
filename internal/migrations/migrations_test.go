package migrations_test

import (
	"strings"
	"testing"

	"github.com/shareinto/paas/internal/migrations"
)

func TestAllMigrationsAreUniqueAndOrderedByVersion(t *testing.T) {
	all := migrations.All()
	seen := map[int64]string{}
	for _, migration := range all {
		if migration.Version == 0 {
			t.Fatalf("migration %q has empty version", migration.Name)
		}
		if migration.Name == "" {
			t.Fatalf("migration %d has empty name", migration.Version)
		}
		if migration.Up == "" || migration.Down == "" {
			t.Fatalf("migration %d %q must define up and down SQL", migration.Version, migration.Name)
		}
		if existing, ok := seen[migration.Version]; ok {
			t.Fatalf("duplicate migration version %d: %q and %q", migration.Version, existing, migration.Name)
		}
		seen[migration.Version] = migration.Name
	}
}

func TestRepositorySnapshotsAreDroppedByFinalMigration(t *testing.T) {
	var found bool
	for _, migration := range migrations.All() {
		if migration.Name != "drop_repository_snapshots" {
			continue
		}
		found = true
		if !strings.Contains(migration.Up, "DROP TABLE IF EXISTS repository_snapshots") {
			t.Fatalf("drop migration must remove repository_snapshots table:\n%s", migration.Up)
		}
	}
	if !found {
		t.Fatalf("drop_repository_snapshots migration not found")
	}
}

func TestApplicationSourceJenkinsTemplateColumnIsBackfilledBeforeRuntimeEnvironmentMigration(t *testing.T) {
	all := migrations.All()
	var runtimeVersion int64
	for _, migration := range all {
		if migration.Name == "application_runtime_environment_tables" {
			runtimeVersion = migration.Version
			break
		}
	}
	if runtimeVersion == 0 {
		t.Fatalf("application_runtime_environment_tables migration not found")
	}

	var found bool
	for _, migration := range all {
		if migration.Version >= runtimeVersion {
			continue
		}
		if !strings.Contains(migration.Up, "application_sources") ||
			!strings.Contains(migration.Up, "jenkins_template_id") ||
			!strings.Contains(migration.Up, "ADD COLUMN jenkins_template_id") {
			continue
		}
		found = true
		if !strings.Contains(migration.Up, "information_schema.columns") {
			t.Fatalf("jenkins_template_id backfill migration must be safe for fresh databases:\n%s", migration.Up)
		}
	}
	if !found {
		t.Fatalf("missing application_sources.jenkins_template_id backfill migration before application_runtime_environment_tables")
	}
}

func TestRepositorySnapshotCleanupMigrationRemovesDeprecatedBuildSpecFieldsAndStaticPresets(t *testing.T) {
	var found bool
	for _, migration := range migrations.All() {
		if migration.Name != "cleanup_buildspec_static_snapshot_fields" {
			continue
		}
		found = true
		for _, want := range []string{
			"build_env_node_static",
			"runtime_env_nginx",
			"JSON_REMOVE",
			"start_command",
			"target_path",
			"default_build_command",
			"default_artifact_path",
			"artifact_path",
			"DefaultBuildCommand",
			"DefaultArtifactPath",
			"ArtifactPath",
			"StartCommand",
			"TargetPath",
		} {
			if !strings.Contains(migration.Up, want) {
				t.Fatalf("cleanup migration Up SQL missing %q:\n%s", want, migration.Up)
			}
		}
	}
	if !found {
		t.Fatalf("cleanup_buildspec_static_snapshot_fields migration not found")
	}
}

func TestBuildLogsMigrationDropsBuildRunsForeignKey(t *testing.T) {
	var found bool
	for _, migration := range migrations.All() {
		if migration.Name != "drop_build_logs_run_foreign_key" {
			continue
		}
		found = true
		for _, want := range []string{
			"build_logs",
			"fk_build_logs_run",
			"DROP FOREIGN KEY",
		} {
			if !strings.Contains(migration.Up, want) {
				t.Fatalf("drop build log foreign key migration Up SQL missing %q:\n%s", want, migration.Up)
			}
		}
	}
	if !found {
		t.Fatalf("drop_build_logs_run_foreign_key migration not found")
	}
}

func TestBuildCorePipelineColumnsAreBackfilled(t *testing.T) {
	var found bool
	for _, migration := range migrations.All() {
		if migration.Name != "backfill_build_core_pipeline_columns" {
			continue
		}
		found = true
		for _, want := range []string{
			"information_schema.columns",
			"ADD COLUMN",
			"build_pipelines",
			"name",
			"display_name",
			"description",
			"template_id",
			"config_hash",
			"managed_by_platform",
			"build_runs",
			"pipeline_name",
			"pipeline_display_name",
			"build_pipeline_sources",
			"build_environment_id",
		} {
			if !strings.Contains(migration.Up, want) {
				t.Fatalf("build core backfill migration Up SQL missing %q:\n%s", want, migration.Up)
			}
		}
	}
	if !found {
		t.Fatalf("backfill_build_core_pipeline_columns migration not found")
	}
}

func TestBuildPipelineIdentityColumnsAreBackfilled(t *testing.T) {
	var found bool
	for _, migration := range migrations.All() {
		if migration.Name != "backfill_build_pipeline_identity_columns" {
			continue
		}
		found = true
		for _, want := range []string{
			"information_schema.columns",
			"ADD COLUMN",
			"build_pipelines",
			"name",
			"display_name",
			"description",
		} {
			if !strings.Contains(migration.Up, want) {
				t.Fatalf("build pipeline identity backfill migration Up SQL missing %q:\n%s", want, migration.Up)
			}
		}
	}
	if !found {
		t.Fatalf("backfill_build_pipeline_identity_columns migration not found")
	}
}

func TestAllReturnsCopy(t *testing.T) {
	first := migrations.All()
	second := migrations.All()
	first[0].Name = "mutated"
	if second[0].Name == "mutated" {
		t.Fatalf("All() should return a copy")
	}
}
