package appenv

import (
	"context"
	"strings"
	"testing"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/testsupport"
)

func TestMigrationsBackfillJenkinsTemplateIDForExistingApplicationSources(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	oldCore.Up = strings.Replace(oldCore.Up, "  jenkins_template_id VARCHAR(64) NOT NULL DEFAULT '',\n", "", 1)
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove jenkins_template_id from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	if err := migrator.Up(ctx, Migrations[1:]); err != nil {
		t.Fatalf("apply current follow-up migrations: %v", err)
	}
	for _, column := range []string{"jenkins_template_id", "build_environment_id"} {
		var count int
		err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'application_sources' AND column_name = ?`, column).Scan(&count)
		if err != nil {
			t.Fatalf("query column %s: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("column %s count = %d, want 1", column, count)
		}
	}
}

func TestMigrationsCreateWorkloadTables(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t, Migrations...)

	checkColumns := map[string][]string{
		"workloads": {
			"id",
			"tenant_id",
			"project_id",
			"application_id",
			"name",
			"workload_type",
			"status",
			"active_name",
			"image_source_mode",
			"pipeline_id",
			"created_by",
		},
		"workload_stage_configs": {
			"id",
			"application_id",
			"workload_id",
			"stage_key",
			"replicas",
			"service_ports_json",
			"resource_requests_json",
			"resource_limits_json",
			"probes_json",
			"ingress_hosts_json",
			"config_files_json",
			"writable_dirs_json",
			"values_override_json",
		},
		"workload_default_configs": {
			"id",
			"application_id",
			"workload_id",
			"replicas",
			"service_ports_json",
			"resource_requests_json",
			"resource_limits_json",
			"probes_json",
			"ingress_hosts_json",
			"config_files_json",
			"writable_dirs_json",
			"values_override_json",
		},
	}
		for table, columns := range checkColumns {
		for _, column := range columns {
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
	}
	var uniqueCount int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = 'workloads'
  AND index_name = 'uk_workloads_application_active_name'
  AND column_name IN ('application_id', 'active_name')`).Scan(&uniqueCount); err != nil {
		t.Fatalf("query workload active unique key: %v", err)
	}
	if uniqueCount != 2 {
		t.Fatalf("workloads active unique key column count = %d, want 2", uniqueCount)
	}
}
