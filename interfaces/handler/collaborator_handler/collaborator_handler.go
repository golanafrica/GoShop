package collaboratorhandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
//
// 🎯 Objectif :
//   Exposer les endpoints HTTP pour la gestion des collaborateurs
//   (plateforme et boutique) avec invitation par email.
//
// 📋 Endpoints :
//
//   ADMIN PLATFORM (super_admin + admin) :
//   - POST   /api/admin/collaborators/platform/invite
//   - GET    /api/admin/collaborators/platform
//   - PUT    /api/admin/collaborators/platform/{id}/role
//   - DELETE /api/admin/collaborators/platform/{id}
//
//   SHOP (shop_admin / merchant) :
//   - POST   /api/shops/{shop_id}/collaborators/invite
//   - GET    /api/shops/{shop_id}/collaborators
//   - PUT    /api/shops/{shop_id}/collaborators/{id}/role
//   - DELETE /api/shops/{shop_id}/collaborators/{id}
//
//   PUBLIC (rate limited) :
//   - GET    /api/collaborators/invitations/{token}/preview
//   - POST   /api/collaborators/invitations/{token}
//
// 🔐 Sécurité :
//   - RBAC via RequireRoles()
//   - Rate limiting sur endpoints publics
//   - AdminContext extrait automatiquement du JWT
//
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
// @Description Envoie une invitation par email pour ajouter un collaborateur au niveau de la plateforme.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param request body collaboratorusecase.InvitePlatformCollaboratorRequest true "Détails de l'invitation (email, role)"
// @Success 201 {object} collaboratorusecase.InvitePlatformCollaboratorResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou email déjà invité"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC: super_admin, admin requis)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/collaborators/platform/invite [post]
func (h *CollaboratorHandler) InvitePlatformCollaborator(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.invitePlatformUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Lister les collaborateurs plateforme
// @Description Récupère la liste paginée et filtrée des collaborateurs de la plateforme.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param role query string false "Filtrer par rôle"
// @Param search query string false "Recherche par email ou nom"
// @Param is_active query boolean false "Filtrer par statut actif (true/false)"
// @Param limit query int false "Nombre de résultats (défaut: 20)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Param sort_by query string false "Colonne de tri"
// @Param sort_order query string false "Ordre de tri (ASC/DESC)"
// @Success 200 {object} collaboratorusecase.ListCollaboratorsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/collaborators/platform [get]
func (h *CollaboratorHandler) ListPlatformCollaborators(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.listCollabsUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Modifier le rôle d'un collaborateur plateforme
// @Description Met à jour le rôle d'un collaborateur existant au niveau de la plateforme.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param id path string true "ID du collaborateur (UUID)"
// @Param request body object true "Nouveau rôle" example({"new_role": "admin"})
// @Success 200 {object} collaboratorusecase.UpdateCollaboratorRoleResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou rôle invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (ex: tentative de modifier son propre rôle)"
// @Failure 404 {object} utils.AppError "Collaborateur introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/collaborators/platform/{id}/role [put]
func (h *CollaboratorHandler) UpdatePlatformCollaboratorRole(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.updateRoleUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Supprimer un collaborateur plateforme
// @Description Révoque l'accès d'un collaborateur au niveau de la plateforme.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param id path string true "ID du collaborateur (UUID)"
// @Param request body object true "Motif de la suppression" example({"reason": "Fin de contrat"})
// @Success 200 {object} collaboratorusecase.RemoveCollaboratorResponse
// @Failure 400 {object} utils.AppError "Motif requis ou payload invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (ex: tentative de se supprimer soi-même)"
// @Failure 404 {object} utils.AppError "Collaborateur introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/collaborators/platform/{id} [delete]
func (h *CollaboratorHandler) RemovePlatformCollaborator(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.removeCollabUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// SHOP ENDPOINTS
// ============================================================

// @Summary Inviter un collaborateur boutique
// @Description Envoie une invitation par email pour ajouter un collaborateur à une boutique spécifique.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param shop_id path string true "ID de la boutique (UUID)"
// @Param request body collaboratorusecase.InviteShopCollaboratorRequest true "Détails de l'invitation (email, role)"
// @Success 201 {object} collaboratorusecase.InviteShopCollaboratorResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou email déjà invité"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC: merchant, shop_admin requis)"
// @Failure 404 {object} utils.AppError "Boutique introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{shop_id}/collaborators/invite [post]
func (h *CollaboratorHandler) InviteShopCollaborator(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.inviteShopUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Lister les collaborateurs d'une boutique
// @Description Récupère la liste paginée et filtrée des collaborateurs d'une boutique spécifique.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param shop_id path string true "ID de la boutique (UUID)"
// @Param role query string false "Filtrer par rôle"
// @Param search query string false "Recherche par email ou nom"
// @Param is_active query boolean false "Filtrer par statut actif (true/false)"
// @Param limit query int false "Nombre de résultats (défaut: 20)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Param sort_by query string false "Colonne de tri"
// @Param sort_order query string false "Ordre de tri (ASC/DESC)"
// @Success 200 {object} collaboratorusecase.ListCollaboratorsResponse
// @Failure 400 {object} utils.AppError "ID de boutique manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (RBAC)"
// @Failure 404 {object} utils.AppError "Boutique introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{shop_id}/collaborators [get]
func (h *CollaboratorHandler) ListShopCollaborators(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.listCollabsUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Modifier le rôle d'un collaborateur boutique
// @Description Met à jour le rôle d'un collaborateur existant dans une boutique spécifique.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param shop_id path string true "ID de la boutique (UUID)"
// @Param id path string true "ID du collaborateur (UUID)"
// @Param request body object true "Nouveau rôle" example({"new_role": "manager"})
// @Success 200 {object} collaboratorusecase.UpdateCollaboratorRoleResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou IDs manquants"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (ex: tentative de modifier son propre rôle)"
// @Failure 404 {object} utils.AppError "Collaborateur ou boutique introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{shop_id}/collaborators/{id}/role [put]
func (h *CollaboratorHandler) UpdateShopCollaboratorRole(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.updateRoleUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Supprimer un collaborateur boutique
// @Description Révoque l'accès d'un collaborateur à une boutique spécifique.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param shop_id path string true "ID de la boutique (UUID)"
// @Param id path string true "ID du collaborateur (UUID)"
// @Param request body object true "Motif de la suppression" example({"reason": "Fin de contrat"})
// @Success 200 {object} collaboratorusecase.RemoveCollaboratorResponse
// @Failure 400 {object} utils.AppError "Motif requis, payload invalide ou dernier admin"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (ex: tentative de se supprimer soi-même)"
// @Failure 404 {object} utils.AppError "Collaborateur ou boutique introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{shop_id}/collaborators/{id} [delete]
func (h *CollaboratorHandler) RemoveShopCollaborator(w http.ResponseWriter, r *http.Request) error {
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

	response, err := h.removeCollabUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// PUBLIC ENDPOINTS (rate limited)
// ============================================================

// @Summary Aperçu d'une invitation collaborateur
// @Description Permet de voir les détails d'une invitation (email, rôle, entité) avant de l'accepter, sans être authentifié.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param token path string true "Token d'invitation unique"
// @Success 200 {object} collaboratorusecase.InvitationPreview
// @Failure 400 {object} utils.AppError "Token manquant ou format invalide"
// @Failure 404 {object} utils.AppError "Invitation introuvable ou expirée"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Router /api/collaborators/invitations/{token}/preview [get]
func (h *CollaboratorHandler) PreviewInvitation(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "token")
	if token == "" {
		return utils.NewAppError("MISSING_TOKEN", "token is required in URL", http.StatusBadRequest)
	}

	// Normaliser le token
	token = collaboratorusecase.NormalizeToken(token)

	preview, err := h.acceptInvitationUC.GetInvitationPreview(r.Context(), token)
	if err != nil {
		return handleCollaboratorError(err)
	}

	utils.WriteJSON(w, http.StatusOK, preview)
	return nil
}

// @Summary Accepter une invitation collaborateur
// @Description Permet à un utilisateur de rejoindre la plateforme ou une boutique en utilisant un token d'invitation valide.
// @Tags Collaborator Management
// @Accept json
// @Produce json
// @Param token path string true "Token d'invitation unique"
// @Success 200 {object} collaboratorusecase.AcceptInvitationResponse
// @Failure 400 {object} utils.AppError "Token manquant, expiré ou déjà utilisé"
// @Failure 404 {object} utils.AppError "Invitation introuvable"
// @Failure 409 {object} utils.AppError "L'utilisateur est déjà collaborateur"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Router /api/collaborators/invitations/{token} [post]
func (h *CollaboratorHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.acceptInvitationUC.Execute(r.Context(), req)
	if err != nil {
		return handleCollaboratorError(err)
	}

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
	// Erreurs typées (entity)
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

	// Erreurs string
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

// RegisterAdminPlatformRoutes enregistre les routes admin plateforme
// À appeler dans app.go avec RequireRoles("super_admin", "admin")
func (h *CollaboratorHandler) RegisterAdminPlatformRoutes(r chi.Router) {
	r.Post("/invite", middl.ErrorHandler(h.InvitePlatformCollaborator))
	r.Get("/", middl.ErrorHandler(h.ListPlatformCollaborators))
	r.Put("/{id}/role", middl.ErrorHandler(h.UpdatePlatformCollaboratorRole))
	r.Delete("/{id}", middl.ErrorHandler(h.RemovePlatformCollaborator))
}

// RegisterShopRoutes enregistre les routes shop collaborateurs
// À appeler dans app.go avec RequireRoles("merchant", "super_admin")
func (h *CollaboratorHandler) RegisterShopRoutes(r chi.Router) {
	r.Post("/invite", middl.ErrorHandler(h.InviteShopCollaborator))
	r.Get("/", middl.ErrorHandler(h.ListShopCollaborators))
	r.Put("/{id}/role", middl.ErrorHandler(h.UpdateShopCollaboratorRole))
	r.Delete("/{id}", middl.ErrorHandler(h.RemoveShopCollaborator))
}

// RegisterPublicRoutes enregistre les routes publiques (rate limited)
// À appeler dans app.go SANS AuthMiddleware mais AVEC RateLimiter
func (h *CollaboratorHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/{token}/preview", middl.ErrorHandler(h.PreviewInvitation))
	r.Post("/{token}", middl.ErrorHandler(h.AcceptInvitation))
}
