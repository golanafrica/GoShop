package postgres

import (
	"context"
	"database/sql"
)

type SqlTx struct {
	tx *sql.Tx
}

func (t *SqlTx) Commit() error {
	return t.tx.Commit()
}

func (t *SqlTx) Rollback() error {
	return t.tx.Rollback()
}

func NewSqlTx(tx *sql.Tx) *SqlTx {
	return &SqlTx{tx: tx}
}

func (t *SqlTx) Raw() *sql.Tx {
	return t.tx
}

func (t *SqlTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *SqlTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *SqlTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}
