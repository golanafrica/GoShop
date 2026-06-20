package repository

import (
	"context"
	"database/sql"
)

// DBExecutor définit l'interface minimale pour exécuter des requêtes SQL
// C'est une abstraction qui permet d'utiliser *sql.DB ou repository.Tx
// Votre Tx implémente déjà cette interface implicitement
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
