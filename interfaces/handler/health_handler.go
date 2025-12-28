// interfaces/handler/health_handler.go (ou handlers/health_handler.go)
// interfaces/handler/health_handler.go
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"Goshop/config/setupLogging"

	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	DB     *sql.DB
	Rdb    *redis.Client
	Logger *setupLogging.Logger
}

type HealthResponse struct {
	Status    string `json:"status"`
	Postgres  bool   `json:"postgres"`
	Redis     bool   `json:"redis"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message,omitempty"`
}

// Live — Liveness probe: léger, pas de dépendances
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	if h.Logger != nil {
		h.Logger.Debug().Msg("Liveness probe received")
	}

	res := map[string]string{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// Ready — Readiness probe: vérifie les dépendances critiques
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	postgresOK := false
	redisOK := false
	status := "not_ready"
	message := ""
	httpStatus := http.StatusServiceUnavailable

	// Test PostgreSQL - CRITIQUE
	if h.DB != nil {
		if err := h.DB.PingContext(ctx); err == nil {
			postgresOK = true
		} else if h.Logger != nil {
			h.Logger.Error().Err(err).Msg("PostgreSQL readiness check failed")
			message = "Database connection failed"
		}
	} else {
		message = "Database connection not initialized"
	}

	// Test Redis (optionnel)
	if h.Rdb != nil {
		if _, err := h.Rdb.Ping(ctx).Result(); err == nil {
			redisOK = true
		} else if h.Logger != nil {
			h.Logger.Warn().Err(err).Msg("Redis readiness check failed")
		}
	} else {
		// Redis non configuré = OK pour les tests
		redisOK = true
	}

	// Déterminer le statut final
	if postgresOK {
		status = "ready"
		httpStatus = http.StatusOK
		if message == "" {
			message = "All systems operational"
		}
	}

	resp := HealthResponse{
		Status:    status,
		Postgres:  postgresOK,
		Redis:     redisOK,
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(resp)
}

// SimpleHealth pour les tests sans dépendances
func (h *HealthHandler) SimpleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":    "ok",
		"message":   "API is running",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
