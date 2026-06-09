package database_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shareinto/paas/internal/platform/database"
)

func TestMigratorUpAppliesPendingMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE test_table (id BIGINT NOT NULL PRIMARY KEY)")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)")).
		WithArgs(int64(1), "create test table", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	migrator := database.NewMigrator(db)
	err = migrator.Up(context.Background(), []database.Migration{{
		Version: 1,
		Name:    "create test table",
		Up:      "CREATE TABLE test_table (id BIGINT NOT NULL PRIMARY KEY)",
		Down:    "DROP TABLE test_table",
	}})
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMigratorUpExecutesMultiStatementMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE first_table (id BIGINT NOT NULL PRIMARY KEY)")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE second_table (name VARCHAR(64) NOT NULL DEFAULT ';')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)")).
		WithArgs(int64(2), "multi statement", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	migrator := database.NewMigrator(db)
	err = migrator.Up(context.Background(), []database.Migration{{
		Version: 2,
		Name:    "multi statement",
		Up: `
CREATE TABLE first_table (id BIGINT NOT NULL PRIMARY KEY);
CREATE TABLE second_table (name VARCHAR(64) NOT NULL DEFAULT ';');
`,
		Down: `
DROP TABLE IF EXISTS second_table;
DROP TABLE IF EXISTS first_table;
`,
	}})
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMigratorUpSkipsAppliedMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), []database.Migration{{Version: 1, Name: "done", Up: "SELECT 1", Down: "SELECT 1"}}); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMigratorUpReturnsEnsureTableError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnError(errors.New("ddl failed"))

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), nil); err == nil {
		t.Fatalf("Up() should return ensure table error")
	}
}

func TestMigratorUpReturnsAppliedQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(errors.New("select failed"))

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), []database.Migration{{Version: 1, Up: "SELECT 1"}}); err == nil {
		t.Fatalf("Up() should return applied query error")
	}
}

func TestMigratorUpRollsBackOnMigrationExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("BROKEN SQL").WillReturnError(errors.New("syntax error"))
	mock.ExpectRollback()

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), []database.Migration{{Version: 1, Up: "BROKEN SQL"}}); err == nil {
		t.Fatalf("Up() should return migration exec error")
	}
}

func TestMigratorUpRollsBackOnVersionInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)")).
		WithArgs(int64(1), "", sqlmock.AnyArg()).
		WillReturnError(errors.New("insert version failed"))
	mock.ExpectRollback()

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), []database.Migration{{Version: 1, Up: "SELECT 1"}}); err == nil {
		t.Fatalf("Up() should return version insert error")
	}
}

func TestMigratorDownAppliesRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE test_table")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	migrator := database.NewMigrator(db)
	err = migrator.Down(context.Background(), []database.Migration{{
		Version: 1,
		Name:    "drop test table",
		Up:      "CREATE TABLE test_table (id BIGINT NOT NULL PRIMARY KEY)",
		Down:    "DROP TABLE test_table",
	}})
	if err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMigratorDownSkipsUnappliedMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	migrator := database.NewMigrator(db)
	if err := migrator.Down(context.Background(), []database.Migration{{Version: 1, Down: "DROP TABLE test_table"}}); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
}

func TestMigratorDownRejectsEmptySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	migrator := database.NewMigrator(db)
	if err := migrator.Down(context.Background(), []database.Migration{{Version: 1}}); err == nil {
		t.Fatalf("empty down sql should fail")
	}
}

func TestMigratorDownReturnsAppliedQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(errors.New("select failed"))

	migrator := database.NewMigrator(db)
	if err := migrator.Down(context.Background(), []database.Migration{{Version: 1, Down: "DROP TABLE test_table"}}); err == nil {
		t.Fatalf("Down() should return applied query error")
	}
}

func TestMigratorDownRollsBackOnExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("DROP BROKEN").WillReturnError(errors.New("drop failed"))
	mock.ExpectRollback()

	migrator := database.NewMigrator(db)
	if err := migrator.Down(context.Background(), []database.Migration{{Version: 1, Down: "DROP BROKEN"}}); err == nil {
		t.Fatalf("Down() should return exec error")
	}
}

func TestMigratorDownRollsBackOnVersionDeleteError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE test_table").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(errors.New("delete version failed"))
	mock.ExpectRollback()

	migrator := database.NewMigrator(db)
	if err := migrator.Down(context.Background(), []database.Migration{{Version: 1, Down: "DROP TABLE test_table"}}); err == nil {
		t.Fatalf("Down() should return version delete error")
	}
}

func TestMigratorRejectsEmptySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM schema_migrations WHERE version = ?")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	migrator := database.NewMigrator(db)
	if err := migrator.Up(context.Background(), []database.Migration{{Version: 1}}); err == nil {
		t.Fatalf("empty up sql should fail")
	}
}

func TestTransactorCommitRollbackAndNested(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	transactor := database.NewTransactor(db)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO tx_test").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err = transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		if database.TxFromContext(ctx) == nil {
			t.Fatalf("tx should be present in context")
		}
		_, err := database.ExecutorFromContext(ctx, db).ExecContext(ctx, "INSERT INTO tx_test VALUES (1)")
		return err
	})
	if err != nil {
		t.Fatalf("commit tx error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectRollback()
	err = transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		return errors.New("rollback")
	})
	if err == nil {
		t.Fatalf("rollback tx should return original error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTransactorNestedUsesExistingTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	transactor := database.NewTransactor(db)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err = transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		return transactor.WithinTx(ctx, func(nested context.Context) error {
			if database.TxFromContext(nested) == nil {
				t.Fatalf("nested tx should reuse existing transaction")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatalf("nested transaction error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestExecutorFromContextReturnsDBWithoutTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	if exec := database.ExecutorFromContext(context.Background(), db); exec != db {
		t.Fatalf("ExecutorFromContext should return db when no tx exists")
	}
}
