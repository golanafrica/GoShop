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

// CreateGroup gère POST /api/tontine/groups
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

// JoinGroup gère POST /api/tontine/groups/join
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

// PayCycle gère POST /api/tontine/groups/{group_id}/pay
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

// ListCustomerPayments gère GET /api/tontine/groups/{group_id}/payments
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
