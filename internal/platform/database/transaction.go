package database

import (
	"context"
	"database/sql"
)

type contextKey string

const txContextKey contextKey = "paas_tx"

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Transactor struct {
	db *sql.DB
}

func NewTransactor(db *sql.DB) *Transactor {
	return &Transactor{db: db}
}

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if TxFromContext(ctx) != nil {
		return fn(ctx)
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, txContextKey, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func TxFromContext(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(txContextKey).(*sql.Tx)
	return tx
}

func ExecutorFromContext(ctx context.Context, db *sql.DB) Executor {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return db
}
