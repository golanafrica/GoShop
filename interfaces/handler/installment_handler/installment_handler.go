package installmenthandler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	installment_dto "Goshop/application/dto/installment_dto"
	installmentusecase "Goshop/application/usecase/installment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// INTERFACES (Découplage du handler)
// ============================================================

type ConfigurePlanUC interface {
	Execute(ctx context.Context, req installmentusecase.ConfigurePlanRequest) (*entity.InstallmentPlan, error)
}

type GetInstallmentsUC interface {
	Execute(ctx context.Context, orderID string) (*installmentusecase.InstallmentsResponse, error)
}

type ReleaseEscrowUC interface {
	Execute(ctx context.Context, orderID string) (*installmentusecase.ReleaseResponse, error)
}

type GetMerchantDashboardUC interface {
	Execute(ctx context.Context, shopID string) (*installment_dto.MerchantInstallmentDashboardResponse, []*installment_dto.InstallmentOrderSummary, error)
}

// ============================================================
// HANDLER
// ============================================================

type InstallmentHandler struct {
	configurePlanUC        ConfigurePlanUC
	createOrderUC          *installmentusecase.CreateInstallmentOrderUsecase // 🆕 AJOUTÉ
	getInstallmentsUC      GetInstallmentsUC
	releaseEscrowUC        ReleaseEscrowUC
	getMerchantDashboardUC GetMerchantDashboardUC
}

func NewInstallmentHandler(
	configurePlanUC ConfigurePlanUC,
	createOrderUC *installmentusecase.CreateInstallmentOrderUsecase,
	getInstallmentsUC GetInstallmentsUC,
	releaseEscrowUC ReleaseEscrowUC,
	getMerchantDashboardUC GetMerchantDashboardUC,
) *InstallmentHandler {
	return &InstallmentHandler{
		configurePlanUC:        configurePlanUC,
		createOrderUC:          createOrderUC,
		getInstallmentsUC:      getInstallmentsUC,
		releaseEscrowUC:        releaseEscrowUC,
		getMerchantDashboardUC: getMerchantDashboardUC,
	}
}

func (h *InstallmentHandler) RegisterRoutes(r chi.Router) {
	r.Post("/products/{product_id}/installment-plan", middl.ErrorHandler(h.ConfigurePlan))
	r.Get("/products/{product_id}/installment-plan", middl.ErrorHandler(h.GetPlan))

	// 🆕 AJOUT : Endpoint pour créer une commande en tranches
	r.Post("/orders/installment", middl.ErrorHandler(h.CreateInstallmentOrder))

	r.Get("/orders/{order_id}/installments", middl.ErrorHandler(h.GetInstallments))
	r.Post("/orders/{order_id}/release-escrow", middl.ErrorHandler(h.ReleaseEscrow))

	// 🆕 v5.3.0 : Routes Dashboard Marchand
	r.Get("/merchant/installments/dashboard", middl.ErrorHandler(h.GetMerchantDashboard))
}

// ============================================================
// POST /api/products/{product_id}/installment-plan
// ============================================================
func (h *InstallmentHandler) ConfigurePlan(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	productID := chi.URLParam(r, "product_id")
	if productID == "" {
		return utils.NewAppError("INVALID_PRODUCT_ID", "ID du produit manquant", http.StatusBadRequest)
	}

	var req struct {
		NbTranches       int    `json:"nb_tranches"`
		DelaiJours       int    `json:"delai_jours"`
		DeliveryZoneCode string `json:"delivery_zone_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload for installment plan")
		return utils.ErrInvalidPayload
	}

	plan, err := h.configurePlanUC.Execute(ctx, installmentusecase.ConfigurePlanRequest{
		ProductID:        productID,
		NbTranches:       req.NbTranches,
		DelaiJours:       req.DelaiJours,
		DeliveryZoneCode: req.DeliveryZoneCode,
	})
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Str("product_id", productID).Float64("duration_seconds", duration).Msg("Failed to configure installment plan")
		return utils.NewAppError("CONFIGURE_PLAN_FAILED", err.Error(), http.StatusBadRequest)
	}

	logger.Info().Str("product_id", productID).Str("plan_id", plan.ID).Int("tranches", plan.NbTranches).Float64("duration_seconds", duration).Msg("Installment plan configured successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "plan": plan})
	return nil
}

// ============================================================
// GET /api/products/{product_id}/installment-plan
// ============================================================
func (h *InstallmentHandler) GetPlan(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	productID := chi.URLParam(r, "product_id")
	if productID == "" {
		return utils.NewAppError("INVALID_PRODUCT_ID", "ID du produit manquant", http.StatusBadRequest)
	}

	plan, err := h.configurePlanUC.Execute(ctx, installmentusecase.ConfigurePlanRequest{
		ProductID:  productID,
		NbTranches: 0,
		DelaiJours: 0,
	})

	if err != nil || plan == nil {
		logger.Debug().Str("product_id", productID).Msg("No installment plan found")
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true, "plan": nil, "message": "Aucun plan configuré",
		})
		return nil
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "plan": plan})
	return nil
}

// ============================================================
// 🆕 POST /api/orders/installment
// ============================================================
func (h *InstallmentHandler) CreateInstallmentOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return utils.NewAppError("UNAUTHORIZED", "Boutique non identifiée", http.StatusUnauthorized)
	}

	var req struct {
		CustomerID    string `json:"customer_id"`
		PaymentMethod string `json:"payment_method"`
		Items         []struct {
			ProductID  string `json:"product_id"`
			Quantity   int    `json:"quantity"`
			PriceCents int64  `json:"price_cents"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload for installment order")
		return utils.ErrInvalidPayload
	}

	if len(req.Items) == 0 {
		return utils.NewAppError("INVALID_PAYLOAD", "Au moins un article est requis", http.StatusBadRequest)
	}

	var items []*entity.OrderItem
	var totalCents int64
	for _, item := range req.Items {
		items = append(items, &entity.OrderItem{
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			PriceCents: item.PriceCents,
		})
		totalCents += item.PriceCents * int64(item.Quantity)
	}

	order, err := h.createOrderUC.Execute(ctx, shop.ID.String(), req.CustomerID, totalCents, items)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create installment order")
		return utils.NewAppError("CREATE_ORDER_FAILED", err.Error(), http.StatusBadRequest)
	}

	logger.Info().Str("order_id", order.ID).Msg("Installment order created successfully")
	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "order": order})
	return nil
}

// ============================================================
// GET /api/orders/{order_id}/installments
// ============================================================
func (h *InstallmentHandler) GetInstallments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		return utils.NewAppError("INVALID_ORDER_ID", "ID de commande manquant", http.StatusBadRequest)
	}

	resp, err := h.getInstallmentsUC.Execute(ctx, orderID)
	if err != nil {
		logger.Error().Err(err).Str("order_id", orderID).Msg("Failed to get installments")
		return utils.NewAppError("GET_INSTALLMENTS_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"installments":    resp.Installments,
		"total_paid":      resp.TotalPaid,
		"total_remaining": resp.TotalRemaining,
	})
	return nil
}

// ============================================================
// POST /api/orders/{order_id}/release-escrow
// ============================================================
func (h *InstallmentHandler) ReleaseEscrow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		return utils.NewAppError("INVALID_ORDER_ID", "ID de commande manquant", http.StatusBadRequest)
	}

	resp, err := h.releaseEscrowUC.Execute(ctx, orderID)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Str("order_id", orderID).Float64("duration_seconds", duration).Msg("Failed to release escrow funds")
		return utils.NewAppError("RELEASE_ESCROW_FAILED", err.Error(), http.StatusBadRequest)
	}

	logger.Info().Str("order_id", orderID).Int64("net_cents", resp.NetMerchantCents).Int64("commission_cents", resp.CommissionCents).Float64("duration_seconds", duration).Msg("Escrow funds released successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "release": resp})
	return nil
}

// ============================================================
// 🆕 v5.3.0 : GET /api/merchant/installments/dashboard
// ============================================================
func (h *InstallmentHandler) GetMerchantDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Shop not found in context")
		return utils.NewAppError("UNAUTHORIZED", "Boutique non identifiée", http.StatusUnauthorized)
	}

	dashboard, summaries, err := h.getMerchantDashboardUC.Execute(ctx, shop.ID.String())
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shop.ID.String()).Msg("Failed to get merchant dashboard")
		return utils.NewAppError("DASHBOARD_FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"dashboard": dashboard,
		"orders":    summaries,
	})
	return nil
}
