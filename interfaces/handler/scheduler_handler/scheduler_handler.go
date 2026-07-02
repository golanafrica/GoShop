package schedulerhandler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	appscheduler "Goshop/application/scheduler"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// HANDLER ADMIN DU SCHEDULER
// ============================================================

// SchedulerHandler gère les endpoints admin du scheduler
type SchedulerHandler struct {
	scheduler *appscheduler.CommissionScheduler
	batchRepo repository.CommissionBatchRepository
}

// NewSchedulerHandler crée une nouvelle instance
func NewSchedulerHandler(
	scheduler *appscheduler.CommissionScheduler,
	batchRepo repository.CommissionBatchRepository,
) *SchedulerHandler {
	return &SchedulerHandler{
		scheduler: scheduler,
		batchRepo: batchRepo,
	}
}

// ============================================================
// ENDPOINTS
// ============================================================

// TriggerManualCollection déclenche manuellement la collecte
// POST /api/admin/scheduler/trigger
func (h *SchedulerHandler) TriggerManualCollection(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("🔧 Manual commission collection triggered by admin")

	// Récupérer l'ID de l'admin qui déclenche (optionnel)
	executedBy := r.Header.Get("X-Admin-ID")
	if executedBy == "" {
		executedBy = "admin-manual"
	}

	// Lancer la collecte en arrière-plan
	go func() {
		bgCtx := context.Background()
		if err := h.scheduler.RunNightlyCollection(bgCtx); err != nil {
			logger.Error().
				Err(err).
				Str("executed_by", executedBy).
				Msg("❌ Manual collection failed")
		}
	}()

	// Réponse immédiate
	utils.WriteJSON(w, http.StatusAccepted, map[string]interface{}{
		"success":     true,
		"message":     "Commission collection triggered in background",
		"executed_by": executedBy,
	})
	return nil
}

// GetRecentBatches retourne les N derniers batches
// GET /api/admin/scheduler/batches?limit=10
func (h *SchedulerHandler) GetRecentBatches(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Paramètre limit (défaut: 10, max: 100)
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	logger.Debug().
		Int("limit", limit).
		Msg("Fetching recent batches")

	batches, err := h.batchRepo.FindRecentBatches(ctx, limit)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch batches")
		return utils.NewAppError("BATCH_FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(batches),
		"batches": batches,
	})
	return nil
}

// GetBatchDetails retourne les détails d'un batch spécifique
// GET /api/admin/scheduler/batches/{id}
func (h *SchedulerHandler) GetBatchDetails(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Récupérer l'ID du batch depuis l'URL
	batchID := r.PathValue("id")
	if batchID == "" {
		return utils.NewAppError("MISSING_BATCH_ID", "batch ID is required", http.StatusBadRequest)
	}

	logger.Debug().
		Str("batch_id", batchID).
		Msg("Fetching batch details")

	// Récupérer le batch
	batch, err := h.batchRepo.FindBatchByID(ctx, batchID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch batch")
		return utils.NewAppError("BATCH_NOT_FOUND", err.Error(), http.StatusNotFound)
	}

	// Récupérer les items du batch
	items, err := h.batchRepo.FindItemsByBatchID(ctx, batchID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch batch items")
		return utils.NewAppError("ITEMS_FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"batch":   batch,
		"items":   items,
		"count":   len(items),
	})
	return nil
}

// GetDailyStats retourne les statistiques quotidiennes
// GET /api/admin/scheduler/stats?days=30
func (h *SchedulerHandler) GetDailyStats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Paramètre days (défaut: 30, max: 365)
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	logger.Debug().
		Int("days", days).
		Msg("Fetching daily stats")

	stats, err := h.batchRepo.GetDailyStats(ctx, days)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch stats")
		return utils.NewAppError("STATS_FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// Calculer les totaux
	var totalCollected int64
	var totalFailed int64
	var totalSuccessful int
	var totalFailedCount int

	for _, stat := range stats {
		totalCollected += stat.TotalCollectedCents
		totalFailed += stat.TotalFailedCents
		totalSuccessful += stat.TotalSuccessful
		totalFailedCount += stat.TotalFailed
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"days":    days,
		"stats":   stats,
		"summary": map[string]interface{}{
			"total_collected_cents": totalCollected,
			"total_collected_fcfa":  totalCollected / 100,
			"total_failed_cents":    totalFailed,
			"total_failed_fcfa":     totalFailed / 100,
			"total_successful":      totalSuccessful,
			"total_failed":          totalFailedCount,
			"success_rate_percent":  calculateSuccessRate(totalSuccessful, totalFailedCount),
		},
	})
	return nil
}

// ============================================================
// HELPERS
// ============================================================

func calculateSuccessRate(success, failed int) float64 {
	total := success + failed
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}

// Silence "imported and not used" pour fmt
var _ = fmt.Sprintf
