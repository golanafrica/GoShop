// Package testutilitis fournit des utilitaires pour les tests
package testutilitis

import (
	"database/sql"
	"fmt"
	"os"
)

// RunMigrations applique les migrations SQL à la base de données de test
func RunMigrations(db *sql.DB) error {
	sqlFile, err := os.ReadFile("../migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("lecture migration: %w", err)
	}
	_, err = db.Exec(string(sqlFile))
	return err
}
