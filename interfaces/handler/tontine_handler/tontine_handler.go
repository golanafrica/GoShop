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

type TontineHandler struct {
	createGroupUC          *tontineusecase.CreateTontineGroupUsecase
	joinGroupUC            *tontineusecase.JoinTontineGroupUsecase
	payCycleUC             *tontineusecase.PayCycleUsecase
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase
	syncPaymentUC          *tontineusecase.SyncTontinePaymentUsecase
	customerRepo           repository.CustomerRepositoryInterface
}

func NewTontineHandler(
	createGroupUC *tontineusecase.CreateTontineGroupUsecase,
	joinGroupUC *tontineusecase.JoinTontineGroupUsecase,
	payCycleUC *tontineusecase.PayCycleUsecase,
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase,
	syncPaymentUC *tontineusecase.SyncTontinePaymentUsecase,
	customerRepo repository.CustomerRepositoryInterface,
) *TontineHandler {
	return &TontineHandler{
		createGroupUC:          createGroupUC,
		joinGroupUC:            joinGroupUC,
		payCycleUC:             payCycleUC,
		listCustomerPaymentsUC: listCustomerPaymentsUC,
		syncPaymentUC:          syncPaymentUC,
		customerRepo:           customerRepo,
	}
}

func (h *TontineHandler) CreateGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.CreatorCustomerID = customer.ID

	group, err := h.createGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("CREATE_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, group)
	return nil
}

func (h *TontineHandler) JoinGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.JoinGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.CustomerID = customer.ID

	response, err := h.joinGroupUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("JOIN_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

func (h *TontineHandler) PayCycle(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		return utils.ErrInvalidPayload
	}

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.PayCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.GroupID = groupID
	req.CustomerID = customer.ID

	response, err := h.payCycleUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("PAY_CYCLE_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

func (h *TontineHandler) ListCustomerPayments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		return utils.ErrInvalidPayload
	}

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	payments, err := h.listCustomerPaymentsUC.Execute(ctx, groupID, customer.ID)
	if err != nil {
		return utils.NewAppError("LIST_PAYMENTS_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, payments)
	return nil
}

// 🆕 FIX B2 : SyncPayment avec vérification d'ownership via JWT
func (h *TontineHandler) SyncPayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found for sync")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	var req tontineusecase.SyncPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	if req.PaymentID == "" {
		return utils.NewAppError("INVALID_PAYLOAD", "payment_id is required", http.StatusBadRequest)
	}

	// 🛡️ FIX B2 : Forcer le customer_id du JWT
	req.CustomerID = customer.ID

	resp, err := h.syncPaymentUC.Execute(ctx, &req)
	if err != nil {
		logger.Error().Err(err).Str("payment_id", req.PaymentID).Msg("Failed to sync tontine payment")
		return utils.NewAppError("SYNC_PAYMENT_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *TontineHandler) RegisterRoutes(r chi.Router) {
	r.Route("/tontine", func(r chi.Router) {
		r.Post("/groups", middl.ErrorHandler(h.CreateGroup))
		r.Post("/groups/join", middl.ErrorHandler(h.JoinGroup))
		r.Post("/groups/{group_id}/pay", middl.ErrorHandler(h.PayCycle))
		r.Get("/groups/{group_id}/payments", middl.ErrorHandler(h.ListCustomerPayments))
		r.Post("/payments/sync", middl.ErrorHandler(h.SyncPayment))
	})
}
