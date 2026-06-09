package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shareinto/paas/internal/shared"
)

func newMySQLRepositoryForLogTest(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	snapshot := buildSnapshot{
		Runs: []BuildRun{{
			ID:            "build_run_1",
			TenantID:      "tenant_1",
			ProjectID:     "project_1",
			PipelineID:    "pipeline_1",
			ApplicationID: "app_1",
			Status:        BuildRunRunning,
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload FROM repository_snapshots WHERE module = ?")).
		WithArgs("build").
		WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(string(payload)))

	repo, err := NewMySQLRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo, mock, db
}

func TestMySQLRepositoryAppendBuildLogWritesAppendOnlyTable(t *testing.T) {
	repo, mock, db := newMySQLRepositoryForLogTest(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO build_logs(build_run_id, log_text, created_at) VALUES (?, ?, ?)")).
		WithArgs(shared.ID("build_run_1"), "line 1\n", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.AppendBuildLog(context.Background(), "build_run_1", "line 1\n"); err != nil {
		t.Fatalf("AppendBuildLog() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMySQLRepositoryAppendBuildLogSplitsLargeChunks(t *testing.T) {
	repo, mock, db := newMySQLRepositoryForLogTest(t)
	defer db.Close()

	first := strings.Repeat("a", maxBuildLogChunkBytes)
	second := strings.Repeat("b", 10*1024)
	text := first + second
	insertSQL := regexp.QuoteMeta("INSERT INTO build_logs(build_run_id, log_text, created_at) VALUES (?, ?, ?)")
	mock.ExpectExec(insertSQL).
		WithArgs(shared.ID("build_run_1"), first, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(insertSQL).
		WithArgs(shared.ID("build_run_1"), second, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))

	if err := repo.AppendBuildLog(context.Background(), "build_run_1", text); err != nil {
		t.Fatalf("AppendBuildLog() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMySQLRepositoryListBuildLogsReadsAppendOnlyTable(t *testing.T) {
	repo, mock, db := newMySQLRepositoryForLogTest(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT log_text FROM build_logs WHERE build_run_id = ? ORDER BY id")).
		WithArgs(shared.ID("build_run_1")).
		WillReturnRows(sqlmock.NewRows([]string{"log_text"}).AddRow("line 1\n").AddRow("line 2\n"))

	logs, err := repo.ListBuildLogs(context.Background(), "build_run_1")
	if err != nil {
		t.Fatalf("ListBuildLogs() error = %v", err)
	}
	if len(logs) != 2 || logs[0] != "line 1\n" || logs[1] != "line 2\n" {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
