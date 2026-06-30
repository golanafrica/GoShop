package credit_handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// CREDIT HANDLER
// ============================================================

// CreditHandler gère les endpoints liés au crédit par tempérament
type CreditHandler struct {
	configureUC *creditusecase.ConfigureCreditPlanUsecase
	applyUC     *creditusecase.ApplyForCreditUsecase
	approveUC   *creditusecase.ApproveCreditUsecase
	rejectUC    *creditusecase.RejectCreditUsecase
	payDownUC   *creditusecase.PayDownPaymentUsecase
}

// NewCreditHandler crée une nouvelle instance du handler
func NewCreditHandler(
	configureUC *creditusecase.ConfigureCreditPlanUsecase,
	applyUC *creditusecase.ApplyForCreditUsecase,
	approveUC *creditusecase.ApproveCreditUsecase,
	rejectUC *creditusecase.RejectCreditUsecase,
	payDownUC *creditusecase.PayDownPaymentUsecase,
) *CreditHandler {
	return &CreditHandler{
		configureUC: configureUC,
		applyUC:     applyUC,
		approveUC:   approveUC,
		rejectUC:    rejectUC,
		payDownUC:   payDownUC,
	}
}

// ============================================================
// REQUEST/RESPONSE TYPES
// ============================================================

// ConfigurePlanRequest représente la requête pour configurer un plan
type ConfigurePlanRequest struct {
	ProductID             string `json:"product_id"`
	IsEnabled             bool   `json:"is_enabled"`
	MinDownPaymentPercent int    `json:"min_down_payment_percent"`
	MaxDurationMonths     int    `json:"max_duration_months"`
	InterestRateBps       int    `json:"interest_rate_bps"`
	PenaltyRateBps        int    `json:"penalty_rate_bps"`
	MinCreditScore        int    `json:"min_credit_score"`
}

// ApplyForCreditRequest représente la requête pour demander un crédit
type ApplyForCreditRequest struct {
	CustomerID              string `json:"customer_id"`
	ProductID               string `json:"product_id"`
	RequestedDurationMonths int    `json:"requested_duration_months"`
}

// ApproveCreditRequest représente la requête pour approuver
type ApproveCreditRequest struct {
	ApplicationID string `json:"application_id"`
}

// RejectCreditRequest représente la requête pour rejeter
type RejectCreditRequest struct {
	ApplicationID   string `json:"application_id"`
	RejectionReason string `json:"rejection_reason"`
}

// PayDownPaymentRequest représente la requête pour payer l'apport
type PayDownPaymentRequest struct {
	ContractID string `json:"contract_id"`
	PaymentID  string `json:"payment_id"`
}

// SimulateCreditRequest représente la requête pour simuler
type SimulateCreditRequest struct {
	ProductID               string `json:"product_id"`
	RequestedDurationMonths int    `json:"requested_duration_months"`
}

// ============================================================
// HANDLERS : CREDIT PLANS (MARCHAND)
// ============================================================

// ConfigureCreditPlan configure un plan de crédit pour un produit
// POST /api/credit/plans
func (h *CreditHandler) ConfigureCreditPlan(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req ConfigurePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.ProductID == "" {
		utils.WriteError(w, http.StatusBadRequest, "product_id is required")
		return
	}

	// 4. Appeler le usecase
	ucReq := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             req.ProductID,
		IsEnabled:             req.IsEnabled,
		MinDownPaymentPercent: req.MinDownPaymentPercent,
		MaxDurationMonths:     req.MaxDurationMonths,
		InterestRateBps:       req.InterestRateBps,
		PenaltyRateBps:        req.PenaltyRateBps,
		MinCreditScore:        req.MinCreditScore,
	}

	resp, err := h.configureUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to configure credit plan")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to configure plan: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("product_id", req.ProductID).
		Bool("is_enabled", req.IsEnabled).
		Str("action", resp.Action).
		Msg("Credit plan configured")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Credit plan %s successfully", resp.Action),
		"action":  resp.Action,
		"plan":    resp,
	})
}

// GetCreditPlan retourne le plan de crédit d'un produit
// GET /api/credit/plans/{product_id}
func (h *CreditHandler) GetCreditPlan(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Récupérer le product_id depuis l'URL
	productID := chi.URLParam(r, "product_id")
	if productID == "" {
		utils.WriteError(w, http.StatusBadRequest, "product_id is required in URL")
		return
	}

	// 3. Récupérer le plan
	plan, err := h.configureUC.GetCreditPlan(r.Context(), productID)
	if err != nil {
		logger.Error().Err(err).Str("product_id", productID).Msg("Credit plan not found")
		utils.WriteError(w, http.StatusNotFound, "Credit plan not found for this product")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"plan": map[string]interface{}{
			"id":                       plan.ID,
			"product_id":               plan.ProductID,
			"shop_id":                  plan.ShopID,
			"is_enabled":               plan.IsEnabled,
			"min_down_payment_percent": plan.MinDownPaymentPercent,
			"max_duration_months":      plan.MaxDurationMonths,
			"interest_rate_bps":        plan.InterestRateBps,
			"interest_rate_percent":    plan.InterestRatePercent(),
			"penalty_rate_bps":         plan.PenaltyRateBps,
			"penalty_rate_percent":     float64(plan.PenaltyRateBps) / 100.0,
			"min_credit_score":         plan.MinCreditScore,
			"created_at":               plan.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"updated_at":               plan.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}

// ListCreditPlans retourne tous les plans d'une boutique
// GET /api/credit/plans
func (h *CreditHandler) ListCreditPlans(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Filtrer par statut (optionnel)
	enabledOnly := r.URL.Query().Get("enabled") == "true"

	var plans []*entity.CreditPlan
	if enabledOnly {
		plans, err = h.configureUC.ListEnabledCreditPlansByShop(r.Context(), shopID)
	} else {
		plans, err = h.configureUC.ListCreditPlansByShop(r.Context(), shopID)
	}

	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to list credit plans")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve credit plans")
		return
	}

	// 3. Construire la réponse
	planResponses := make([]map[string]interface{}, 0, len(plans))
	for _, plan := range plans {
		planResponses = append(planResponses, map[string]interface{}{
			"id":                       plan.ID,
			"product_id":               plan.ProductID,
			"is_enabled":               plan.IsEnabled,
			"min_down_payment_percent": plan.MinDownPaymentPercent,
			"max_duration_months":      plan.MaxDurationMonths,
			"interest_rate_bps":        plan.InterestRateBps,
			"interest_rate_percent":    plan.InterestRatePercent(),
			"penalty_rate_bps":         plan.PenaltyRateBps,
			"min_credit_score":         plan.MinCreditScore,
		})
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"plans": planResponses,
		"count": len(planResponses),
	})
}

// ============================================================
// HANDLERS : CREDIT APPLICATIONS (CLIENT)
// ============================================================

// ApplyForCredit permet à un client de demander un crédit
// POST /api/credit/apply
func (h *CreditHandler) ApplyForCredit(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req ApplyForCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.CustomerID == "" || req.ProductID == "" {
		utils.WriteError(w, http.StatusBadRequest, "customer_id and product_id are required")
		return
	}
	if req.RequestedDurationMonths <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "requested_duration_months must be positive")
		return
	}

	// 4. Appeler le usecase
	ucReq := &creditusecase.ApplyForCreditRequest{
		CustomerID:              req.CustomerID,
		ProductID:               req.ProductID,
		RequestedDurationMonths: req.RequestedDurationMonths,
	}

	resp, err := h.applyUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to apply for credit")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to apply: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("application_id", resp.ApplicationID).
		Str("customer_id", req.CustomerID).
		Str("product_id", req.ProductID).
		Int64("monthly_payment_cents", resp.MonthlyPaymentCents).
		Msg("Credit application created")

	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success":     true,
		"message":     "Credit application created successfully. Waiting for merchant approval.",
		"application": resp,
	})
}

// SimulateCredit simule un crédit sans créer de demande
// POST /api/credit/simulate
func (h *CreditHandler) SimulateCredit(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req SimulateCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs
	if req.ProductID == "" {
		utils.WriteError(w, http.StatusBadRequest, "product_id is required")
		return
	}
	if req.RequestedDurationMonths <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "requested_duration_months must be positive")
		return
	}

	// 4. Appeler le usecase
	resp, err := h.applyUC.SimulateCreditCalculation(r.Context(), req.ProductID, req.RequestedDurationMonths)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to simulate credit")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to simulate: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("product_id", req.ProductID).
		Int("duration_months", req.RequestedDurationMonths).
		Int64("monthly_payment_cents", resp.MonthlyPaymentCents).
		Msg("Credit simulation completed")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Credit simulation completed",
		"simulation": resp,
	})
}

// CheckEligibility vérifie si un client peut demander un crédit
// GET /api/credit/eligibility/{customer_id}/{product_id}
func (h *CreditHandler) CheckEligibility(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Récupérer les paramètres
	customerID := chi.URLParam(r, "customer_id")
	productID := chi.URLParam(r, "product_id")
	if customerID == "" || productID == "" {
		utils.WriteError(w, http.StatusBadRequest, "customer_id and product_id are required in URL")
		return
	}

	// 3. Vérifier l'éligibilité
	eligible, reason, err := h.applyUC.CanApplyForCredit(r.Context(), customerID, productID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check eligibility")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to check eligibility")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"eligible": eligible,
		"reason":   reason,
	})
}

// GetCustomerScore retourne le score de crédit d'un client
// GET /api/credit/score/{customer_id}
func (h *CreditHandler) GetCustomerScore(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer le customer_id
	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		utils.WriteError(w, http.StatusBadRequest, "customer_id is required in URL")
		return
	}

	// 3. Récupérer le score
	score, err := h.applyUC.GetCustomerCreditScore(r.Context(), customerID, shopID)
	if err != nil {
		// Score n'existe pas, retourner score par défaut
		logger.Info().
			Str("customer_id", customerID).
			Str("shop_id", shopID).
			Msg("Customer has no credit score yet")

		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"customer_id": customerID,
			"shop_id":     shopID,
			"score":       500,
			"level":       "good",
			"message":     "No credit history yet. Default score assigned.",
		})
		return
	}

	// 4. Déterminer le niveau
	level := determineScoreLevel(score.Score)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"customer_id":         score.CustomerID,
		"shop_id":             score.ShopID,
		"score":               score.Score,
		"level":               level,
		"total_contracts":     score.TotalContracts,
		"completed_contracts": score.CompletedContracts,
		"on_time_payments":    score.OnTimePayments,
		"late_payments":       score.LatePayments,
		"defaults":            score.Defaults,
		"last_updated_at":     score.LastUpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ============================================================
// HANDLERS : APPROVAL (MARCHAND)
// ============================================================

// ApproveCredit permet au marchand d'approuver une demande
// POST /api/credit/approve
func (h *CreditHandler) ApproveCredit(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'user_id (pour audit)
	userID, _ := utils.UserIDFromContext(r.Context())
	if userID == "" {
		userID = shopID
	}

	// 3. Parser la requête
	var req ApproveCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 4. Valider les champs
	if req.ApplicationID == "" {
		utils.WriteError(w, http.StatusBadRequest, "application_id is required")
		return
	}

	// 5. Appeler le usecase
	ucReq := &creditusecase.ApproveCreditRequest{
		ApplicationID: req.ApplicationID,
		ReviewedBy:    userID,
	}

	resp, err := h.approveUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to approve credit")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to approve: %v", err))
		return
	}

	// 6. Logger et retourner
	logger.Info().
		Str("application_id", req.ApplicationID).
		Str("contract_id", resp.ContractID).
		Int("installments_count", resp.InstallmentsCount).
		Msg("Credit application approved")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Credit application approved. Contract created with installments. Waiting for down payment.",
		"approval": resp,
	})
}

// RejectCredit permet au marchand de rejeter une demande
// POST /api/credit/reject
func (h *CreditHandler) RejectCredit(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'user_id
	userID, _ := utils.UserIDFromContext(r.Context())
	if userID == "" {
		userID = shopID
	}

	// 3. Parser la requête
	var req RejectCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 4. Valider les champs
	if req.ApplicationID == "" || req.RejectionReason == "" {
		utils.WriteError(w, http.StatusBadRequest, "application_id and rejection_reason are required")
		return
	}

	// 5. Appeler le usecase
	ucReq := &creditusecase.RejectCreditRequest{
		ApplicationID:   req.ApplicationID,
		ReviewedBy:      userID,
		RejectionReason: req.RejectionReason,
	}

	resp, err := h.rejectUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to reject credit")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to reject: %v", err))
		return
	}

	// 6. Logger et retourner
	logger.Info().
		Str("application_id", req.ApplicationID).
		Str("reason", req.RejectionReason).
		Msg("Credit application rejected")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "Credit application rejected",
		"rejection": resp,
	})
}

// ListPendingApplications retourne les demandes en attente
// GET /api/credit/applications/pending
func (h *CreditHandler) ListPendingApplications(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les demandes en attente
	apps, err := h.approveUC.GetPendingApplications(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to list pending applications")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve pending applications")
		return
	}

	// 3. Construire la réponse
	appResponses := make([]map[string]interface{}, 0, len(apps))
	for _, app := range apps {
		appResponses = append(appResponses, map[string]interface{}{
			"application_id":            app.ID,
			"customer_id":               app.CustomerID,
			"product_id":                app.ProductID,
			"requested_duration_months": app.RequestedDurationMonths,
			"credit_score":              app.CreditScoreAtApplication,
			"product_price_cents":       app.ProductPriceCents,
			"down_payment_cents":        app.DownPaymentCents,
			"financed_amount_cents":     app.FinancedAmountCents,
			"interest_amount_cents":     app.InterestAmountCents,
			"total_amount_cents":        app.TotalAmountCents,
			"monthly_payment_cents":     app.MonthlyPaymentCents,
			"status":                    app.Status,
			"created_at":                app.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"applications": appResponses,
		"count":        len(appResponses),
	})
}

// ============================================================
// HANDLERS : DOWN PAYMENT (CLIENT)
// ============================================================

// PayDownPayment permet au client de payer l'apport initial
// POST /api/credit/down-payment
func (h *CreditHandler) PayDownPayment(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req PayDownPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs
	if req.ContractID == "" || req.PaymentID == "" {
		utils.WriteError(w, http.StatusBadRequest, "contract_id and payment_id are required")
		return
	}

	// 4. Appeler le usecase
	ucReq := &creditusecase.PayDownPaymentRequest{
		ContractID: req.ContractID,
		PaymentID:  req.PaymentID,
	}

	resp, err := h.payDownUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to pay down payment")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to pay down payment: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("contract_id", req.ContractID).
		Int64("down_payment_cents", resp.DownPaymentCents).
		Msg("Down payment received")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Down payment received. Contract is now active. Installments schedule started.",
		"payment": resp,
	})
}

// ============================================================
// HANDLERS : CONTRACTS & INSTALLMENTS
// ============================================================

// GetContract retourne les détails d'un contrat
// GET /api/credit/contracts/{contract_id}
func (h *CreditHandler) GetContract(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer le contract_id
	contractID := chi.URLParam(r, "contract_id")
	if contractID == "" {
		utils.WriteError(w, http.StatusBadRequest, "contract_id is required in URL")
		return
	}

	// 3. Récupérer le contrat
	contract, err := h.approveUC.GetContract(r.Context(), contractID)
	if err != nil {
		logger.Error().Err(err).Str("contract_id", contractID).Msg("Contract not found")
		utils.WriteError(w, http.StatusNotFound, "Contract not found")
		return
	}

	// 4. Vérifier multi-tenant
	if contract.ShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied: contract does not belong to your shop")
		return
	}

	// 5. Récupérer les échéances
	installments, err := h.approveUC.GetContractInstallments(r.Context(), contractID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get installments")
		installments = []*entity.CreditInstallment{}
	}

	// 6. Construire la réponse
	installmentResponses := make([]map[string]interface{}, 0, len(installments))
	for _, inst := range installments {
		instResp := map[string]interface{}{
			"id":                 inst.ID,
			"installment_number": inst.InstallmentNumber,
			"due_date":           inst.DueDate.Format("2006-01-02"),
			"amount_cents":       inst.AmountCents,
			"amount_formatted":   formatMoney(inst.AmountCents),
			"status":             inst.Status,
			"late_fee_cents":     inst.LateFeeCents,
		}
		if inst.PaidAt != nil {
			instResp["paid_at"] = inst.PaidAt.Format("2006-01-02T15:04:05Z")
		}
		installmentResponses = append(installmentResponses, instResp)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"contract": map[string]interface{}{
			"id":                    contract.ID,
			"customer_id":           contract.CustomerID,
			"product_id":            contract.ProductID,
			"product_price_cents":   contract.ProductPriceCents,
			"down_payment_cents":    contract.DownPaymentCents,
			"financed_amount_cents": contract.FinancedAmountCents,
			"interest_amount_cents": contract.InterestAmountCents,
			"total_amount_cents":    contract.TotalAmountCents,
			"monthly_payment_cents": contract.MonthlyPaymentCents,
			"duration_months":       contract.DurationMonths,
			"start_date":            contract.StartDate.Format("2006-01-02"),
			"end_date":              contract.EndDate.Format("2006-01-02"),
			"status":                contract.Status,
		},
		"installments":       installmentResponses,
		"installments_count": len(installmentResponses),
	})
}

// GetContractStats retourne les statistiques d'un contrat
// GET /api/credit/contracts/{contract_id}/stats
func (h *CreditHandler) GetContractStats(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer le contract_id
	contractID := chi.URLParam(r, "contract_id")
	if contractID == "" {
		utils.WriteError(w, http.StatusBadRequest, "contract_id is required in URL")
		return
	}

	// 3. Récupérer les stats
	stats, err := h.approveUC.GetContractStats(r.Context(), contractID)
	if err != nil {
		logger.Error().Err(err).Str("contract_id", contractID).Msg("Failed to get stats")
		utils.WriteError(w, http.StatusNotFound, "Contract not found")
		return
	}

	// 4. Vérifier multi-tenant
	if contractShopID, ok := stats["shop_id"].(string); ok && contractShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied")
		return
	}

	utils.WriteJSON(w, http.StatusOK, stats)
}

// ListActiveContracts retourne les contrats actifs
// GET /api/credit/contracts/active
func (h *CreditHandler) ListActiveContracts(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les contrats actifs
	contracts, err := h.approveUC.GetActiveContractsByShop(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to list active contracts")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve active contracts")
		return
	}

	// 3. Construire la réponse
	contractResponses := make([]map[string]interface{}, 0, len(contracts))
	for _, contract := range contracts {
		contractResponses = append(contractResponses, map[string]interface{}{
			"id":                    contract.ID,
			"customer_id":           contract.CustomerID,
			"product_id":            contract.ProductID,
			"total_amount_cents":    contract.TotalAmountCents,
			"monthly_payment_cents": contract.MonthlyPaymentCents,
			"duration_months":       contract.DurationMonths,
			"start_date":            contract.StartDate.Format("2006-01-02"),
			"end_date":              contract.EndDate.Format("2006-01-02"),
			"status":                contract.Status,
		})
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"contracts": contractResponses,
		"count":     len(contractResponses),
	})
}

// ListOverdueInstallments retourne les échéances en retard
// GET /api/credit/installments/overdue
func (h *CreditHandler) ListOverdueInstallments(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Récupérer les échéances en retard
	installments, err := h.approveUC.GetOverdueInstallments(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list overdue installments")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve overdue installments")
		return
	}

	// 3. Construire la réponse
	installmentResponses := make([]map[string]interface{}, 0, len(installments))
	for _, inst := range installments {
		installmentResponses = append(installmentResponses, map[string]interface{}{
			"id":                 inst.ID,
			"contract_id":        inst.ContractID,
			"installment_number": inst.InstallmentNumber,
			"due_date":           inst.DueDate.Format("2006-01-02"),
			"amount_cents":       inst.AmountCents,
			"amount_formatted":   formatMoney(inst.AmountCents),
			"status":             inst.Status,
			"days_overdue":       inst.DaysOverdue(),
		})
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"installments": installmentResponses,
		"count":        len(installmentResponses),
	})
}

// ============================================================
// HELPERS
// ============================================================

// determineScoreLevel retourne le niveau de score en string
func determineScoreLevel(score int) string {
	switch {
	case score >= entity.ScoreExcellent:
		return "excellent"
	case score >= entity.ScoreGood:
		return "good"
	case score >= entity.ScoreMedium:
		return "medium"
	default:
		return "bad"
	}
}

// formatMoney formate un montant en centimes pour affichage
func formatMoney(cents int64) string {
	if cents < 0 {
		return "-" + formatMoney(-cents)
	}
	fcfa := cents / 100
	return formatNumber(fcfa) + " FCFA"
}

// formatNumber formate un nombre avec séparateurs de milliers
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	str := fmt.Sprintf("%d", n)
	result := ""
	count := 0

	for i := len(str) - 1; i >= 0; i-- {
		if count > 0 && count%3 == 0 {
			result = " " + result
		}
		result = string(str[i]) + result
		count++
	}

	return result
}

// ============================================================
// ROUTER SETUP
// ============================================================

// RegisterRoutes enregistre les routes du credit handler
func (h *CreditHandler) RegisterRoutes(r chi.Router) {
	// Plans de crédit (marchand)
	r.Post("/plans", h.ConfigureCreditPlan)
	r.Get("/plans", h.ListCreditPlans)
	r.Get("/plans/{product_id}", h.GetCreditPlan)

	// Demandes de crédit (client)
	r.Post("/apply", h.ApplyForCredit)
	r.Post("/simulate", h.SimulateCredit)
	r.Get("/eligibility/{customer_id}/{product_id}", h.CheckEligibility)
	r.Get("/score/{customer_id}", h.GetCustomerScore)

	// Approbation (marchand)
	r.Post("/approve", h.ApproveCredit)
	r.Post("/reject", h.RejectCredit)
	r.Get("/applications/pending", h.ListPendingApplications)

	// Paiement apport initial (client)
	r.Post("/down-payment", h.PayDownPayment)

	// Contrats et échéances
	r.Get("/contracts/active", h.ListActiveContracts)
	r.Get("/contracts/{contract_id}", h.GetContract)
	r.Get("/contracts/{contract_id}/stats", h.GetContractStats)
	r.Get("/installments/overdue", h.ListOverdueInstallments)
}
