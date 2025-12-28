package repository

import (
	"context"
	"database/sql"
)

//go:generate mockgen -destination=../../mocks/repository/mock_tx.go -package=repository . Tx
//go:generate mockgen -destination=../../mocks/repository/mock_tx_manager.go -package=repository . TxManager

type Tx interface {
	Commit() error
	Rollback() error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type TxManager interface {
	BeginTx(ctx context.Context) (Tx, error)
}
