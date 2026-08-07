package tontinehandler

import (
	"encoding/json"
	"errors" // 🆕 AJOUTÉ pour la détection d'erreurs
	"net/http"
	"strings" // 🆕 AJOUTÉ pour la détection d'erreurs métier

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
	redeemVoucherUC        *tontineusecase.RedeemTontineVoucherUsecase // Phase 5
	voucherRepo            repository.TontineVoucherRepository         // 🆕 Phase 5 : pour ListVouchers
	customerRepo           repository.CustomerRepositoryInterface
}

// NewTontineHandler crée une nouvelle instance
func NewTontineHandler(
	createGroupUC *tontineusecase.CreateTontineGroupUsecase,
	joinGroupUC *tontineusecase.JoinTontineGroupUsecase,
	payCycleUC *tontineusecase.PayCycleUsecase,
	listCustomerPaymentsUC *tontineusecase.ListCustomerPaymentsUsecase,
	syncPaymentUC *tontineusecase.SyncTontinePaymentUsecase,
	redeemVoucherUC *tontineusecase.RedeemTontineVoucherUsecase, // Phase 5 (peut être nil)
	voucherRepo repository.TontineVoucherRepository, // 🆕 Phase 5
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

// JoinGroup permet à un client de rejoindre un groupe via invite_code
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

// PayCycle initie le paiement d'un cycle pour le client connecté
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

// ListCustomerPayments liste les paiements du client connecté pour un groupe
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

// SyncPayment synchronise le statut d'un paiement tontine (ownership via JWT)
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

// ============================================================
// Phase 5 : ListVouchers
// GET /api/tontine/vouchers?group_id=xxx
//
// Comportement :
//   - Si group_id fourni : retourne tous les vouchers du groupe (filtré par tenant shop_id)
//   - Si group_id absent ET user a un profil customer : retourne les vouchers du client connecté
//   - Si group_id absent ET user n'a pas de profil customer (marchand) : retourne tous les vouchers du shop
//
// ============================================================
func (h *TontineHandler) ListVouchers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Vérifier le tenant (shop_id)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		// 🆕 FIX : Ne pas retourner d'erreur si le tenant n'est pas résolu
		// Retourner un tableau vide au lieu d'une 404
		logger.Warn().Err(err).Msg("Tenant context not resolved, returning empty vouchers list")
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
		// Lister tous les vouchers d'un groupe (filtré par shop_id via tenant)
		vouchers, err = h.voucherRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			logger.Error().Err(err).Str("group_id", groupID).Msg("Failed to list vouchers by group")
			// 🆕 FIX : Retourner un tableau vide au lieu d'une erreur 500
			vouchers = []*entity.TontineVoucher{}
		}
	} else {
		// 🆕 FIX CRITIQUE : Essayer d'abord de trouver un profil customer
		customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
		if err != nil {
			// Pas de profil customer → c'est probablement un marchand
			// Retourner tous les vouchers du shop
			logger.Debug().Str("user_id", authUserID).Msg("No customer profile found, fetching shop vouchers")
			vouchers, err = h.voucherRepo.FindByShopID(ctx, shop.ID.String())
			if err != nil {
				logger.Error().Err(err).Str("shop_id", shop.ID.String()).Msg("Failed to list vouchers by shop")
				// Retourner un tableau vide au lieu d'une erreur
				vouchers = []*entity.TontineVoucher{}
			}
		} else {
			// Profil customer trouvé → retourner les vouchers du client
			vouchers, err = h.voucherRepo.FindByCustomerID(ctx, customer.ID)
			if err != nil {
				logger.Error().Err(err).Str("customer_id", customer.ID).Msg("Failed to list vouchers by customer")
				vouchers = []*entity.TontineVoucher{}
			}
		}
	}

	// Filtrer par shop_id pour sécurité multi-tenant (au cas où le repo ne l'a pas fait)
	filtered := make([]*entity.TontineVoucher, 0)
	for _, v := range vouchers {
		if v.ShopID == shop.ID.String() {
			filtered = append(filtered, v)
		}
	}

	// 🆕 FIX : Toujours retourner 200 OK avec un tableau (même vide), jamais de 404
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    filtered,
		"count":   len(filtered),
	})
	return nil
}

// ============================================================
// Phase 5 : RedeemVoucher
// POST /api/tontine/vouchers/redeem
// Body: { "voucher_code": "ABC123..." }
//
// Effet :
//  1. Marque le voucher comme "redeemed"
//  2. Libère held_amount_cents sur le wallet marchand
//
// ============================================================
func (h *TontineHandler) RedeemVoucher(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
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
		RedeemedBy:  authUserID, // forcé depuis le JWT
	}

	resp, err := h.redeemVoucherUC.Execute(ctx, req)
	if err != nil {
		logger.Error().Err(err).
			Str("voucher_code", body.VoucherCode).
			Str("redeemed_by", authUserID).
			Msg("Failed to redeem tontine voucher")

		// 🆕 FIX CRITIQUE : Détection robuste des erreurs métier
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
