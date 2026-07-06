package sessionhandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"

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
//
// 🎯 Objectif :
//   Exposer les endpoints HTTP pour la gestion des sessions.
//
// 📋 Endpoints :
//   - GET    /api/admin/sessions         - Lister les sessions
//   - GET    /api/admin/sessions/stats   - Statistiques globales
//   - POST   /api/admin/sessions/revoke  - Révoquer une session
//   - POST   /api/admin/sessions/revoke-all - Révoquer toutes
//   - POST   /api/admin/sessions/cleanup - Nettoyage (super_admin)
//
// 🔐 Sécurité :
//   - Tous les endpoints nécessitent une authentification admin
//   - RBAC : super_admin / admin
//   - Protection contre auto-revocation de la session courante
//
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

// ListSessions liste les sessions de l'utilisateur authentifié
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	// Paramètres optionnels
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

	response, err := h.listSessionsUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleSessionError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 2 : GET STATISTICS
// ============================================================

// GetStatistics retourne les statistiques des sessions
func (h *SessionHandler) GetStatistics(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.getStatsUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleSessionError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 3 : REVOKE SESSION
// ============================================================

// RevokeSession révoque une session spécifique
func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.RevokeSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	// Validation
	if req.TargetSessionID == "" {
		return utils.NewAppError("MISSING_FIELD", "target_session_id is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_session_id", req.TargetSessionID).
		Msg("🔐 Revoke session request")

	response, err := h.revokeSessionUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleSessionError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 4 : REVOKE ALL SESSIONS
// ============================================================

// RevokeAllSessions révoque toutes les sessions (sauf courante)
func (h *SessionHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.RevokeAllSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body optionnel, continuer avec valeurs par défaut
		req = sessionusecase.RevokeAllSessionsRequest{}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Msg("🔐 Revoke all sessions request")

	response, err := h.revokeAllSessionsUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleSessionError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 5 : CLEANUP SESSIONS
// ============================================================

// CleanupSessions nettoie les sessions expirées (super_admin)
func (h *SessionHandler) CleanupSessions(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req sessionusecase.CleanupSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body optionnel, utiliser valeurs par défaut
		req = sessionusecase.CleanupSessionsRequest{
			IncludeRevoked: false,
		}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("include_revoked", req.IncludeRevoked).
		Msg("🧹 Cleanup sessions request")

	response, err := h.cleanupUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleSessionError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

// RegisterRoutes enregistre toutes les routes de gestion des sessions
// Routes accessibles aux admins (super_admin + admin)
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

// extractAdminContext extrait le contexte admin depuis la requête
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

// extractIPWithoutPort extrait l'IP sans le port
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

// handleSessionError convertit les erreurs usecase en AppError HTTP
func handleSessionError(err error) error {
	errMsg := err.Error()

	switch {
	// Erreurs entité
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

	// Erreurs repository
	case errors.Is(err, repository.ErrSessionNotFound):
		return utils.NewAppError("SESSION_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrSessionAlreadyExists):
		return utils.NewAppError("SESSION_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrSessionInvalidData):
		return utils.NewAppError("SESSION_INVALID_DATA", errMsg, http.StatusBadRequest)
	case errors.Is(err, repository.ErrSessionTokenMismatch):
		return utils.NewAppError("SESSION_TOKEN_MISMATCH", errMsg, http.StatusUnauthorized)

	// Erreurs de permissions (strings)
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
