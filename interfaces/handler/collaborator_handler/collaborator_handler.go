package collaboratorhandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Goshop/application/metrics"
	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"
	"Goshop/domain/entity"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : COLLABORATOR HANDLER
// ============================================================

// CollaboratorHandler gère les endpoints collaborateurs
type CollaboratorHandler struct {
	invitePlatformUC   *collaboratorusecase.InvitePlatformCollaboratorUsecase
	inviteShopUC       *collaboratorusecase.InviteShopCollaboratorUsecase
	acceptInvitationUC *collaboratorusecase.AcceptInvitationUsecase
	listCollabsUC      *collaboratorusecase.ListCollaboratorsUsecase
	updateRoleUC       *collaboratorusecase.UpdateCollaboratorRoleUsecase
	removeCollabUC     *collaboratorusecase.RemoveCollaboratorUsecase
}

// NewCollaboratorHandler crée une nouvelle instance
func NewCollaboratorHandler(
	invitePlatformUC *collaboratorusecase.InvitePlatformCollaboratorUsecase,
	inviteShopUC *collaboratorusecase.InviteShopCollaboratorUsecase,
	acceptInvitationUC *collaboratorusecase.AcceptInvitationUsecase,
	listCollabsUC *collaboratorusecase.ListCollaboratorsUsecase,
	updateRoleUC *collaboratorusecase.UpdateCollaboratorRoleUsecase,
	removeCollabUC *collaboratorusecase.RemoveCollaboratorUsecase,
) *CollaboratorHandler {
	return &CollaboratorHandler{
		invitePlatformUC:   invitePlatformUC,
		inviteShopUC:       inviteShopUC,
		acceptInvitationUC: acceptInvitationUC,
		listCollabsUC:      listCollabsUC,
		updateRoleUC:       updateRoleUC,
		removeCollabUC:     removeCollabUC,
	}
}

// ============================================================
// ADMIN PLATFORM ENDPOINTS
// ============================================================

// @Summary Inviter un collaborateur plateforme
func (h *CollaboratorHandler) InvitePlatformCollaborator(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req collaboratorusecase.InvitePlatformCollaboratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("email", req.Email).
		Str("role", string(req.Role)).
		Msg("📧 Invitation collaborateur plateforme")

	response, err := h.invitePlatformUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec invitation
		metrics.CollaboratorInvitationTotal.WithLabelValues("platform", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("invite_platform").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_invite_platform", "collaborator_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Failed to invite platform collaborator")

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès invitation
	metrics.CollaboratorInvitationTotal.WithLabelValues("platform", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("invite_platform").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("email", req.Email).
		Float64("duration_seconds", duration).
		Msg("✅ Platform collaborator invited successfully")

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Lister les collaborateurs plateforme
func (h *CollaboratorHandler) ListPlatformCollaborators(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:      "platform",
		Role:      r.URL.Query().Get("role"),
		Search:    r.URL.Query().Get("search"),
		Limit:     parseIntParam(r, "limit", 20),
		Offset:    parseIntParam(r, "offset", 0),
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}

	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		b := isActive == "true"
		req.IsActive = &b
	}

	response, err := h.listCollabsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec listing
		metrics.CollaboratorListTotal.WithLabelValues("platform", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("list_platform").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_list_platform", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès listing
	metrics.CollaboratorListTotal.WithLabelValues("platform", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("list_platform").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Modifier le rôle d'un collaborateur plateforme
func (h *CollaboratorHandler) UpdatePlatformCollaboratorRole(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	collabID := chi.URLParam(r, "id")
	if collabID == "" {
		return utils.NewAppError("MISSING_ID", "collaborator_id is required in URL", http.StatusBadRequest)
	}

	var body struct {
		NewRole string `json:"new_role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID,
		Type:           "platform",
		NewRole:        body.NewRole,
	}

	response, err := h.updateRoleUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec update
		metrics.CollaboratorRoleUpdateTotal.WithLabelValues("platform", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("update_role_platform").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_update_role_platform", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès update
	metrics.CollaboratorRoleUpdateTotal.WithLabelValues("platform", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("update_role_platform").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Supprimer un collaborateur plateforme
func (h *CollaboratorHandler) RemovePlatformCollaborator(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	collabID := chi.URLParam(r, "id")
	if collabID == "" {
		return utils.NewAppError("MISSING_ID", "collaborator_id is required in URL", http.StatusBadRequest)
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID,
		Type:           "platform",
		Reason:         body.Reason,
	}

	response, err := h.removeCollabUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec removal
		metrics.CollaboratorRemovalTotal.WithLabelValues("platform", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("remove_platform").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_remove_platform", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès removal
	metrics.CollaboratorRemovalTotal.WithLabelValues("platform", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("remove_platform").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// SHOP ENDPOINTS
// ============================================================

// @Summary Inviter un collaborateur boutique
func (h *CollaboratorHandler) InviteShopCollaborator(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	shopID := chi.URLParam(r, "shop_id")
	if shopID == "" {
		return utils.NewAppError("MISSING_SHOP_ID", "shop_id is required in URL", http.StatusBadRequest)
	}

	var req collaboratorusecase.InviteShopCollaboratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	req.ShopID = shopID

	response, err := h.inviteShopUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec invitation
		metrics.CollaboratorInvitationTotal.WithLabelValues("shop", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("invite_shop").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_invite_shop", "collaborator_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("Failed to invite shop collaborator")

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès invitation
	metrics.CollaboratorInvitationTotal.WithLabelValues("shop", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("invite_shop").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Str("email", req.Email).
		Float64("duration_seconds", duration).
		Msg("✅ Shop collaborator invited successfully")

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Lister les collaborateurs d'une boutique
func (h *CollaboratorHandler) ListShopCollaborators(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	shopID := chi.URLParam(r, "shop_id")
	if shopID == "" {
		return utils.NewAppError("MISSING_SHOP_ID", "shop_id is required in URL", http.StatusBadRequest)
	}

	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:      "shop",
		ShopID:    shopID,
		Role:      r.URL.Query().Get("role"),
		Search:    r.URL.Query().Get("search"),
		Limit:     parseIntParam(r, "limit", 20),
		Offset:    parseIntParam(r, "offset", 0),
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}

	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		b := isActive == "true"
		req.IsActive = &b
	}

	response, err := h.listCollabsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec listing
		metrics.CollaboratorListTotal.WithLabelValues("shop", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("list_shop").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_list_shop", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès listing
	metrics.CollaboratorListTotal.WithLabelValues("shop", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("list_shop").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Modifier le rôle d'un collaborateur boutique
func (h *CollaboratorHandler) UpdateShopCollaboratorRole(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	shopID := chi.URLParam(r, "shop_id")
	collabID := chi.URLParam(r, "id")
	if shopID == "" || collabID == "" {
		return utils.NewAppError("MISSING_ID", "shop_id and collaborator_id are required in URL", http.StatusBadRequest)
	}

	var body struct {
		NewRole string `json:"new_role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID,
		Type:           "shop",
		ShopID:         shopID,
		NewRole:        body.NewRole,
	}

	response, err := h.updateRoleUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec update
		metrics.CollaboratorRoleUpdateTotal.WithLabelValues("shop", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("update_role_shop").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_update_role_shop", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès update
	metrics.CollaboratorRoleUpdateTotal.WithLabelValues("shop", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("update_role_shop").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Supprimer un collaborateur boutique
func (h *CollaboratorHandler) RemoveShopCollaborator(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	shopID := chi.URLParam(r, "shop_id")
	collabID := chi.URLParam(r, "id")
	if shopID == "" || collabID == "" {
		return utils.NewAppError("MISSING_ID", "shop_id and collaborator_id are required in URL", http.StatusBadRequest)
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID,
		Type:           "shop",
		ShopID:         shopID,
		Reason:         body.Reason,
	}

	response, err := h.removeCollabUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec removal
		metrics.CollaboratorRemovalTotal.WithLabelValues("shop", "error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("remove_shop").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_remove_shop", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès removal
	metrics.CollaboratorRemovalTotal.WithLabelValues("shop", "success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("remove_shop").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// PUBLIC ENDPOINTS (rate limited)
// ============================================================

// @Summary Aperçu d'une invitation collaborateur
func (h *CollaboratorHandler) PreviewInvitation(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	token := chi.URLParam(r, "token")
	if token == "" {
		return utils.NewAppError("MISSING_TOKEN", "token is required in URL", http.StatusBadRequest)
	}

	// Normaliser le token
	token = collaboratorusecase.NormalizeToken(token)

	preview, err := h.acceptInvitationUC.GetInvitationPreview(ctx, token)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec preview
		metrics.CollaboratorInvitationPreviewTotal.WithLabelValues("error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("preview_invitation").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_preview_invitation", "collaborator_handler").Inc()

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès preview
	metrics.CollaboratorInvitationPreviewTotal.WithLabelValues("success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("preview_invitation").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, preview)
	return nil
}

// @Summary Accepter une invitation collaborateur
func (h *CollaboratorHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	token := chi.URLParam(r, "token")
	if token == "" {
		return utils.NewAppError("MISSING_TOKEN", "token is required in URL", http.StatusBadRequest)
	}

	// Normaliser le token
	token = collaboratorusecase.NormalizeToken(token)

	logger.Info().
		Str("token_preview", token[:8]+"...").
		Str("remote_ip", r.RemoteAddr).
		Msg("🎫 Tentative d'acceptation d'invitation")

	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	response, err := h.acceptInvitationUC.Execute(ctx, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec accept
		metrics.CollaboratorInvitationAcceptTotal.WithLabelValues("error").Inc()
		metrics.CollaboratorOperationDuration.WithLabelValues("accept_invitation").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("collaborator_accept_invitation", "collaborator_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Failed to accept invitation")

		return handleCollaboratorError(err)
	}

	// 📊 MÉTRIQUES : Succès accept
	metrics.CollaboratorInvitationAcceptTotal.WithLabelValues("success").Inc()
	metrics.CollaboratorOperationDuration.WithLabelValues("accept_invitation").Observe(duration)

	logger.Info().
		Float64("duration_seconds", duration).
		Msg("✅ Invitation accepted successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// HELPERS
// ============================================================

// extractAdminContext extrait AdminContext depuis le contexte HTTP
func extractAdminContext(r *http.Request) (*collaboratorusecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())

	return &collaboratorusecase.AdminContext{
		AdminID:    adminID,
		AdminEmail: "", // Non disponible dans le contexte JWT
		AdminRole:  adminRole,
		IPAddress:  parseIP(r.RemoteAddr),
		UserAgent:  r.UserAgent(),
		RequestID:  r.Header.Get("X-Request-ID"),
	}, nil
}

// parseIP extrait l'IP depuis RemoteAddr (enlève le port)
func parseIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	// Cas IPv6 avec crochets : [::1]:8080
	if strings.HasPrefix(remoteAddr, "[") {
		endBracket := strings.Index(remoteAddr, "]")
		if endBracket > 0 {
			return remoteAddr[1:endBracket]
		}
	}

	// Cas IPv4 avec port : 192.168.1.1:8080
	lastColon := strings.LastIndex(remoteAddr, ":")
	if lastColon > 0 {
		if strings.Count(remoteAddr, ":") == 1 {
			return remoteAddr[:lastColon]
		}
	}

	return remoteAddr
}

// handleCollaboratorError convertit les erreurs usecase en AppError HTTP
func handleCollaboratorError(err error) error {
	errMsg := err.Error()

	switch {
	case errors.Is(err, entity.ErrInvalidPlatformRole):
		return utils.NewAppError("INVALID_PLATFORM_ROLE", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrInvalidShopRole):
		return utils.NewAppError("INVALID_SHOP_ROLE", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrInvitationExpired):
		return utils.NewAppError("INVITATION_EXPIRED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrInvitationAlreadyUsed):
		return utils.NewAppError("INVITATION_ALREADY_USED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrInvitationCancelled):
		return utils.NewAppError("INVITATION_CANCELLED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrUserAlreadyCollaborator):
		return utils.NewAppError("USER_ALREADY_COLLABORATOR", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrCollaboratorNotFound):
		return utils.NewAppError("COLLABORATOR_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, entity.ErrInvitationNotFound):
		return utils.NewAppError("INVITATION_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, entity.ErrDeletionReasonRequired):
		return utils.NewAppError("DELETION_REASON_REQUIRED", errMsg, http.StatusBadRequest)

	case errMsg == "shop not found":
		return utils.NewAppError("SHOP_NOT_FOUND", errMsg, http.StatusNotFound)
	case errMsg == "shop is not active":
		return utils.NewAppError("SHOP_NOT_ACTIVE", errMsg, http.StatusBadRequest)
	case errMsg == "invalid shop_id format":
		return utils.NewAppError("INVALID_SHOP_ID", errMsg, http.StatusBadRequest)
	case errMsg == "invalid collaborator_id format":
		return utils.NewAppError("INVALID_COLLABORATOR_ID", errMsg, http.StatusBadRequest)
	case errMsg == "cannot change your own role":
		return utils.NewAppError("CANNOT_CHANGE_OWN_ROLE", errMsg, http.StatusForbidden)
	case errMsg == "cannot delete yourself":
		return utils.NewAppError("CANNOT_DELETE_YOURSELF", errMsg, http.StatusForbidden)
	case strings.HasPrefix(errMsg, "user with email"):
		return utils.NewAppError("USER_NOT_FOUND", errMsg, http.StatusNotFound)
	case strings.HasPrefix(errMsg, "pending invitation already exists"):
		return utils.NewAppError("PENDING_INVITATION_EXISTS", errMsg, http.StatusConflict)
	case strings.HasPrefix(errMsg, "insufficient permissions"):
		return utils.NewAppError("FORBIDDEN", errMsg, http.StatusForbidden)
	case strings.HasPrefix(errMsg, "reason must be at least"):
		return utils.NewAppError("REASON_TOO_SHORT", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "reason must be at most"):
		return utils.NewAppError("REASON_TOO_LONG", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "invalid token format"):
		return utils.NewAppError("INVALID_TOKEN", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "collaborator is already deleted"):
		return utils.NewAppError("COLLABORATOR_ALREADY_DELETED", errMsg, http.StatusConflict)
	case strings.HasPrefix(errMsg, "collaborator does not belong to this shop"):
		return utils.NewAppError("COLLABORATOR_WRONG_SHOP", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "cannot delete the last shop_admin"):
		return utils.NewAppError("LAST_SHOP_ADMIN", errMsg, http.StatusBadRequest)

	default:
		return err
	}
}

// parseIntParam parse un paramètre entier avec valeur par défaut
func parseIntParam(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	var i int
	_, err := fmt.Sscanf(value, "%d", &i)
	if err != nil {
		return defaultValue
	}

	return i
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

func (h *CollaboratorHandler) RegisterAdminPlatformRoutes(r chi.Router) {
	r.Post("/invite", middl.ErrorHandler(h.InvitePlatformCollaborator))
	r.Get("/", middl.ErrorHandler(h.ListPlatformCollaborators))
	r.Put("/{id}/role", middl.ErrorHandler(h.UpdatePlatformCollaboratorRole))
	r.Delete("/{id}", middl.ErrorHandler(h.RemovePlatformCollaborator))
}

func (h *CollaboratorHandler) RegisterShopRoutes(r chi.Router) {
	r.Post("/invite", middl.ErrorHandler(h.InviteShopCollaborator))
	r.Get("/", middl.ErrorHandler(h.ListShopCollaborators))
	r.Put("/{id}/role", middl.ErrorHandler(h.UpdateShopCollaboratorRole))
	r.Delete("/{id}", middl.ErrorHandler(h.RemoveShopCollaborator))
}

func (h *CollaboratorHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/{token}/preview", middl.ErrorHandler(h.PreviewInvitation))
	r.Post("/{token}", middl.ErrorHandler(h.AcceptInvitation))
}
