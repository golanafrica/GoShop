package tontinehandler

import (
	"encoding/json"
	"net/http"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
)

// TontineHandler gère les routes de la tontine
type TontineHandler struct {
	createGroupUC          *tontineusecase.CreateTontineGroupUsecase
	joinGroupUC            *tontineusecase.JoinTontineGroupUsecase
	payCycleUC             *tontineusecase.PayCycleUsecase
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase
}

// NewTontineHandler crée une nouvelle instance
func NewTontineHandler(
	createGroupUC *tontineusecase.CreateTontineGroupUsecase,
	joinGroupUC *tontineusecase.JoinTontineGroupUsecase,
	payCycleUC *tontineusecase.PayCycleUsecase,
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase,
) *TontineHandler {
	return &TontineHandler{
		createGroupUC:          createGroupUC,
		joinGroupUC:            joinGroupUC,
		payCycleUC:             payCycleUC,
		listCustomerPaymentsUC: listCustomerPaymentsUC,
	}
}

// @Summary Créer un groupe de tontine
// @Description Initialise un nouveau groupe de tontine pour un produit spécifique.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param request body tontineusecase.CreateGroupRequest true "Détails du groupe à créer"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou règles de tontine non respectées"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (KYC non validé ou droits insuffisants)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups [post]
func (h *TontineHandler) CreateGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req tontineusecase.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	group, err := h.createGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("CREATE_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, group)
	return nil
}

// @Summary Rejoindre un groupe de tontine
// @Description Permet à un client de rejoindre un groupe de tontine existant via un code d'invitation.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param request body tontineusecase.JoinGroupRequest true "Code d'invitation et ID client"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou code incorrect"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (KYC non validé)"
// @Failure 404 {object} utils.AppError "Groupe introuvable"
// @Failure 409 {object} utils.AppError "Groupe déjà complet ou client déjà membre"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/join [post]
func (h *TontineHandler) JoinGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req tontineusecase.JoinGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	response, err := h.joinGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("JOIN_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Payer une cotisation de tontine
// @Description Lance le processus de paiement pour la cotisation d'un cycle spécifique.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param group_id path string true "ID du groupe de tontine (UUID)"
// @Param request body tontineusecase.PayCycleRequest true "Détails du paiement (provider, phone_number, etc.)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou cycle non éligible"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Groupe ou cycle introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/{group_id}/pay [post]
func (h *TontineHandler) PayCycle(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		return utils.ErrInvalidPayload
	}

	var req tontineusecase.PayCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.GroupID = groupID

	response, err := h.payCycleUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("PAY_CYCLE_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Lister les paiements d'un client dans un groupe
// @Description Retourne l'historique des cotisations payées par un client spécifique dans un groupe de tontine.
// @Tags Tontine
// @Accept json
// @Produce json
// @Param group_id path string true "ID du groupe de tontine (UUID)"
// @Param customer_id query string true "ID du client (UUID)"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} utils.AppError "Paramètres manquants"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (accès aux données d'un autre client)"
// @Failure 404 {object} utils.AppError "Groupe introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/tontine/groups/{group_id}/payments [get]
func (h *TontineHandler) ListCustomerPayments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	groupID := chi.URLParam(r, "group_id")
	customerID := r.URL.Query().Get("customer_id")

	if groupID == "" || customerID == "" {
		return utils.ErrInvalidPayload
	}

	payments, err := h.listCustomerPaymentsUC.Execute(ctx, groupID, customerID)
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
