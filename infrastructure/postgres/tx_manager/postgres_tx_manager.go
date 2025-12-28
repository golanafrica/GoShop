package txmanager

import (
	"Goshop/domain/repository"
	"Goshop/infrastructure/postgres"
	"context"
	"database/sql"
)

type TxManagerPostgresInfra struct {
	db *sql.DB
}

func NewTxManagerPostgresInfra(db *sql.DB) *TxManagerPostgresInfra {
	return &TxManagerPostgresInfra{db: db}
}

func (tmp *TxManagerPostgresInfra) BeginTx(ctx context.Context) (repository.Tx, error) {

	tx, err := tmp.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return postgres.NewSqlTx(tx), nil
}
