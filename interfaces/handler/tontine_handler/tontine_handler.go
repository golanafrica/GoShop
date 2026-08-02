package tontinehandler

import (
	"encoding/json"
	"net/http"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/repository"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// TontineHandler gère les routes de la tontine
type TontineHandler struct {
	createGroupUC          *tontineusecase.CreateTontineGroupUsecase
	joinGroupUC            *tontineusecase.JoinTontineGroupUsecase
	payCycleUC             *tontineusecase.PayCycleUsecase
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase
	customerRepo           repository.CustomerRepositoryInterface
}

// NewTontineHandler crée une nouvelle instance
func NewTontineHandler(
	createGroupUC *tontineusecase.CreateTontineGroupUsecase,
	joinGroupUC *tontineusecase.JoinTontineGroupUsecase,
	payCycleUC *tontineusecase.PayCycleUsecase,
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase,
	customerRepo repository.CustomerRepositoryInterface,
) *TontineHandler {
	return &TontineHandler{
		createGroupUC:          createGroupUC,
		joinGroupUC:            joinGroupUC,
		payCycleUC:             payCycleUC,
		listCustomerPaymentsUC: listCustomerPaymentsUC,
		customerRepo:           customerRepo,
	}
}

// @Summary Créer un groupe de tontine
// @Description Initialise un nouveau groupe de tontine pour un produit spécifique.
// L'ID client créateur est résolu de manière sécurisée via le JWT (prévention IDOR).
// @Tags Tontine
// @Accept json
// @Produce json
// @Param request body tontineusecase.CreateGroupRequest true "Détails du groupe à créer"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou règles de tontine non respectées"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (KYC non validé ou droits insuffisants)"
// @Failure 404 {object} utils.AppError "Profil client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups [post]
func (h *TontineHandler) CreateGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer l'ID utilisateur du JWT
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	// 2. 🛡️ TRADUCTION SÉCURISÉE User → Customer
	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found for create group")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. 🛡️ SÉCURITÉ : Forcer le créateur = client authentifié
	req.CreatorCustomerID = customer.ID

	group, err := h.createGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("CREATE_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, group)
	return nil
}

// @Summary Rejoindre un groupe de tontine
// @Description Permet à un client de rejoindre un groupe via un code d'invitation. L'ID client est résolu de manière sécurisée via le JWT pour prévenir les failles IDOR.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param request body tontineusecase.JoinGroupRequest true "Code d'invitation (le customer_id du body sera ignoré et remplacé par celui du token)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou code incorrect"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Profil client ou groupe introuvable"
// @Failure 409 {object} utils.AppError "Groupe déjà complet ou client déjà membre"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/join [post]
func (h *TontineHandler) JoinGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer l'ID utilisateur du JWT
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	// 2. 🛡️ TRADUCTION SÉCURISÉE
	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.JoinGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. 🛡️ SÉCURITÉ : Écraser le customer_id fourni par le client avec celui vérifié en base
	req.CustomerID = customer.ID

	response, err := h.joinGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("JOIN_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Payer une cotisation de tontine
// @Description Lance le processus de paiement pour la cotisation d'un cycle spécifique. L'ID client est résolu de manière sécurisée via le JWT.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param group_id path string true "ID du groupe de tontine (UUID)"
// @Param request body tontineusecase.PayCycleRequest true "Détails du paiement (le customer_id du body sera ignoré et remplacé par celui du token)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou cycle non éligible"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Profil client ou groupe introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/{group_id}/pay [post]
func (h *TontineHandler) PayCycle(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		return utils.ErrInvalidPayload
	}

	// 1. Récupérer l'ID utilisateur du JWT
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	// 2. 🛡️ TRADUCTION SÉCURISÉE
	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.PayCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. 🛡️ SÉCURITÉ : Écraser les IDs avec ceux vérifiés en base
	req.GroupID = groupID
	req.CustomerID = customer.ID

	response, err := h.payCycleUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("PAY_CYCLE_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Lister mes paiements tontine dans un groupe
// @Description Retourne l'historique des cotisations du client authentifié (customer_id forcé via JWT — anti-IDOR).
// @Tags Tontine
// @Accept json
// @Produce json
// @Param group_id path string true "ID du groupe de tontine (UUID)"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} utils.AppError "Paramètres manquants"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Groupe ou profil client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/{group_id}/payments [get]
func (h *TontineHandler) ListCustomerPayments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		return utils.ErrInvalidPayload
	}

	// 1. 🛡️ SÉCURITÉ : Récupérer l'ID utilisateur du JWT (ignore tout paramètre de requête customer_id)
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	// 2. 🛡️ TRADUCTION SÉCURISÉE User → Customer
	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found for list payments")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	// 3. Exécuter le usecase avec le customerID sécurisé
	payments, err := h.listCustomerPaymentsUC.Execute(ctx, groupID, customer.ID)
	if err != nil {
		return utils.NewAppError("LIST_PAYMENTS_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, payments)
	return nil
}

// RegisterRoutes enregistre les routes tontine dans le router
func (h *TontineHandler) RegisterRoutes(r chi.Router) {
	r.Route("/tontine", func(r chi.Router) {
		r.Post("/groups", middl.ErrorHandler(h.CreateGroup))
		r.Post("/groups/join", middl.ErrorHandler(h.JoinGroup))
		r.Post("/groups/{group_id}/pay", middl.ErrorHandler(h.PayCycle))
		r.Get("/groups/{group_id}/payments", middl.ErrorHandler(h.ListCustomerPayments))
	})
}
