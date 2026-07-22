package apikeyhandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	apikeyusecase "Goshop/application/usecase/apikey_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.3 : API KEY MANAGEMENT HANDLER
// ============================================================
//
// 🎯 Objectif :
//   Exposer les endpoints HTTP pour la gestion des clés API.
//
// 📋 Endpoints :
//   - POST   /api/admin/api-keys           - Créer une clé
//   - GET    /api/admin/api-keys           - Lister les clés
//   - POST   /api/admin/api-keys/revoke    - Révoquer une clé
//   - POST   /api/admin/api-keys/revoke-all - Révoquer toutes
//   - GET    /api/admin/api-keys/stats     - Statistiques
//
// 🔐 Sécurité :
//   - Tous les endpoints nécessitent une authentification admin
//   - RBAC : super_admin / admin
//   - La clé complète est affichée UNE SEULE FOIS à la création
//
// ============================================================

// APIKeyHandler gère les endpoints de gestion des clés API
type APIKeyHandler struct {
	createUC    *apikeyusecase.CreateAPIKeyUsecase
	listUC      *apikeyusecase.ListAPIKeysUsecase
	revokeUC    *apikeyusecase.RevokeAPIKeyUsecase
	revokeAllUC *apikeyusecase.RevokeAllAPIKeysUsecase
	getStatsUC  *apikeyusecase.GetAPIKeyStatsUsecase
}

// NewAPIKeyHandler crée une nouvelle instance
func NewAPIKeyHandler(
	createUC *apikeyusecase.CreateAPIKeyUsecase,
	listUC *apikeyusecase.ListAPIKeysUsecase,
	revokeUC *apikeyusecase.RevokeAPIKeyUsecase,
	revokeAllUC *apikeyusecase.RevokeAllAPIKeysUsecase,
	getStatsUC *apikeyusecase.GetAPIKeyStatsUsecase,
) *APIKeyHandler {
	return &APIKeyHandler{
		createUC:    createUC,
		listUC:      listUC,
		revokeUC:    revokeUC,
		revokeAllUC: revokeAllUC,
		getStatsUC:  getStatsUC,
	}
}

// ============================================================
// ENDPOINT 1 : CREATE API KEY
// ============================================================

// @Summary Créer une nouvelle clé API
// @Description Génère une nouvelle clé API pour un utilisateur avec des scopes spécifiques. La clé complète n'est affichée qu'une seule fois à la création.
// @Tags API Key Management
// @Accept json
// @Produce json
// @Param request body apikeyusecase.CreateAPIKeyRequest true "Détails de la clé API (name, scopes, is_test)"
// @Success 201 {object} apikeyusecase.CreateAPIKeyResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 429 {object} utils.AppError "Limite de clés atteinte"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/api-keys [post]
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	// Validation du nom
	if req.Name == "" {
		return utils.NewAppError("MISSING_FIELD", "name is required", http.StatusBadRequest)
	}

	// Validation des scopes
	if len(req.Scopes) == 0 {
		return utils.NewAppError("MISSING_FIELD", "at least one scope is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("key_name", req.Name).
		Int("scopes_count", len(req.Scopes)).
		Bool("is_test", req.IsTest).
		Msg("🔑 Create API key request")

	response, err := h.createUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleAPIKeyError(err)
	}

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// ============================================================
// ENDPOINT 2 : LIST API KEYS
// ============================================================

// @Summary Lister les clés API
// @Description Récupère la liste paginée des clés API d'un utilisateur (ou de tous les utilisateurs pour les admins).
// @Tags API Key Management
// @Accept json
// @Produce json
// @Param user_id query string false "ID de l'utilisateur cible (admin seulement)"
// @Param active_only query boolean false "Ne retourner que les clés actives (défaut: true)"
// @Param limit query int false "Nombre de résultats (défaut: 50, max: 100)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Success 200 {object} apikeyusecase.ListAPIKeysResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/api-keys [get]
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) error {
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

	req := &apikeyusecase.ListAPIKeysRequest{
		UserID:     userID,
		ActiveOnly: activeOnly,
		Limit:      limit,
		Offset:     offset,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Bool("active_only", activeOnly).
		Msg("📋 List API keys request")

	response, err := h.listUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleAPIKeyError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 3 : REVOKE API KEY
// ============================================================

// @Summary Révoquer une clé API
// @Description Révoque une clé API spécifique, l'empêchant d'être utilisée pour de futures requêtes.
// @Tags API Key Management
// @Accept json
// @Produce json
// @Param request body apikeyusecase.RevokeAPIKeyRequest true "ID de la clé et motif de révocation"
// @Success 200 {object} apikeyusecase.RevokeAPIKeyResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou api_key_id manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 404 {object} utils.AppError "Clé API introuvable"
// @Failure 409 {object} utils.AppError "Clé déjà révoquée"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/api-keys/revoke [post]
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.RevokeAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	// Validation
	if req.APIKeyID == "" {
		return utils.NewAppError("MISSING_FIELD", "api_key_id is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("api_key_id", req.APIKeyID).
		Str("reason", req.Reason).
		Msg("🔑 Revoke API key request")

	response, err := h.revokeUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleAPIKeyError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 4 : REVOKE ALL API KEYS
// ============================================================

// @Summary Révoquer toutes les clés API d'un utilisateur
// @Description Révoque toutes les clés API actives associées à un utilisateur spécifique.
// @Tags API Key Management
// @Accept json
// @Produce json
// @Param request body apikeyusecase.RevokeAllAPIKeysRequest false "ID de l'utilisateur et motif de révocation"
// @Success 200 {object} apikeyusecase.RevokeAllAPIKeysResponse
// @Failure 400 {object} utils.AppError "Payload invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/api-keys/revoke-all [post]
func (h *APIKeyHandler) RevokeAllAPIKeys(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.RevokeAllAPIKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body optionnel, continuer avec valeurs par défaut
		req = apikeyusecase.RevokeAllAPIKeysRequest{}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Str("reason", req.Reason).
		Msg("🔑 Revoke all API keys request")

	response, err := h.revokeAllUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleAPIKeyError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 5 : GET STATISTICS
// ============================================================

// @Summary Obtenir les statistiques des clés API
// @Description Récupère les statistiques d'utilisation et l'état des clés API pour un utilisateur ou globalement.
// @Tags API Key Management
// @Accept json
// @Produce json
// @Param user_id query string false "ID de l'utilisateur cible (admin seulement)"
// @Success 200 {object} apikeyusecase.GetAPIKeyStatsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/api-keys/stats [get]
func (h *APIKeyHandler) GetStatistics(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	userID := r.URL.Query().Get("user_id")

	req := &apikeyusecase.GetAPIKeyStatsRequest{
		UserID: userID,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Msg("📊 Get API key statistics request")

	response, err := h.getStatsUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleAPIKeyError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

// RegisterRoutes enregistre toutes les routes de gestion des clés API
// Routes accessibles aux admins (super_admin + admin)
func (h *APIKeyHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", middl.ErrorHandler(h.CreateAPIKey))
	r.Get("/", middl.ErrorHandler(h.ListAPIKeys))
	r.Post("/revoke", middl.ErrorHandler(h.RevokeAPIKey))
	r.Post("/revoke-all", middl.ErrorHandler(h.RevokeAllAPIKeys))
	r.Get("/stats", middl.ErrorHandler(h.GetStatistics))
}

// ============================================================
// HELPERS
// ============================================================

// extractAdminContext extrait le contexte admin depuis la requête
func extractAdminContext(r *http.Request) (*apikeyusecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())
	adminEmail, _ := utils.UserEmailFromContext(r.Context())

	return &apikeyusecase.AdminContext{
		AdminID:    adminID,
		AdminEmail: adminEmail,
		AdminRole:  adminRole,
		IPAddress:  extractIPWithoutPort(r.RemoteAddr),
		UserAgent:  r.UserAgent(),
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

// handleAPIKeyError convertit les erreurs usecase en AppError HTTP
func handleAPIKeyError(err error) error {
	errMsg := err.Error()

	switch {
	// Erreurs entité
	case errors.Is(err, entity.ErrAPIKeyNotFound):
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, entity.ErrAPIKeyExpired):
		return utils.NewAppError("API_KEY_EXPIRED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrAPIKeyRevoked):
		return utils.NewAppError("API_KEY_REVOKED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrAPIKeyInvalid):
		return utils.NewAppError("API_KEY_INVALID", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrAPIKeyInsufficientScope):
		return utils.NewAppError("API_KEY_INSUFFICIENT_SCOPE", errMsg, http.StatusForbidden)
	case errors.Is(err, entity.ErrAPIKeyNameRequired):
		return utils.NewAppError("API_KEY_NAME_REQUIRED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrAPIKeyAlreadyRevoked):
		return utils.NewAppError("API_KEY_ALREADY_REVOKED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrAPIKeyInvalidPrefix):
		return utils.NewAppError("API_KEY_INVALID_PREFIX", errMsg, http.StatusBadRequest)

	// Erreurs repository
	case errors.Is(err, repository.ErrAPIKeyNotFound):
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrAPIKeyAlreadyExists):
		return utils.NewAppError("API_KEY_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrAPIKeyInvalidData):
		return utils.NewAppError("API_KEY_INVALID_DATA", errMsg, http.StatusBadRequest)

	// Erreurs de permissions (strings)
	case errMsg == "insufficient permissions: can only create own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: can only view own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: can only revoke own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: admin role required":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "API key is already revoked":
		return utils.NewAppError("API_KEY_ALREADY_REVOKED", errMsg, http.StatusConflict)
	case errMsg == "API key not found":
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errMsg == "name is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "at least one scope is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "api_key_id is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "maximum API keys limit reached (10 active keys per user)":
		return utils.NewAppError("API_KEY_LIMIT_REACHED", errMsg, http.StatusTooManyRequests)

	default:
		// Vérifier si c'est une erreur de scope invalide
		if strings.HasPrefix(errMsg, "invalid scope:") {
			return utils.NewAppError("INVALID_SCOPE", errMsg, http.StatusBadRequest)
		}
		return err
	}
}

// strings est importé via le package standard
var _ = strings.HasPrefix
