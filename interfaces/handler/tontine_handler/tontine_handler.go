package tontinehandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"Goshop/application/metrics"
	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
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
	syncPaymentUC          *tontineusecase.SyncTontinePaymentUsecase
	redeemVoucherUC        *tontineusecase.RedeemTontineVoucherUsecase
	voucherRepo            repository.TontineVoucherRepository
	customerRepo           repository.CustomerRepositoryInterface
}

// NewTontineHandler crée une nouvelle instance
func NewTontineHandler(
	createGroupUC *tontineusecase.CreateTontineGroupUsecase,
	joinGroupUC *tontineusecase.JoinTontineGroupUsecase,
	payCycleUC *tontineusecase.PayCycleUsecase,
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase,
	syncPaymentUC *tontineusecase.SyncTontinePaymentUsecase,
	redeemVoucherUC *tontineusecase.RedeemTontineVoucherUsecase,
	voucherRepo repository.TontineVoucherRepository,
	customerRepo repository.CustomerRepositoryInterface,
) *TontineHandler {
	return &TontineHandler{
		createGroupUC:          createGroupUC,
		joinGroupUC:            joinGroupUC,
		payCycleUC:             payCycleUC,
		listCustomerPaymentsUC: listCustomerPaymentsUC,
		syncPaymentUC:          syncPaymentUC,
		redeemVoucherUC:        redeemVoucherUC,
		voucherRepo:            voucherRepo,
		customerRepo:           customerRepo,
	}
}

// CreateGroup initialise un nouveau groupe de tontine
func (h *TontineHandler) CreateGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
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
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUE : Échec création de groupe
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_create_group", "tontine_handler").Inc()
		metrics.TontineOperationDuration.WithLabelValues("create_group").Observe(duration)
		return utils.NewAppError("CREATE_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès création de groupe
	metrics.TontineGroupCreatedTotal.Inc()
	metrics.TontineOperationDuration.WithLabelValues("create_group").Observe(duration)

	logger.Info().
		Str("group_id", group.ID).
		Float64("duration_seconds", duration).
		Msg("Tontine group created successfully")

	utils.WriteJSON(w, http.StatusCreated, group)
	return nil
}

// JoinGroup permet à un client de rejoindre un groupe via invite_code
func (h *TontineHandler) JoinGroup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

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
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUE : Échec de join
		metrics.TontineGroupJoinTotal.WithLabelValues("error").Inc()
		metrics.TontineOperationDuration.WithLabelValues("join_group").Observe(duration)
		return utils.NewAppError("JOIN_GROUP_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès de join
	metrics.TontineGroupJoinTotal.WithLabelValues("success").Inc()
	metrics.TontineOperationDuration.WithLabelValues("join_group").Observe(duration)

	logger.Info().
		Str("customer_id", customer.ID).
		Float64("duration_seconds", duration).
		Msg("Customer joined tontine group successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// PayCycle initie le paiement d'un cycle pour le client connecté
func (h *TontineHandler) PayCycle(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

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
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec de paiement
		metrics.TontinePaymentFailedTotal.Inc()
		metrics.TontinePaymentDuration.Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_pay_cycle", "tontine_handler").Inc()

		logger.Error().
			Err(err).
			Str("group_id", groupID).
			Str("customer_id", customer.ID).
			Float64("duration_seconds", duration).
			Msg("Failed to initiate tontine payment")

		return utils.NewAppError("PAY_CYCLE_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès de paiement initié
	metrics.TontinePaymentInitiatedTotal.WithLabelValues(req.Operator).Inc()
	metrics.TontinePaymentSuccessTotal.Inc()
	metrics.TontinePaymentDuration.Observe(duration)

	logger.Info().
		Str("group_id", groupID).
		Str("customer_id", customer.ID).
		Str("operator", req.Operator).
		Float64("duration_seconds", duration).
		Msg("Tontine payment initiated successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ListCustomerPayments liste les paiements du client connecté pour un groupe
func (h *TontineHandler) ListCustomerPayments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

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
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.TontineOperationDuration.WithLabelValues("list_payments").Observe(duration)
		return utils.NewAppError("LIST_PAYMENTS_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUE : Durée de listing
	metrics.TontineOperationDuration.WithLabelValues("list_payments").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, payments)
	return nil
}

// SyncPayment synchronise le statut d'un paiement tontine (ownership via JWT)
func (h *TontineHandler) SyncPayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
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
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec de sync
		metrics.TontineSyncPaymentTotal.WithLabelValues("error").Inc()
		metrics.TontineOperationDuration.WithLabelValues("sync_payment").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_sync_payment", "tontine_handler").Inc()

		logger.Error().
			Err(err).
			Str("payment_id", req.PaymentID).
			Float64("duration_seconds", duration).
			Msg("Failed to sync tontine payment")

		return utils.NewAppError("SYNC_PAYMENT_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès de sync
	status := "no_change"
	if resp.Synced {
		status = "synced"
	}
	metrics.TontineSyncPaymentTotal.WithLabelValues(status).Inc()
	metrics.TontineOperationDuration.WithLabelValues("sync_payment").Observe(duration)

	logger.Info().
		Str("payment_id", req.PaymentID).
		Str("sync_status", status).
		Float64("duration_seconds", duration).
		Msg("Tontine payment synced")

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// ListVouchers liste les vouchers de tontine
func (h *TontineHandler) ListVouchers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// Vérifier le tenant (shop_id)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Tenant context not resolved, returning empty vouchers list")
		metrics.TontineVoucherListedTotal.Inc()
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*entity.TontineVoucher{},
			"count":   0,
		})
		return nil
	}

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	groupID := r.URL.Query().Get("group_id")

	var vouchers []*entity.TontineVoucher

	if groupID != "" {
		vouchers, err = h.voucherRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			logger.Error().Err(err).Str("group_id", groupID).Msg("Failed to list vouchers by group")
			vouchers = []*entity.TontineVoucher{}
		}
	} else {
		customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
		if err != nil {
			logger.Debug().Str("user_id", authUserID).Msg("No customer profile found, fetching shop vouchers")
			vouchers, err = h.voucherRepo.FindByShopID(ctx, shop.ID.String())
			if err != nil {
				logger.Error().Err(err).Str("shop_id", shop.ID.String()).Msg("Failed to list vouchers by shop")
				vouchers = []*entity.TontineVoucher{}
			}
		} else {
			vouchers, err = h.voucherRepo.FindByCustomerID(ctx, customer.ID)
			if err != nil {
				logger.Error().Err(err).Str("customer_id", customer.ID).Msg("Failed to list vouchers by customer")
				vouchers = []*entity.TontineVoucher{}
			}
		}
	}

	// Filtrer par shop_id pour sécurité multi-tenant
	filtered := make([]*entity.TontineVoucher, 0)
	for _, v := range vouchers {
		if v.ShopID == shop.ID.String() {
			filtered = append(filtered, v)
		}
	}

	// 📊 MÉTRIQUE : Vouchers listés
	metrics.TontineVoucherListedTotal.Inc()
	metrics.TontineOperationDuration.WithLabelValues("list_vouchers").Observe(time.Since(start).Seconds())

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    filtered,
		"count":   len(filtered),
	})
	return nil
}

// RedeemVoucher échange un voucher de tontine
func (h *TontineHandler) RedeemVoucher(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	if h.redeemVoucherUC == nil {
		return utils.NewAppError("NOT_IMPLEMENTED", "redeem voucher usecase not configured", http.StatusNotImplemented)
	}

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	var body struct {
		VoucherCode string `json:"voucher_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	if body.VoucherCode == "" {
		return utils.NewAppError("INVALID_PAYLOAD", "voucher_code is required", http.StatusBadRequest)
	}

	req := &tontineusecase.RedeemTontineVoucherRequest{
		VoucherCode: body.VoucherCode,
		RedeemedBy:  authUserID,
	}

	resp, err := h.redeemVoucherUC.Execute(ctx, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec de redeem
		metrics.TontineVoucherRedeemFailedTotal.Inc()
		metrics.TontineOperationDuration.WithLabelValues("redeem_voucher").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_redeem_voucher", "tontine_handler").Inc()

		logger.Error().
			Err(err).
			Str("voucher_code", body.VoucherCode).
			Str("redeemed_by", authUserID).
			Float64("duration_seconds", duration).
			Msg("Failed to redeem tontine voucher")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "already redeemed") ||
			strings.Contains(errMsg, "expired") ||
			strings.Contains(errMsg, "not redeemable") {
			return utils.NewAppError("VOUCHER_NOT_REDEEMABLE", errMsg, http.StatusBadRequest)
		}

		return utils.NewAppError("REDEEM_VOUCHER_FAILED", errMsg, http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès de redeem
	metrics.TontineVoucherRedeemedTotal.Inc()
	metrics.TontineOperationDuration.WithLabelValues("redeem_voucher").Observe(duration)

	logger.Info().
		Str("voucher_code", body.VoucherCode).
		Str("redeemed_by", authUserID).
		Float64("duration_seconds", duration).
		Msg("Tontine voucher redeemed successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// RegisterRoutes enregistre les routes tontine
func (h *TontineHandler) RegisterRoutes(r chi.Router) {
	r.Route("/tontine", func(r chi.Router) {
		r.Post("/groups", middl.ErrorHandler(h.CreateGroup))
		r.Post("/groups/join", middl.ErrorHandler(h.JoinGroup))
		r.Post("/groups/{group_id}/pay", middl.ErrorHandler(h.PayCycle))
		r.Get("/groups/{group_id}/payments", middl.ErrorHandler(h.ListCustomerPayments))
		r.Post("/payments/sync", middl.ErrorHandler(h.SyncPayment))
		// Phase 5 : Vouchers
		r.Get("/vouchers", middl.ErrorHandler(h.ListVouchers))
		r.Post("/vouchers/redeem", middl.ErrorHandler(h.RedeemVoucher))
	})
}
