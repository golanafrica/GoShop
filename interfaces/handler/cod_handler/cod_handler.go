package cod_handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// CODHandler gère les endpoints liés au Cash On Delivery
type CODHandler struct {
	submitClientUC   *codusecase.SubmitClientProofUsecase
	submitMerchantUC *codusecase.SubmitMerchantProofUsecase
	collectUC        *codusecase.CollectCommissionUsecase
	codProofRepo     repository.CODProofRepository
}

// NewCODHandler crée une nouvelle instance du handler
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

// SubmitClientProofRequest représente la requête pour soumettre une preuve client
type SubmitClientProofRequest struct {
	OrderID       string `json:"order_id"`
	CustomerID    string `json:"customer_id"`
	ProofURL      string `json:"proof_url"`
	AmountCents   int64  `json:"amount_cents"`
	PaymentDate   string `json:"payment_date"` // Format: "2006-01-02T15:04:05Z"
	ReceiptNumber string `json:"receipt_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

// SubmitMerchantProofRequest représente la requête pour soumettre une preuve marchand
type SubmitMerchantProofRequest struct {
	OrderID     string `json:"order_id"`
	ProofURL    string `json:"proof_url"`
	AmountCents int64  `json:"amount_cents"`
	ReceiptDate string `json:"receipt_date"` // Format: "2006-01-02T15:04:05Z"
	Notes       string `json:"notes,omitempty"`
}

// CollectCommissionRequest représente la requête pour collecter une commission
type CollectCommissionRequest struct {
	OrderID      string `json:"order_id"`
	ForceCollect bool   `json:"force_collect,omitempty"`
}

// CODProofResponse représente la réponse standard pour une preuve COD
type CODProofResponse struct {
	ProofID          string `json:"proof_id"`
	OrderID          string `json:"order_id"`
	ShopID           string `json:"shop_id"`
	CustomerID       string `json:"customer_id"`
	Status           string `json:"status"`
	CommissionCents  int64  `json:"commission_cents"`
	CommissionStatus string `json:"commission_status"`

	// Preuve client
	HasClientProof    bool   `json:"has_client_proof"`
	ClientProofURL    string `json:"client_proof_url,omitempty"`
	ClientAmountCents *int64 `json:"client_amount_cents,omitempty"`
	ClientPaymentDate string `json:"client_payment_date,omitempty"`
	ClientSubmittedAt string `json:"client_submitted_at,omitempty"`

	// Preuve marchand
	HasMerchantProof    bool   `json:"has_merchant_proof"`
	MerchantProofURL    string `json:"merchant_proof_url,omitempty"`
	MerchantAmountCents *int64 `json:"merchant_amount_cents,omitempty"`
	MerchantReceiptDate string `json:"merchant_receipt_date,omitempty"`
	MerchantSubmittedAt string `json:"merchant_submitted_at,omitempty"`

	// Cohérence
	AmountsMatch *bool `json:"amounts_match,omitempty"`
	DatesMatch   *bool `json:"dates_match,omitempty"`
	IsCoherent   bool  `json:"is_coherent"`

	// Délai
	Deadline       string `json:"deadline,omitempty"`
	DaysRemaining  int    `json:"days_remaining,omitempty"`
	IsPastDeadline bool   `json:"is_past_deadline"`
}

// ============================================================
// HANDLERS : GET PROOF
// ============================================================

// @Summary Récupérer une preuve COD
// @Description Retourne les détails d'une preuve de paiement Cash on Delivery pour une commande spécifique.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param order_id path string true "ID de la commande (UUID)"
// @Success 200 {object} cod_handler.CODProofResponse
// @Failure 400 {object} utils.AppError "ID de commande manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : la preuve n'appartient pas à votre boutique"
// @Failure 404 {object} utils.AppError "Preuve non trouvée pour cette commande"
// @Security ApiKeyAuth
// @Router /api/cod/proof/{order_id} [get]
func (h *CODHandler) GetCODProof(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'order_id depuis l'URL
	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		utils.WriteError(w, http.StatusBadRequest, "order_id is required in URL")
		return
	}

	// 3. Récupérer la preuve
	proof, err := h.codProofRepo.FindByOrderID(r.Context(), orderID)
	if err != nil {
		logger.Error().Err(err).Str("order_id", orderID).Msg("COD proof not found")
		utils.WriteError(w, http.StatusNotFound, "COD proof not found for this order")
		return
	}

	// 4. Vérifier multi-tenant
	if proof.ShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 5. Construire la réponse
	response := buildCODProofResponse(proof)

	utils.WriteJSON(w, http.StatusOK, response)
}

// @Summary Lister les preuves COD d'une boutique
// @Description Retourne la liste de toutes les preuves COD pour la boutique active, avec un filtre de statut optionnel.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param status query string false "Filtrer par statut (ex: pending_client, pending_merchant, collected)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Filtre de statut invalide"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/proofs [get]
func (h *CODHandler) ListCODProofsByShop(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Filtrer par statut (optionnel)
	statusFilter := r.URL.Query().Get("status")

	var proofs []*entity.CODProof
	if statusFilter != "" {
		status := entity.CODProofStatus(statusFilter)
		if !status.IsValid() {
			utils.WriteError(w, http.StatusBadRequest, "Invalid status filter")
			return
		}
		proofs, err = h.codProofRepo.FindByShopIDAndStatus(r.Context(), shopID, status)
	} else {
		proofs, err = h.codProofRepo.FindByShopID(r.Context(), shopID)
	}

	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to list COD proofs")
		utils.WriteError(w, http.StatusInternalServerError, "Failed to retrieve COD proofs")
		return
	}

	// 3. Construire la réponse
	responses := make([]CODProofResponse, 0, len(proofs))
	for _, proof := range proofs {
		responses = append(responses, *buildCODProofResponse(proof))
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"proofs": responses,
		"count":  len(responses),
	})
}

// ============================================================
// HANDLERS : SUBMIT PROOFS
// ============================================================

// @Summary Soumettre une preuve de paiement client
// @Description Permet à un client de soumettre sa preuve de paiement en espèces pour une commande COD.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param request body cod_handler.SubmitClientProofRequest true "Détails de la preuve client"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/client-proof [post]
func (h *CODHandler) SubmitClientProof(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	_, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}

	// 2. Parser la requête
	var req SubmitClientProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.OrderID == "" || req.CustomerID == "" || req.ProofURL == "" {
		utils.WriteError(w, http.StatusBadRequest, "order_id, customer_id, and proof_url are required")
		return
	}
	if req.AmountCents <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	if req.PaymentDate == "" {
		utils.WriteError(w, http.StatusBadRequest, "payment_date is required")
		return
	}

	// 4. Parser la date
	paymentDate, err := parseDate(req.PaymentDate)
	if err != nil {
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

	resp, err := h.submitClientUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to submit client proof")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit client proof: %v", err))
		return
	}

	// 7. Logger et retourner
	logger.Info().
		Str("order_id", req.OrderID).
		Str("customer_id", req.CustomerID).
		Str("proof_id", resp.ProofID).
		Msg("Client proof submitted successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Client proof submitted successfully",
		"proof":   resp,
	})
}

// @Summary Soumettre une preuve de réception marchand
// @Description Permet au marchand de confirmer la réception des fonds et de soumettre sa propre preuve de livraison.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param request body cod_handler.SubmitMerchantProofRequest true "Détails de la preuve marchand"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou champs manquants"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : la preuve n'appartient pas à votre boutique"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/merchant-proof [post]
func (h *CODHandler) SubmitMerchantProof(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Parser la requête
	var req SubmitMerchantProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider les champs obligatoires
	if req.OrderID == "" || req.ProofURL == "" {
		utils.WriteError(w, http.StatusBadRequest, "order_id and proof_url are required")
		return
	}
	if req.AmountCents <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	if req.ReceiptDate == "" {
		utils.WriteError(w, http.StatusBadRequest, "receipt_date is required")
		return
	}

	// 4. Parser la date
	receiptDate, err := parseDate(req.ReceiptDate)
	if err != nil {
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

	resp, err := h.submitMerchantUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to submit merchant proof")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to submit merchant proof: %v", err))
		return
	}

	// 7. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 8. Logger et retourner
	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Str("proof_id", resp.ProofID).
		Msg("Merchant proof submitted successfully")

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
// @Description Déclenche la collecte de la commission plateforme sur une vente COD dont les preuves sont cohérentes.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param request body cod_handler.CollectCommissionRequest true "Détails de la collecte"
// @Success 200 {object} codusecase.CollectCommissionResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou collecte impossible"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : la preuve n'appartient pas à votre boutique"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/collect [post]
func (h *CODHandler) CollectCommission(w http.ResponseWriter, r *http.Request) {
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
	var req CollectCommissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 4. Valider les champs
	if req.OrderID == "" {
		utils.WriteError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	// 5. Appeler le usecase
	ucReq := &codusecase.CollectCommissionRequest{
		OrderID:      req.OrderID,
		CollectedBy:  userID,
		ForceCollect: req.ForceCollect,
	}

	resp, err := h.collectUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to collect commission")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to collect commission: %v", err))
		return
	}

	// 6. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 7. Logger et retourner
	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Int64("commission_cents", resp.CommissionCents).
		Str("commission_status", string(resp.CommissionStatus)).
		Bool("account_frozen", resp.AccountFrozen).
		Msg("Commission collection processed")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Lister les commissions dues
// @Description Retourne la liste de toutes les commissions en attente de collecte pour la boutique active, avec le total dû.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/due [get]
func (h *CODHandler) ListDueCommissions(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les commissions dues
	proofs, err := h.codProofRepo.FindCommissionDueByShopID(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to list due commissions")
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

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"proofs":              responses,
		"count":               len(responses),
		"total_due_cents":     totalDueCents,
		"total_due_formatted": formatMoney(totalDueCents),
	})
}

// @Summary Retenter la collecte d'une commission
// @Description Force ou retente la collecte d'une commission due pour une commande spécifique.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param order_id path string true "ID de la commande (UUID)"
// @Success 200 {object} codusecase.CollectCommissionResponse
// @Failure 400 {object} utils.AppError "ID de commande manquant ou collecte impossible"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 403 {object} utils.AppError "Accès refusé : la preuve n'appartient pas à votre boutique"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/retry/{order_id} [post]
func (h *CODHandler) RetryCommission(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer l'order_id depuis l'URL
	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		utils.WriteError(w, http.StatusBadRequest, "order_id is required in URL")
		return
	}

	// 3. Récupérer l'user_id (pour audit)
	userID, _ := utils.UserIDFromContext(r.Context())
	if userID == "" {
		userID = shopID
	}

	// 4. Appeler le usecase
	resp, err := h.collectUC.RetryCommissionDue(r.Context(), orderID, userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to retry commission")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to retry commission: %v", err))
		return
	}

	// 5. Vérifier que la preuve appartient au shop
	if resp.ShopID != shopID {
		utils.WriteError(w, http.StatusForbidden, "Access denied: proof does not belong to your shop")
		return
	}

	// 6. Logger et retourner
	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shopID).
		Int64("commission_cents", resp.CommissionCents).
		Str("commission_status", string(resp.CommissionStatus)).
		Msg("Commission retry processed")

	utils.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Obtenir les statistiques des commissions COD
// @Description Retourne un résumé des commissions en attente, dues et collectées pour la boutique active.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/cod/stats [get]
func (h *CODHandler) GetCommissionStats(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer les statistiques
	pendingCents, err := h.codProofRepo.SumCommissionPendingByShopID(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get pending commissions")
		pendingCents = 0
	}

	dueCents, err := h.codProofRepo.SumCommissionDueByShopID(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get due commissions")
		dueCents = 0
	}

	collectedCents, err := h.codProofRepo.SumCommissionCollectedByShopID(r.Context(), shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get collected commissions")
		collectedCents = 0
	}

	pendingCount, err := h.codProofRepo.CountPendingByShopID(r.Context(), shopID)
	if err != nil {
		pendingCount = 0
	}

	confirmedCount, err := h.codProofRepo.CountConfirmedByShopID(r.Context(), shopID)
	if err != nil {
		confirmedCount = 0
	}

	disputedCount, err := h.codProofRepo.CountDisputedByShopID(r.Context(), shopID)
	if err != nil {
		disputedCount = 0
	}

	// 3. Construire la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"shop_id": shopID,
		"pending": map[string]interface{}{
			"count":           pendingCount,
			"total_cents":     pendingCents,
			"total_formatted": formatMoney(pendingCents),
		},
		"due": map[string]interface{}{
			"count":           0, // À calculer si besoin
			"total_cents":     dueCents,
			"total_formatted": formatMoney(dueCents),
		},
		"collected": map[string]interface{}{
			"count":           0, // À calculer si besoin
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

// buildCODProofResponse construit une réponse à partir d'une preuve
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

	// Infos client
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

	// Infos marchand
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

	// Cohérence
	if proof.AmountsMatch != nil {
		response.AmountsMatch = proof.AmountsMatch
	}
	if proof.DatesMatch != nil {
		response.DatesMatch = proof.DatesMatch
	}

	// Délai
	if !proof.CreatedAt.IsZero() {
		deadline := proof.ProofDeadline()
		response.Deadline = deadline.Format("2006-01-02T15:04:05Z")
		response.DaysRemaining = proof.DaysUntilDeadline()
	}

	return response
}

// parseDate parse une date au format ISO 8601
func parseDate(dateStr string) (time.Time, error) {
	// Essayer plusieurs formats
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

// RegisterRoutes enregistre les routes du COD handler
func (h *CODHandler) RegisterRoutes(r chi.Router) {
	// Informations
	r.Get("/proof/{order_id}", h.GetCODProof)
	r.Get("/proofs", h.ListCODProofsByShop)
	r.Get("/due", h.ListDueCommissions)
	r.Get("/stats", h.GetCommissionStats)

	// Soumission des preuves
	r.Post("/client-proof", h.SubmitClientProof)
	r.Post("/merchant-proof", h.SubmitMerchantProof)

	// Collecte des commissions
	r.Post("/collect", h.CollectCommission)
	r.Post("/retry/{order_id}", h.RetryCommission)
}
