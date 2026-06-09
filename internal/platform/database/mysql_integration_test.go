package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
)

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("PAAS_TEST_MYSQL_DSN")
	if dsn == "" {
		cfg := database.ConfigFromEnv()
		if cfg.Password == "" {
			t.Skip("skip MySQL integration test: set PAAS_TEST_MYSQL_DSN or MYSQL_* with MYSQL_PASSWORD")
		}
		dsn = cfg.DSN()
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("PingContext() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigratorUpDownIntegration(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()
	tableName := "shared_kernel_migration_test"
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName)
	_, _ = db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", 1001)

	migration := database.Migration{
		Version: 1001,
		Name:    "shared kernel test",
		Up:      "CREATE TABLE " + tableName + " (id BIGINT NOT NULL PRIMARY KEY) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		Down:    "DROP TABLE IF EXISTS " + tableName,
	}
	migrator := database.NewMigrator(db)

	if err := migrator.Up(ctx, []database.Migration{migration}); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO "+tableName+"(id) VALUES (1)"); err != nil {
		t.Fatalf("insert migrated table error = %v", err)
	}
	if err := migrator.Down(ctx, []database.Migration{migration}); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
}

func TestTransactorCommitAndRollbackIntegration(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()
	tableName := "shared_kernel_tx_test"
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName)
	_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (id BIGINT NOT NULL PRIMARY KEY) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")
	if err != nil {
		t.Fatalf("create table error = %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName) })

	transactor := database.NewTransactor(db)
	if err := transactor.WithinTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, db)
		_, err := exec.ExecContext(txCtx, "INSERT INTO "+tableName+"(id) VALUES (1)")
		return err
	}); err != nil {
		t.Fatalf("commit transaction error = %v", err)
	}

	if err := transactor.WithinTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, db)
		if _, err := exec.ExecContext(txCtx, "INSERT INTO "+tableName+"(id) VALUES (2)"); err != nil {
			return err
		}
		return errors.New("force rollback")
	}); err == nil {
		t.Fatalf("rollback transaction should return error")
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&count); err != nil {
		t.Fatalf("count query error = %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 after rollback", count)
	}
}
