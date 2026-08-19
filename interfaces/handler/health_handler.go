// interfaces/handler/health_handler.go
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"Goshop/application/metrics"
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

// @Summary Liveness Probe (Sonde de vivacité)
// @Description Vérifie que l'API est en cours d'exécution. Requête légère sans vérification des dépendances externes. Idéal pour les probes Kubernetes.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if h.Logger != nil {
		h.Logger.Debug().Msg("Liveness probe received")
	}

	res := map[string]string{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Liveness check
	metrics.HealthCheckTotal.WithLabelValues("liveness", "success").Inc()
	metrics.HealthCheckDuration.WithLabelValues("liveness").Observe(duration)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// @Summary Readiness Probe (Sonde de disponibilité)
// @Description Vérifie que l'API est prête à traiter les requêtes en testant les connexions aux dépendances critiques (PostgreSQL, Redis).
// @Tags Health
// @Produce json
// @Success 200 {object} handler.HealthResponse "Tous les systèmes sont opérationnels"
// @Failure 503 {object} handler.HealthResponse "Service indisponible (dépendances non prêtes)"
// @Router /health/ready [get]
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	postgresOK := false
	redisOK := false
	status := "not_ready"
	message := ""
	httpStatus := http.StatusServiceUnavailable

	// Test PostgreSQL - CRITIQUE
	if h.DB != nil {
		dbStart := time.Now()
		if err := h.DB.PingContext(ctx); err == nil {
			postgresOK = true
			metrics.HealthDependenciesStatus.WithLabelValues("postgres").Set(1)
		} else {
			metrics.HealthDependenciesStatus.WithLabelValues("postgres").Set(0)
			if h.Logger != nil {
				h.Logger.Error().Err(err).Msg("PostgreSQL readiness check failed")
			}
			message = "Database connection failed"
		}
		metrics.HealthDatabasePingDuration.Observe(time.Since(dbStart).Seconds())
	} else {
		metrics.HealthDependenciesStatus.WithLabelValues("postgres").Set(0)
		message = "Database connection not initialized"
	}

	// Test Redis (optionnel)
	if h.Rdb != nil {
		redisStart := time.Now()
		if _, err := h.Rdb.Ping(ctx).Result(); err == nil {
			redisOK = true
			metrics.HealthDependenciesStatus.WithLabelValues("redis").Set(1)
		} else {
			metrics.HealthDependenciesStatus.WithLabelValues("redis").Set(0)
			if h.Logger != nil {
				h.Logger.Warn().Err(err).Msg("Redis readiness check failed")
			}
		}
		metrics.HealthRedisPingDuration.Observe(time.Since(redisStart).Seconds())
	} else {
		// Redis non configuré = OK pour les tests
		redisOK = true
		metrics.HealthDependenciesStatus.WithLabelValues("redis").Set(1)
	}

	// Déterminer le statut final
	if postgresOK {
		status = "ready"
		httpStatus = http.StatusOK
		if message == "" {
			message = "All systems operational"
		}
	}

	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Readiness check
	if httpStatus == http.StatusOK {
		metrics.HealthCheckTotal.WithLabelValues("readiness", "success").Inc()
	} else {
		metrics.HealthCheckTotal.WithLabelValues("readiness", "error").Inc()
	}
	metrics.HealthCheckDuration.WithLabelValues("readiness").Observe(duration)

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

// @Summary Vérification de santé simple
// @Description Retourne un statut OK basique sans vérifier les dépendances externes. Utile pour les tests rapides.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *HealthHandler) SimpleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	resp := map[string]string{
		"status":    "ok",
		"message":   "API is running",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Simple health check
	metrics.HealthCheckTotal.WithLabelValues("simple", "success").Inc()
	metrics.HealthCheckDuration.WithLabelValues("simple").Observe(duration)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
