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
	configureUC      *creditusecase.ConfigureCreditPlanUsecase
	applyUC          *creditusecase.ApplyForCreditUsecase
	approveUC        *creditusecase.ApproveCreditUsecase
	rejectUC         *creditusecase.RejectCreditUsecase
	payDownUC        *creditusecase.PayDownPaymentUsecase
	payInstallmentUC *creditusecase.PayInstallmentUsecase // 🆕 AJOUT
}

// NewCreditHandler crée une nouvelle instance du handler
func NewCreditHandler(
	configureUC *creditusecase.ConfigureCreditPlanUsecase,
	applyUC *creditusecase.ApplyForCreditUsecase,
	approveUC *creditusecase.ApproveCreditUsecase,
	rejectUC *creditusecase.RejectCreditUsecase,
	payDownUC *creditusecase.PayDownPaymentUsecase,
	payInstallmentUC *creditusecase.PayInstallmentUsecase, // 🆕 AJOUT
) *CreditHandler {
	return &CreditHandler{
		configureUC:      configureUC,
		applyUC:          applyUC,
		approveUC:        approveUC,
		rejectUC:         rejectUC,
		payDownUC:        payDownUC,
		payInstallmentUC: payInstallmentUC, // 🆕 AJOUT
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

// @Summary Configurer un plan de crédit pour un produit
// @Description Permet à un marchand d'activer ou de mettre à jour les conditions de crédit pour un produit spécifique.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body credit_handler.ConfigurePlanRequest true "Détails du plan de crédit"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou données manquantes"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/plans [post]
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

// @Summary Récupérer le plan de crédit d'un produit
// @Description Retourne les détails de la configuration de crédit pour un produit spécifique.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param product_id path string true "ID du produit (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "ID de produit manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 404 {object} utils.AppError "Plan de crédit non trouvé"
// @Security ApiKeyAuth
// @Router /api/credit/plans/{product_id} [get]
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

// @Summary Lister les plans de crédit d'une boutique
// @Description Retourne la liste de tous les plans de crédit configurés pour la boutique active.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param enabled query boolean false "Filtrer uniquement les plans activés (true/false)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/plans [get]
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

// @Summary Demander un crédit
// @Description Permet à un client de soumettre une demande de financement pour un produit spécifique.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body credit_handler.ApplyForCreditRequest true "Détails de la demande de crédit"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou éligibilité non remplie"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/apply [post]
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

// @Summary Simuler un crédit
// @Description Calcule les mensualités et le coût total d'un crédit sans créer de demande officielle.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body credit_handler.SimulateCreditRequest true "Paramètres de la simulation"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou produit non éligible"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/simulate [post]
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

// @Summary Vérifier l'éligibilité au crédit
// @Description Vérifie si un client spécifique est éligible pour demander un crédit sur un produit donné.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param customer_id path string true "ID du client (UUID)"
// @Param product_id path string true "ID du produit (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "IDs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/eligibility/{customer_id}/{product_id} [get]
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

// @Summary Obtenir le score de crédit d'un client
// @Description Retourne le score de fiabilité actuel d'un client et son historique de paiement.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param customer_id path string true "ID du client (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "ID client manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/score/{customer_id} [get]
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

// @Summary Approuver une demande de crédit
// @Description Permet au marchand d'accepter une demande de crédit, ce qui génère le contrat et les échéances.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body credit_handler.ApproveCreditRequest true "ID de la demande à approuver"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou demande introuvable"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/approve [post]
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

// @Summary Rejeter une demande de crédit
// @Description Permet au marchand de refuser une demande de crédit avec un motif obligatoire.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body credit_handler.RejectCreditRequest true "ID de la demande et motif du rejet"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou motif manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/reject [post]
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

// @Summary Lister les demandes de crédit en attente
// @Description Retourne la liste de toutes les demandes de crédit nécessitant une validation du marchand.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/applications/pending [get]
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
			"product_price_formatted":   utils.FormatMoney(app.ProductPriceCents),
			"down_payment_cents":        app.DownPaymentCents,
			"down_payment_formatted":    utils.FormatMoney(app.DownPaymentCents),
			"financed_amount_cents":     app.FinancedAmountCents,
			"financed_amount_formatted": utils.FormatMoney(app.FinancedAmountCents),
			"interest_amount_cents":     app.InterestAmountCents,
			"interest_amount_formatted": utils.FormatMoney(app.InterestAmountCents),
			"total_amount_cents":        app.TotalAmountCents,
			"total_amount_formatted":    utils.FormatMoney(app.TotalAmountCents),
			"monthly_payment_cents":     app.MonthlyPaymentCents,
			"monthly_payment_formatted": utils.FormatMoney(app.MonthlyPaymentCents),
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

// @Summary Payer l'apport initial d'un crédit
// @Description Permet au client d'initier le paiement de l'apport obligatoire pour activer son contrat de crédit.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param request body creditusecase.PayDownPaymentRequest true "Détails du paiement de l'apport"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou contrat non éligible"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/down-payment [post]
func (h *CreditHandler) PayDownPayment(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req creditusecase.PayDownPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Appeler le usecase sécurisé
	resp, err := h.payDownUC.Execute(r.Context(), &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initiate down payment")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to initiate payment: %v", err))
		return
	}

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": resp.Message,
		"data":    resp,
	})
}

// ============================================================
// 🆕 HANDLERS : INSTALLMENT PAYMENT (CLIENT)
// ============================================================

// @Summary Payer une échéance de crédit
// @Description Permet au client d'initier manuellement le paiement d'une échéance spécifique de son contrat.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param installment_id path string true "ID de l'échéance à payer (UUID)"
// @Param request body creditusecase.PayInstallmentRequest true "Détails du paiement"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou échéance non payable"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/installments/{installment_id}/pay [post]
func (h *CreditHandler) PayInstallment(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Récupérer l'installment_id depuis l'URL
	installmentID := chi.URLParam(r, "installment_id")
	if installmentID == "" {
		utils.WriteError(w, http.StatusBadRequest, "installment_id is required in URL")
		return
	}

	// 3. Parser la requête
	var req creditusecase.PayInstallmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	req.InstallmentID = installmentID // Forcer l'ID depuis l'URL

	// 4. Appeler le usecase
	resp, err := h.payInstallmentUC.Execute(r.Context(), &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initiate installment payment")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to initiate payment: %v", err))
		return
	}

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": resp.Message,
		"data":    resp,
	})
}

// ============================================================
// HANDLERS : CONTRACTS & INSTALLMENTS
// ============================================================

// @Summary Obtenir les détails d'un contrat de crédit
// @Description Retourne les informations complètes d'un contrat, y compris la liste de toutes ses échéances.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param contract_id path string true "ID du contrat (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "ID de contrat manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : le contrat n'appartient pas à votre boutique"
// @Failure 404 {object} utils.AppError "Contrat introuvable"
// @Security ApiKeyAuth
// @Router /api/credit/contracts/{contract_id} [get]
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
			"amount_formatted":   utils.FormatMoney(inst.AmountCents), // 🆕 v3.4.1
			"status":             inst.Status,
			"late_fee_cents":     inst.LateFeeCents,
			"late_fee_formatted": utils.FormatMoney(inst.LateFeeCents), // 🆕 v3.4.1
		}
		if inst.PaidAt != nil {
			instResp["paid_at"] = inst.PaidAt.Format("2006-01-02T15:04:05Z")
		}
		installmentResponses = append(installmentResponses, instResp)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"contract": map[string]interface{}{
			"id":                        contract.ID,
			"customer_id":               contract.CustomerID,
			"product_id":                contract.ProductID,
			"product_price_cents":       contract.ProductPriceCents,
			"product_price_formatted":   utils.FormatMoney(contract.ProductPriceCents), // 🆕 v3.4.1
			"down_payment_cents":        contract.DownPaymentCents,
			"down_payment_formatted":    utils.FormatMoney(contract.DownPaymentCents), // 🆕 v3.4.1
			"financed_amount_cents":     contract.FinancedAmountCents,
			"financed_amount_formatted": utils.FormatMoney(contract.FinancedAmountCents), // 🆕 v3.4.1
			"interest_amount_cents":     contract.InterestAmountCents,
			"interest_amount_formatted": utils.FormatMoney(contract.InterestAmountCents), // 🆕 v3.4.1
			"total_amount_cents":        contract.TotalAmountCents,
			"total_amount_formatted":    utils.FormatMoney(contract.TotalAmountCents), // 🆕 v3.4.1
			"monthly_payment_cents":     contract.MonthlyPaymentCents,
			"monthly_payment_formatted": utils.FormatMoney(contract.MonthlyPaymentCents), // 🆕 v3.4.1
			"duration_months":           contract.DurationMonths,
			"start_date":                contract.StartDate.Format("2006-01-02"),
			"end_date":                  contract.EndDate.Format("2006-01-02"),
			"status":                    contract.Status,
		},
		"installments":       installmentResponses,
		"installments_count": len(installmentResponses),
	})
}

// @Summary Obtenir les statistiques d'un contrat
// @Description Retourne un résumé financier et l'état d'avancement d'un contrat de crédit spécifique.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Param contract_id path string true "ID du contrat (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "ID de contrat manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 404 {object} utils.AppError "Contrat introuvable"
// @Security ApiKeyAuth
// @Router /api/credit/contracts/{contract_id}/stats [get]
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

// @Summary Lister les contrats de crédit actifs
// @Description Retourne la liste de tous les contrats de crédit actuellement en cours pour la boutique.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/contracts/active [get]
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
			"id":                        contract.ID,
			"customer_id":               contract.CustomerID,
			"product_id":                contract.ProductID,
			"total_amount_cents":        contract.TotalAmountCents,
			"total_amount_formatted":    utils.FormatMoney(contract.TotalAmountCents), // 🆕 v3.4.1
			"monthly_payment_cents":     contract.MonthlyPaymentCents,
			"monthly_payment_formatted": utils.FormatMoney(contract.MonthlyPaymentCents), // 🆕 v3.4.1
			"duration_months":           contract.DurationMonths,
			"start_date":                contract.StartDate.Format("2006-01-02"),
			"end_date":                  contract.EndDate.Format("2006-01-02"),
			"status":                    contract.Status,
		})
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"contracts": contractResponses,
		"count":     len(contractResponses),
	})
}

// @Summary Lister les échéances de crédit en retard
// @Description Retourne la liste de toutes les échéances non payées dont la date d'échéance est dépassée.
// @Tags Credit Management
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/credit/installments/overdue [get]
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
			"amount_formatted":   utils.FormatMoney(inst.AmountCents), // 🆕 v3.4.1
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

// 🆕 v3.4.1 : Utiliser le helper centralisé utils.FormatMoney
// formatMoney formate un montant en centimes pour affichage avec arrondi
// Exemple : 5499 centimes → "55 FCFA"
func formatMoney(cents int64) string {
	return utils.FormatMoney(cents)
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

	// 🆕 AJOUT : Paiement d'échéance (client)
	r.Post("/installments/{installment_id}/pay", h.PayInstallment)

	// Contrats et échéances
	r.Get("/contracts/active", h.ListActiveContracts)
	r.Get("/contracts/{contract_id}", h.GetContract)
	r.Get("/contracts/{contract_id}/stats", h.GetContractStats)
	r.Get("/installments/overdue", h.ListOverdueInstallments)
}
