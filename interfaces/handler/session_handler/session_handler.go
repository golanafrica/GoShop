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

// @Summary Lister les sessions utilisateur
// @Description Récupère la liste paginée et filtrée des sessions actives ou passées d'un utilisateur.
// @Tags Session Management
// @Accept json
// @Produce json
// @Param user_id query string false "ID de l'utilisateur cible (admin seulement)"
// @Param active_only query boolean false "Ne retourner que les sessions actives (défaut: true)"
// @Param limit query int false "Nombre de résultats (défaut: 50, max: 100)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Success 200 {object} sessionusecase.ListSessionsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/sessions [get]
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

// @Summary Obtenir les statistiques des sessions
// @Description Retourne un résumé statistique des sessions actives, expirées et révoquées pour un utilisateur ou globalement.
// @Tags Session Management
// @Accept json
// @Produce json
// @Param user_id query string false "ID de l'utilisateur cible (admin seulement)"
// @Success 200 {object} sessionusecase.GetSessionStatsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/sessions/stats [get]
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

// @Summary Révoquer une session spécifique
// @Description Invalide une session active spécifique, forçant la déconnexion de l'appareil concerné.
// @Tags Session Management
// @Accept json
// @Produce json
// @Param request body sessionusecase.RevokeSessionRequest true "ID de la session cible et motif optionnel"
// @Success 200 {object} sessionusecase.RevokeSessionResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou ID manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC ou tentative de révocation de sa propre session)"
// @Failure 404 {object} utils.AppError "Session introuvable"
// @Failure 409 {object} utils.AppError "Session déjà révoquée"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/sessions/revoke [post]
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

// @Summary Révoquer toutes les sessions d'un utilisateur
// @Description Invalide toutes les sessions actives d'un utilisateur (sauf la session courante de l'admin), forçant une déconnexion globale.
// @Tags Session Management
// @Accept json
// @Produce json
// @Param request body sessionusecase.RevokeAllSessionsRequest false "ID de l'utilisateur cible et motif optionnel"
// @Success 200 {object} sessionusecase.RevokeAllSessionsResponse
// @Failure 400 {object} utils.AppError "Payload invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/sessions/revoke-all [post]
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

// @Summary Nettoyer les sessions expirées ou révoquées
// @Description Supprime physiquement les sessions expirées ou révoquées de la base de données pour libérer de l'espace (réservé aux super_admin).
// @Tags Session Management
// @Accept json
// @Produce json
// @Param request body sessionusecase.CleanupSessionsRequest false "Inclure les sessions révoquées dans le nettoyage (défaut: false)"
// @Success 200 {object} sessionusecase.CleanupSessionsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (super_admin requis)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/sessions/cleanup [post]
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
