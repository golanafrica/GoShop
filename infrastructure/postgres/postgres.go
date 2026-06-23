// infrastructure/postgres/postgres.go - VERSION CORRIGÉE
package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func Connect(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("erreur ouverture de la base de donnée: %v", err)
	}

	// ✅ NOUVEAU : Forcer UTC pour tous les timestamps
	_, err = db.Exec("SET TIME ZONE 'UTC'")
	if err != nil {
		return nil, fmt.Errorf("set timezone UTC: %w", err)
	}

	// ⚡ CONFIGURATION CRITIQUE DU POOL ⚡
	db.SetMaxOpenConns(50)                 // Max connections ouvertes
	db.SetMaxIdleConns(25)                 // 50% de MaxOpenConns
	db.SetConnMaxLifetime(5 * time.Minute) // Recycler les connexions
	db.SetConnMaxIdleTime(2 * time.Minute) // Fermer les connexions inactives

	// Test de connexion
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erreur ping DB: %v", err)
	}

	// Monitoring du pool
	go monitorConnectionPool(db)

	log.Println("✅ Connexion PostgreSQL réussie avec pool configuré (UTC)")
	return db, nil
}

func monitorConnectionPool(db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := db.Stats()
		log.Printf("📊 DB Pool Stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, WaitDuration=%v",
			stats.OpenConnections,
			stats.InUse,
			stats.Idle,
			stats.WaitCount,
			stats.WaitDuration)

		// Alert si trop d'attente
		if stats.WaitCount > 100 {
			log.Printf("⚠️  ALERT: DB Pool saturation! WaitCount=%d", stats.WaitCount)
		}
	}
}
