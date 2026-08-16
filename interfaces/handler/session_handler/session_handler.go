package sessionhandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"Goshop/application/metrics"
	sessionusecase "Goshop/application/usecase/session_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.2 : SESSION MANAGEMENT HANDLER
// ============================================================

// SessionHandler gère les endpoints de gestion des sessions
type SessionHandler struct {
	listSessionsUC      *sessionusecase.ListSessionsUsecase
	revokeSessionUC     *sessionusecase.RevokeSessionUsecase
	revokeAllSessionsUC *sessionusecase.RevokeAllSessionsUsecase
	getStatsUC          *sessionusecase.GetSessionStatsUsecase
	cleanupUC           *sessionusecase.CleanupSessionsUsecase
}

// NewSessionHandler crée une nouvelle instance
func NewSessionHandler(
	listSessionsUC *sessionusecase.ListSessionsUsecase,
	revokeSessionUC *sessionusecase.RevokeSessionUsecase,
	revokeAllSessionsUC *sessionusecase.RevokeAllSessionsUsecase,
	getStatsUC *sessionusecase.GetSessionStatsUsecase,
	cleanupUC *sessionusecase.CleanupSessionsUsecase,
) *SessionHandler {
	return &SessionHandler{
		listSessionsUC:      listSessionsUC,
		revokeSessionUC:     revokeSessionUC,
		revokeAllSessionsUC: revokeAllSessionsUC,
		getStatsUC:          getStatsUC,
		cleanupUC:           cleanupUC,
	}
}

// ============================================================
// ENDPOINT 1 : LIST SESSIONS
// ============================================================

func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	userID := r.URL.Query().Get("user_id")
	activeOnlyStr := r.URL.Query().Get("active_only")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	activeOnly := activeOnlyStr == "true"
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	req := &sessionusecase.ListSessionsRequest{
		UserID:     userID,
		ActiveOnly: activeOnly,
		Limit:      limit,
		Offset:     offset,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Bool("active_only", activeOnly).
		Msg("📋 List sessions request")

	response, err := h.listSessionsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.SessionOperationTotal.WithLabelValues("list", "error").Inc()
		metrics.SessionOperationDuration.WithLabelValues("list").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("session_list", "session_handler").Inc()
		return handleSessionError(err)
	}

	// 📊 MÉTRIQUES : Succès
	metrics.SessionOperationTotal.WithLabelValues("list", "success").Inc()
	metrics.SessionOperationDuration.WithLabelValues("list").Observe(duration)

	// ✅ Utilisation du vrai champ Total
	totalSessions := 0
	if response != nil {
		totalSessions = response.Total
	}
	metrics.SessionsListedCount.Observe(float64(totalSessions))

	logger.Info().
		Str("admin_id", admin.AdminID).
		Int("total_sessions", totalSessions).
		Float64("duration_seconds", duration).
		Msg("Sessions listed successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 2 : GET STATISTICS
// ============================================================

func (h *SessionHandler) GetStatistics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	userID := r.URL.Query().Get("user_id")

	req := &sessionusecase.GetSessionStatsRequest{
		UserID: userID,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Msg("📊 Get session statistics request")

	response, err := h.getStatsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.SessionOperationTotal.WithLabelValues("get_stats", "error").Inc()
		metrics.SessionOperationDuration.WithLabelValues("get_stats").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("session_stats", "session_handler").Inc()
		return handleSessionError(err)
	}

	metrics.SessionOperationTotal.WithLabelValues("get_stats", "success").Inc()
	metrics.SessionOperationDuration.WithLabelValues("get_stats").Observe(duration)

	// ✅ Utilisation du vrai champ Statistics (entity.SessionStatistics)
	var topUsersCount int
	if response != nil && response.TopActiveUsers != nil {
		topUsersCount = len(response.TopActiveUsers)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Int("top_users_count", topUsersCount).
		Float64("duration_seconds", duration).
		Msg("Session statistics retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 3 : REVOKE SESSION
// ============================================================

func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.RevokeSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	if req.TargetSessionID == "" {
		return utils.NewAppError("MISSING_FIELD", "target_session_id is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_session_id", req.TargetSessionID).
		Msg("🔐 Revoke session request")

	response, err := h.revokeSessionUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.SessionOperationTotal.WithLabelValues("revoke", "error").Inc()
		metrics.SessionOperationDuration.WithLabelValues("revoke").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("session_revoke", "session_handler").Inc()
		return handleSessionError(err)
	}

	metrics.SessionOperationTotal.WithLabelValues("revoke", "success").Inc()
	metrics.SessionOperationDuration.WithLabelValues("revoke").Observe(duration)
	metrics.SessionRevokedTotal.WithLabelValues("single").Inc()

	// ✅ Utilisation du vrai champ RevokedSessionID
	var revokedID string
	if response != nil {
		revokedID = response.RevokedSessionID
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("revoked_session_id", revokedID).
		Float64("duration_seconds", duration).
		Msg("Session revoked successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 4 : REVOKE ALL SESSIONS
// ============================================================

func (h *SessionHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.RevokeAllSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = sessionusecase.RevokeAllSessionsRequest{}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Msg("🔐 Revoke all sessions request")

	response, err := h.revokeAllSessionsUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.SessionOperationTotal.WithLabelValues("revoke_all", "error").Inc()
		metrics.SessionOperationDuration.WithLabelValues("revoke_all").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("session_revoke_all", "session_handler").Inc()
		return handleSessionError(err)
	}

	metrics.SessionOperationTotal.WithLabelValues("revoke_all", "success").Inc()
	metrics.SessionOperationDuration.WithLabelValues("revoke_all").Observe(duration)
	metrics.SessionRevokedTotal.WithLabelValues("bulk").Inc()

	// ✅ Utilisation du vrai champ RevokedCount
	revokedCount := 0
	if response != nil {
		revokedCount = response.RevokedCount
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Int("revoked_count", revokedCount).
		Float64("duration_seconds", duration).
		Msg("All sessions revoked successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 5 : CLEANUP SESSIONS
// ============================================================

func (h *SessionHandler) CleanupSessions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.CleanupSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = sessionusecase.CleanupSessionsRequest{
			IncludeRevoked: false,
		}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("include_revoked", req.IncludeRevoked).
		Msg("🧹 Cleanup sessions request")

	response, err := h.cleanupUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.SessionOperationTotal.WithLabelValues("cleanup", "error").Inc()
		metrics.SessionOperationDuration.WithLabelValues("cleanup").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("session_cleanup", "session_handler").Inc()
		return handleSessionError(err)
	}

	metrics.SessionOperationTotal.WithLabelValues("cleanup", "success").Inc()
	metrics.SessionOperationDuration.WithLabelValues("cleanup").Observe(duration)

	// ✅ Utilisation du vrai champ DeletedCount
	deletedCount := 0
	if response != nil {
		deletedCount = response.DeletedCount
	}
	metrics.SessionsCleanedTotal.Add(float64(deletedCount))

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("include_revoked", req.IncludeRevoked).
		Int("deleted_count", deletedCount).
		Float64("duration_seconds", duration).
		Msg("Sessions cleaned up successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

func (h *SessionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", middl.ErrorHandler(h.ListSessions))
	r.Get("/stats", middl.ErrorHandler(h.GetStatistics))
	r.Post("/revoke", middl.ErrorHandler(h.RevokeSession))
	r.Post("/revoke-all", middl.ErrorHandler(h.RevokeAllSessions))
	r.Post("/cleanup", middl.ErrorHandler(h.CleanupSessions))
}

// ============================================================
// HELPERS
// ============================================================

func extractAdminContext(r *http.Request) (*sessionusecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())
	adminEmail, _ := utils.UserEmailFromContext(r.Context())
	currentSessionID, _ := utils.SessionIDFromContext(r.Context())

	return &sessionusecase.AdminContext{
		AdminID:          adminID,
		AdminEmail:       adminEmail,
		AdminRole:        adminRole,
		IPAddress:        extractIPWithoutPort(r.RemoteAddr),
		UserAgent:        r.UserAgent(),
		CurrentSessionID: currentSessionID,
	}, nil
}

func extractIPWithoutPort(addr string) string {
	if addr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	return host
}

func handleSessionError(err error) error {
	errMsg := err.Error()

	switch {
	case errors.Is(err, entity.ErrSessionNotFound):
		return utils.NewAppError("SESSION_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, entity.ErrSessionExpired):
		return utils.NewAppError("SESSION_EXPIRED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrSessionRevoked):
		return utils.NewAppError("SESSION_REVOKED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrSessionInvalidToken):
		return utils.NewAppError("SESSION_INVALID_TOKEN", errMsg, http.StatusUnauthorized)
	case errors.Is(err, entity.ErrSessionAlreadyActive):
		return utils.NewAppError("SESSION_ALREADY_ACTIVE", errMsg, http.StatusConflict)

	case errors.Is(err, repository.ErrSessionNotFound):
		return utils.NewAppError("SESSION_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrSessionAlreadyExists):
		return utils.NewAppError("SESSION_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrSessionInvalidData):
		return utils.NewAppError("SESSION_INVALID_DATA", errMsg, http.StatusBadRequest)
	case errors.Is(err, repository.ErrSessionTokenMismatch):
		return utils.NewAppError("SESSION_TOKEN_MISMATCH", errMsg, http.StatusUnauthorized)

	case errMsg == "insufficient permissions: can only view own sessions":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: can only revoke own sessions":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: admin role required":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: super_admin role required":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "cannot revoke current active session":
		return utils.NewAppError("CANNOT_REVOKE_CURRENT", errMsg, http.StatusBadRequest)
	case errMsg == "session is already revoked":
		return utils.NewAppError("SESSION_ALREADY_REVOKED", errMsg, http.StatusConflict)
	case errMsg == "session not found":
		return utils.NewAppError("SESSION_NOT_FOUND", errMsg, http.StatusNotFound)
	case errMsg == "target_session_id is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)

	default:
		return err
	}
}
