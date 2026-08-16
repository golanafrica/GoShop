package codhandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"Goshop/application/metrics"
	codusecase "Goshop/application/usecase/cod_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// COD HANDLER
// ============================================================

type CODHandler struct {
	submitClientUC   *codusecase.SubmitClientProofUsecase
	submitMerchantUC *codusecase.SubmitMerchantProofUsecase
	collectUC        *codusecase.CollectCommissionUsecase
	codProofRepo     repository.CODProofRepository
}

func NewCODHandler(
	submitClientUC *codusecase.SubmitClientProofUsecase,
	submitMerchantUC *codusecase.SubmitMerchantProofUsecase,
	collectUC *codusecase.CollectCommissionUsecase,
	codProofRepo repository.CODProofRepository,
) *CODHandler {
	return &CODHandler{
		submitClientUC:   submitClientUC,
		submitMerchantUC: submitMerchantUC,
		collectUC:        collectUC,
		codProofRepo:     codProofRepo,
	}
}

// ============================================================
// REQUEST/RESPONSE TYPES
// ============================================================

type SubmitClientProofRequest struct {
	OrderID       string `json:"order_id"`
	CustomerID    string `json:"customer_id"`
	ProofURL      string `json:"proof_url"`
	AmountCents   int64  `json:"amount_cents"`
	PaymentDate   string `json:"payment_date"`
	ReceiptNumber string `json:"receipt_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

type SubmitMerchantProofRequest struct {
	OrderID     string `json:"order_id"`
	ProofURL    string `json:"proof_url"`
	AmountCents int64  `json:"amount_cents"`
	ReceiptDate string `json:"receipt_date"`
	Notes       string `json:"notes,omitempty"`
}

type CollectCommissionRequest struct {
	OrderID      string `json:"order_id"`
	ForceCollect bool   `json:"force_collect,omitempty"`
}

type CODProofResponse struct {
	ProofID          string `json:"proof_id"`
	OrderID          string `json:"order_id"`
	ShopID           string `json:"shop_id"`
	CustomerID       string `json:"customer_id"`
	Status           string `json:"status"`
	CommissionCents  int64  `json:"commission_cents"`
	CommissionStatus string `json:"commission_status"`

	HasClientProof    bool   `json:"has_client_proof"`
	ClientProofURL    string `json:"client_proof_url,omitempty"`
	ClientAmountCents *int64 `json:"client_amount_cents,omitempty"`
	ClientPaymentDate string `json:"client_payment_date,omitempty"`
	ClientSubmittedAt string `json:"client_submitted_at,omitempty"`

	HasMerchantProof    bool   `json:"has_merchant_proof"`
	MerchantProofURL    string `json:"merchant_proof_url,omitempty"`
	MerchantAmountCents *int64 `json:"merchant_amount_cents,omitempty"`
	MerchantReceiptDate string `json:"merchant_receipt_date,omitempty"`
	MerchantSubmittedAt string `json:"merchant_submitted_at,omitempty"`

	AmountsMatch *bool `json:"amounts_match,omitempty"`
	DatesMatch   *bool `json:"dates_match,omitempty"`
	IsCoherent   bool  `json:"is_coherent"`

	Deadline       string `json:"deadline,omitempty"`
	DaysRemaining  int    `json:"days_remaining,omitempty"`
	IsPastDeadline bool   `json:"is_past_deadline"`
}

// ============================================================
// HANDLERS : GET PROOF
// ============================================================

// @Summary Récupérer une preuve COD
func (h *CODHandler) GetCODProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur tenant
		metrics.CODProofTenantErrors.Inc()
		metrics.CODProofGetTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'order_id depuis l'URL
	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		metrics.CODProofGetTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "order_id is required in URL")
		return
	}

	// 3. Récupérer la preuve
	proof, err := h.codProofRepo.FindByOrderID(ctx, orderID)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODProofGetTotal.WithLabelValues("not_found").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("get_proof").Observe(duration)

		logger.Error().Err(err).
			Str("order_id", orderID).
			Float64("duration_seconds", duration).
			Msg("COD proof not found")

		utils.WriteError(w, http.StatusNotFound, "COD proof not found for this order")
		return
	}

	// 4. Vérifier multi-tenant
	if proof.ShopID != shopID {
		metrics.CODProofGetTotal.WithLabelValues("forbidden").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("get_proof").Observe(duration)
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 5. Construire la réponse
	response := buildCODProofResponse(proof)

	// 📊 MÉTRIQUES : Succès
	metrics.CODProofGetTotal.WithLabelValues("success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("get_proof").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shopID).
		Float64("duration_seconds", duration).
		Msg("✅ COD proof retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
}

// @Summary Lister les preuves COD d'une boutique
func (h *CODHandler) ListCODProofsByShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODProofListTotal.WithLabelValues("error", "false").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Filtrer par statut (optionnel)
	statusFilter := r.URL.Query().Get("status")
	hasFilter := statusFilter != ""

	var proofs []*entity.CODProof
	if hasFilter {
		status := entity.CODProofStatus(statusFilter)
		if !status.IsValid() {
			metrics.CODProofListTotal.WithLabelValues("invalid_filter", "true").Inc()
			utils.WriteError(w, http.StatusBadRequest, "Invalid status filter")
			return
		}
		proofs, err = h.codProofRepo.FindByShopIDAndStatus(ctx, shopID, status)
	} else {
		proofs, err = h.codProofRepo.FindByShopID(ctx, shopID)
	}

	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		hasFilterStr := "false"
		if hasFilter {
			hasFilterStr = "true"
		}
		metrics.CODProofListTotal.WithLabelValues("error", hasFilterStr).Inc()
		metrics.CODProofOperationDuration.WithLabelValues("list_proofs").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_proof_list", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("Failed to list COD proofs")

		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve COD proofs")
		return
	}

	// 3. Construire la réponse
	responses := make([]CODProofResponse, 0, len(proofs))
	for _, proof := range proofs {
		responses = append(responses, *buildCODProofResponse(proof))
	}

	// 📊 MÉTRIQUES : Succès
	hasFilterStr := "false"
	if hasFilter {
		hasFilterStr = "true"
	}
	metrics.CODProofListTotal.WithLabelValues("success", hasFilterStr).Inc()
	metrics.CODProofOperationDuration.WithLabelValues("list_proofs").Observe(duration)

	logger.Info().
		Str("shop_id", shopID).
		Int("proofs_count", len(responses)).
		Bool("has_filter", hasFilter).
		Float64("duration_seconds", duration).
		Msg("✅ COD proofs listed successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"proofs": responses,
		"count":  len(responses),
	})
}

// ============================================================
// HANDLERS : SUBMIT PROOFS
// ============================================================

// @Summary Soumettre une preuve de paiement client
func (h *CODHandler) SubmitClientProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	_, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODProofSubmitTotal.WithLabelValues("client", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req SubmitClientProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.CODProofPayloadErrors.Inc()
		metrics.CODProofSubmitTotal.WithLabelValues("client", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.OrderID == "" || req.CustomerID == "" || req.ProofURL == "" {
		metrics.CODProofSubmitTotal.WithLabelValues("client", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "order_id, customer_id, and proof_url are required")
		return
	}
	if req.AmountCents <= 0 {
		metrics.CODProofSubmitTotal.WithLabelValues("client", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	if req.PaymentDate == "" {
		metrics.CODProofSubmitTotal.WithLabelValues("client", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "payment_date is required")
		return
	}

	// 4. Parser la date
	paymentDate, err := parseDate(req.PaymentDate)
	if err != nil {
		metrics.CODProofSubmitTotal.WithLabelValues("client", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid payment_date format: %v", err))
		return
	}

	// 5. Construire les pointeurs optionnels
	var receiptNumber, notes *string
	if req.ReceiptNumber != "" {
		receiptNumber = &req.ReceiptNumber
	}
	if req.Notes != "" {
		notes = &req.Notes
	}

	// 6. Appeler le usecase
	ucReq := &codusecase.SubmitClientProofRequest{
		OrderID:       req.OrderID,
		CustomerID:    req.CustomerID,
		ProofURL:      req.ProofURL,
		AmountCents:   req.AmountCents,
		PaymentDate:   paymentDate,
		ReceiptNumber: receiptNumber,
		Notes:         notes,
	}

	resp, err := h.submitClientUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODProofSubmitTotal.WithLabelValues("client", "error").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("submit_client_proof").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_submit_client_proof", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", req.OrderID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit client proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit client proof: %v", err))
		return
	}

	// 📊 MÉTRIQUES : Succès
	metrics.CODProofSubmitTotal.WithLabelValues("client", "success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("submit_client_proof").Observe(duration)

	logger.Info().
		Str("order_id", req.OrderID).
		Str("customer_id", req.CustomerID).
		Str("proof_id", resp.ProofID).
		Int64("amount_cents", req.AmountCents).
		Float64("duration_seconds", duration).
		Msg("✅ Client proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Client proof submitted successfully",
		"proof":   resp,
	})
}

// @Summary Soumettre une preuve de réception marchand
func (h *CODHandler) SubmitMerchantProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Parser la requête
	var req SubmitMerchantProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.CODProofPayloadErrors.Inc()
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.OrderID == "" || req.ProofURL == "" {
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "order_id and proof_url are required")
		return
	}
	if req.AmountCents <= 0 {
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	if req.ReceiptDate == "" {
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "receipt_date is required")
		return
	}

	// 4. Parser la date
	receiptDate, err := parseDate(req.ReceiptDate)
	if err != nil {
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid receipt_date format: %v", err))
		return
	}

	// 5. Construire les pointeurs optionnels
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	// 6. Appeler le usecase
	ucReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     req.OrderID,
		ProofURL:    req.ProofURL,
		AmountCents: req.AmountCents,
		ReceiptDate: receiptDate,
		Notes:       notes,
	}

	resp, err := h.submitMerchantUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "error").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("submit_merchant_proof").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_submit_merchant_proof", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", req.OrderID).
			Float64("duration_seconds", duration).
			Msg("Failed to submit merchant proof")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit merchant proof: %v", err))
		return
	}

	// 7. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		metrics.CODProofSubmitTotal.WithLabelValues("merchant", "forbidden").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("submit_merchant_proof").Observe(duration)
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 📊 MÉTRIQUES : Succès
	metrics.CODProofSubmitTotal.WithLabelValues("merchant", "success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("submit_merchant_proof").Observe(duration)

	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Str("proof_id", resp.ProofID).
		Int64("amount_cents", req.AmountCents).
		Float64("duration_seconds", duration).
		Msg("✅ Merchant proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Merchant proof submitted successfully",
		"proof":   resp,
	})
}

// ============================================================
// HANDLERS : COLLECT COMMISSION
// ============================================================

// @Summary Collecter la commission GoShop
func (h *CODHandler) CollectCommission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODCommissionCollectTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'user_id (pour audit)
	userID, _ := utils.UserIDFromContext(ctx)
	if userID == "" {
		userID = shopID
	}

	// 3. Parser la requête
	var req CollectCommissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.CODProofPayloadErrors.Inc()
		metrics.CODCommissionCollectTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 4. Valider les champs
	if req.OrderID == "" {
		metrics.CODCommissionCollectTotal.WithLabelValues("validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	// 5. Appeler le usecase
	ucReq := &codusecase.CollectCommissionRequest{
		OrderID:      req.OrderID,
		CollectedBy:  userID,
		ForceCollect: req.ForceCollect,
	}

	resp, err := h.collectUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODCommissionCollectTotal.WithLabelValues("error").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("collect_commission").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_collect_commission", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", req.OrderID).
			Float64("duration_seconds", duration).
			Msg("Failed to collect commission")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to collect commission: %v", err))
		return
	}

	// 6. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		metrics.CODCommissionCollectTotal.WithLabelValues("forbidden").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("collect_commission").Observe(duration)
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 📊 MÉTRIQUES : Succès
	metrics.CODCommissionCollectTotal.WithLabelValues("success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("collect_commission").Observe(duration)
	metrics.CODCommissionAmountCents.Observe(float64(resp.CommissionCents))

	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Int64("commission_cents", resp.CommissionCents).
		Str("commission_status", string(resp.CommissionStatus)).
		Bool("account_frozen", resp.AccountFrozen).
		Float64("duration_seconds", duration).
		Msg("✅ Commission collection processed")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Lister les commissions dues
func (h *CODHandler) ListDueCommissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODDueCommissionsTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les commissions dues
	proofs, err := h.codProofRepo.FindCommissionDueByShopID(ctx, shopID)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODDueCommissionsTotal.WithLabelValues("error").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("list_due_commissions").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_list_due_commissions", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("Failed to list due commissions")

		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve due commissions")
		return
	}

	// 3. Calculer le total
	var totalDueCents int64
	responses := make([]CODProofResponse, 0, len(proofs))
	for _, proof := range proofs {
		responses = append(responses, *buildCODProofResponse(proof))
		totalDueCents += proof.CommissionCents
	}

	// 📊 MÉTRIQUES : Succès
	metrics.CODDueCommissionsTotal.WithLabelValues("success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("list_due_commissions").Observe(duration)

	logger.Info().
		Str("shop_id", shopID).
		Int("due_commissions_count", len(responses)).
		Int64("total_due_cents", totalDueCents).
		Float64("duration_seconds", duration).
		Msg("✅ Due commissions listed successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"proofs":              responses,
		"count":               len(responses),
		"total_due_cents":     totalDueCents,
		"total_due_formatted": formatMoney(totalDueCents),
	})
}

// @Summary Retenter la collecte d'une commission
func (h *CODHandler) RetryCommission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODCommissionRetryTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'order_id depuis l'URL
	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		metrics.CODCommissionRetryTotal.WithLabelValues("validation_error").Inc()
		utils.WriteError(w, http.StatusBadRequest, "order_id is required in URL")
		return
	}

	// 3. Récupérer l'user_id (pour audit)
	userID, _ := utils.UserIDFromContext(ctx)
	if userID == "" {
		userID = shopID
	}

	// 4. Appeler le usecase
	resp, err := h.collectUC.RetryCommissionDue(ctx, orderID, userID)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec
		metrics.CODCommissionRetryTotal.WithLabelValues("error").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("retry_commission").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_retry_commission", "cod_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", orderID).
			Float64("duration_seconds", duration).
			Msg("Failed to retry commission")

		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to retry commission: %v", err))
		return
	}

	// 5. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		metrics.CODCommissionRetryTotal.WithLabelValues("forbidden").Inc()
		metrics.CODProofOperationDuration.WithLabelValues("retry_commission").Observe(duration)
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 📊 MÉTRIQUES : Succès
	metrics.CODCommissionRetryTotal.WithLabelValues("success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("retry_commission").Observe(duration)
	metrics.CODCommissionAmountCents.Observe(float64(resp.CommissionCents))

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shopID).
		Int64("commission_cents", resp.CommissionCents).
		Str("commission_status", string(resp.CommissionStatus)).
		Float64("duration_seconds", duration).
		Msg("✅ Commission retry processed")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Obtenir les statistiques des commissions COD
func (h *CODHandler) GetCommissionStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		metrics.CODProofTenantErrors.Inc()
		metrics.CODCommissionStatsTotal.WithLabelValues("error").Inc()
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les statistiques (plusieurs requêtes DB)
	pendingCents, err := h.codProofRepo.SumCommissionPendingByShopID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get pending commissions")
		pendingCents = 0
	}

	dueCents, err := h.codProofRepo.SumCommissionDueByShopID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get due commissions")
		dueCents = 0
	}

	collectedCents, err := h.codProofRepo.SumCommissionCollectedByShopID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get collected commissions")
		collectedCents = 0
	}

	pendingCount, err := h.codProofRepo.CountPendingByShopID(ctx, shopID)
	if err != nil {
		pendingCount = 0
	}

	confirmedCount, err := h.codProofRepo.CountConfirmedByShopID(ctx, shopID)
	if err != nil {
		confirmedCount = 0
	}

	disputedCount, err := h.codProofRepo.CountDisputedByShopID(ctx, shopID)
	if err != nil {
		disputedCount = 0
	}

	duration := time.Since(start).Seconds()

	// 📊 MÉTRIQUES : Succès
	metrics.CODCommissionStatsTotal.WithLabelValues("success").Inc()
	metrics.CODProofOperationDuration.WithLabelValues("get_commission_stats").Observe(duration)

	logger.Info().
		Str("shop_id", shopID).
		Int64("pending_cents", pendingCents).
		Int64("due_cents", dueCents).
		Int64("collected_cents", collectedCents).
		Float64("duration_seconds", duration).
		Msg("✅ Commission stats retrieved successfully")

	// 3. Construire la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"shop_id": shopID,
		"pending": map[string]interface{}{
			"count":           pendingCount,
			"total_cents":     pendingCents,
			"total_formatted": formatMoney(pendingCents),
		},
		"due": map[string]interface{}{
			"count":           0,
			"total_cents":     dueCents,
			"total_formatted": formatMoney(dueCents),
		},
		"collected": map[string]interface{}{
			"count":           0,
			"total_cents":     collectedCents,
			"total_formatted": formatMoney(collectedCents),
		},
		"by_status": map[string]interface{}{
			"pending":   pendingCount,
			"confirmed": confirmedCount,
			"disputed":  disputedCount,
		},
	})
}

// ============================================================
// HELPERS
// ============================================================

func buildCODProofResponse(proof *entity.CODProof) *CODProofResponse {
	response := &CODProofResponse{
		ProofID:          proof.ID,
		OrderID:          proof.OrderID,
		ShopID:           proof.ShopID,
		CustomerID:       proof.CustomerID,
		Status:           string(proof.Status),
		CommissionCents:  proof.CommissionCents,
		CommissionStatus: string(proof.CommissionStatus),
		HasClientProof:   proof.HasClientProof(),
		HasMerchantProof: proof.HasMerchantProof(),
		IsCoherent:       proof.IsCoherent(),
		IsPastDeadline:   proof.IsPastDeadline(),
	}

	if proof.HasClientProof() {
		if proof.ClientPaymentProofURL != nil {
			response.ClientProofURL = *proof.ClientPaymentProofURL
		}
		response.ClientAmountCents = proof.ClientPaymentAmountCents
		if proof.ClientPaymentDate != nil {
			response.ClientPaymentDate = proof.ClientPaymentDate.Format("2006-01-02T15:04:05Z")
		}
		if proof.ClientSubmittedAt != nil {
			response.ClientSubmittedAt = proof.ClientSubmittedAt.Format("2006-01-02T15:04:05Z")
		}
	}

	if proof.HasMerchantProof() {
		if proof.MerchantReceiptProofURL != nil {
			response.MerchantProofURL = *proof.MerchantReceiptProofURL
		}
		response.MerchantAmountCents = proof.MerchantReceivedAmountCents
		if proof.MerchantReceiptDate != nil {
			response.MerchantReceiptDate = proof.MerchantReceiptDate.Format("2006-01-02T15:04:05Z")
		}
		if proof.MerchantSubmittedAt != nil {
			response.MerchantSubmittedAt = proof.MerchantSubmittedAt.Format("2006-01-02T15:04:05Z")
		}
	}

	if proof.AmountsMatch != nil {
		response.AmountsMatch = proof.AmountsMatch
	}
	if proof.DatesMatch != nil {
		response.DatesMatch = proof.DatesMatch
	}

	if !proof.CreatedAt.IsZero() {
		deadline := proof.ProofDeadline()
		response.Deadline = deadline.Format("2006-01-02T15:04:05Z")
		response.DaysRemaining = proof.DaysUntilDeadline()
	}

	return response
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
}

func formatMoney(cents int64) string {
	if cents < 0 {
		return "-" + formatMoney(-cents)
	}
	fcfa := cents / 100
	return formatNumber(fcfa) + " FCFA"
}

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

func (h *CODHandler) RegisterRoutes(r chi.Router) {
	r.Get("/proof/{order_id}", h.GetCODProof)
	r.Get("/proofs", h.ListCODProofsByShop)
	r.Get("/due", h.ListDueCommissions)
	r.Get("/stats", h.GetCommissionStats)

	r.Post("/client-proof", h.SubmitClientProof)
	r.Post("/merchant-proof", h.SubmitMerchantProof)

	r.Post("/collect", h.CollectCommission)
	r.Post("/retry/{order_id}", h.RetryCommission)
}
