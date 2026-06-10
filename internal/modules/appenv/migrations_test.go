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
