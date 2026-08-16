package merchantkychandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"Goshop/application/metrics"
	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// MERCHANT KYC HANDLER
// ============================================================

type MerchantKYCHandler struct {
	submitUC *merchantkycusecase.SubmitMerchantKYCUsecase
	reviewUC *merchantkycusecase.ReviewMerchantKYCUsecase
	listUC   *merchantkycusecase.ListPendingMerchantKYCUsecase
	statusUC *merchantkycusecase.GetMerchantKYCStatusUsecase
}

func NewMerchantKYCHandler(
	submitUC *merchantkycusecase.SubmitMerchantKYCUsecase,
	reviewUC *merchantkycusecase.ReviewMerchantKYCUsecase,
	listUC *merchantkycusecase.ListPendingMerchantKYCUsecase,
	statusUC *merchantkycusecase.GetMerchantKYCStatusUsecase,
) *MerchantKYCHandler {
	return &MerchantKYCHandler{
		submitUC: submitUC,
		reviewUC: reviewUC,
		listUC:   listUC,
		statusUC: statusUC,
	}
}

// ============================================================
// REQUEST/RESPONSE TYPES
// ============================================================

type SubmitKYCRequest struct {
	Documents []DocumentInput `json:"documents"`
}

type DocumentInput struct {
	DocumentType  string `json:"document_type"`
	FilePath      string `json:"file_path"`
	FileName      string `json:"file_name"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	MimeType      string `json:"mime_type"`
}

type ReviewKYCRequest struct {
	Action          string                `json:"action"`
	RejectionReason *string               `json:"rejection_reason,omitempty"`
	DocumentReviews []DocumentReviewInput `json:"document_reviews,omitempty"`
}

type DocumentReviewInput struct {
	DocumentID      string  `json:"document_id"`
	Action          string  `json:"action"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// ============================================================
// ENDPOINTS MARCHAND
// ============================================================

// @Summary Soumettre une demande de KYC Marchand
// @Description Permet à un marchand de soumettre ses documents d'identité pour vérification.
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Param request body merchantkychandler.SubmitKYCRequest true "Liste des documents à soumettre"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou documents manquants"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/merchant/kyc/submit [post]
func (h *MerchantKYCHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	var req SubmitKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	if len(req.Documents) == 0 {
		return utils.NewAppError("VALIDATION_ERROR", "at least one document is required", http.StatusBadRequest)
	}

	ucDocs := make([]merchantkycusecase.DocumentInput, len(req.Documents))
	for i, doc := range req.Documents {
		ucDocs[i] = merchantkycusecase.DocumentInput{
			DocumentType:  doc.DocumentType,
			FilePath:      doc.FilePath,
			FileName:      doc.FileName,
			FileSizeBytes: doc.FileSizeBytes,
			MimeType:      doc.MimeType,
		}
	}

	ucReq := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: ucDocs,
	}

	response, err := h.submitUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec soumission
		metrics.MerchantKYCSubmitTotal.WithLabelValues("error").Inc()
		metrics.MerchantKYCOperationDuration.WithLabelValues("submit").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_submit", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Int("documents_count", len(req.Documents)).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur soumission KYC")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no documents") {
			return utils.NewAppError("KYC_NOT_FOUND", errMsg, http.StatusNotFound)
		}
		if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "pending") {
			return utils.NewAppError("KYC_INVALID_STATUS", errMsg, http.StatusBadRequest)
		}

		return utils.NewAppError("SUBMIT_KYC_FAILED", errMsg, http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès soumission
	metrics.MerchantKYCSubmitTotal.WithLabelValues("success").Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("submit").Observe(duration)
	metrics.MerchantKYCDocumentsPerSubmit.Observe(float64(len(req.Documents)))

	logger.Info().
		Int("documents_count", len(req.Documents)).
		Float64("duration_seconds", duration).
		Msg("✅ KYC submitted successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": response.Message,
		"data":    response,
	})

	return nil
}

// @Summary Obtenir le statut KYC du marchand
// @Description Retourne le statut actuel de la vérification KYC du marchand connecté.
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/merchant/kyc/status [get]
func (h *MerchantKYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	response, err := h.statusUC.Execute(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec status check
		metrics.MerchantKYCOperationDuration.WithLabelValues("get_status").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_status", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur récupération statut KYC")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		return utils.NewAppError("GET_KYC_STATUS_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès status check
	metrics.MerchantKYCStatusCheckTotal.Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("get_status").Observe(duration)

	logger.Info().
		Float64("duration_seconds", duration).
		Msg("✅ KYC status retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

// ============================================================
// ENDPOINTS ADMIN
// ============================================================

// @Summary Lister les demandes KYC marchand en attente (Admin)
// @Description Retourne la liste paginée des boutiques dont le KYC est en attente de validation par un administrateur.
// @Tags Admin Merchant KYC
// @Accept json
// @Produce json
// @Param limit query int false "Nombre de résultats (défaut: 20)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (droits insuffisants)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/merchant-kyc/pending [get]
func (h *MerchantKYCHandler) ListPendingKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	ucReq := &merchantkycusecase.ListPendingRequest{
		Limit:  limit,
		Offset: offset,
	}

	response, err := h.listUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec liste pending
		metrics.MerchantKYCOperationDuration.WithLabelValues("list_pending").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_list", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Int("limit", limit).
			Int("offset", offset).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur liste shops pending")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		return utils.NewAppError("LIST_PENDING_KYC_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès liste pending
	metrics.MerchantKYCPendingListTotal.Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("list_pending").Observe(duration)

	// ✅ EXTRACTION SÉCURISÉE du nombre de résultats
	// On tente d'accéder aux champs publics de ListPendingResponse
	pendingCount := 0
	if response != nil {
		// Tenter d'accéder au champ Shops (slice) ou Count (int) selon la structure
		// Utilisation de reflection pour accéder dynamiquement aux champs
		responseValue := reflect.ValueOf(response).Elem()

		// Chercher un champ "Shops" (slice)
		if shopsField := responseValue.FieldByName("Shops"); shopsField.IsValid() && shopsField.Kind() == reflect.Slice {
			pendingCount = shopsField.Len()
		} else if countField := responseValue.FieldByName("Count"); countField.IsValid() && countField.Kind() == reflect.Int {
			// Ou chercher un champ "Count" (int)
			pendingCount = int(countField.Int())
		} else if totalField := responseValue.FieldByName("Total"); totalField.IsValid() && totalField.Kind() == reflect.Int {
			// Ou chercher un champ "Total" (int)
			pendingCount = int(totalField.Int())
		}
	}

	metrics.MerchantKYCPendingCount.Observe(float64(pendingCount))

	logger.Info().
		Int("limit", limit).
		Int("offset", offset).
		Int("pending_count", pendingCount).
		Float64("duration_seconds", duration).
		Msg("✅ Pending KYC list retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

// @Summary Réviser une demande KYC marchand (Admin)
// @Description Permet à un administrateur d'approuver ou de rejeter les documents KYC soumis par une boutique.
// @Tags Admin Merchant KYC
// @Accept json
// @Produce json
// @Param shop_id path string true "ID de la boutique (UUID)"
// @Param request body merchantkychandler.ReviewKYCRequest true "Décision de révision (approve/reject) et motifs"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou action invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (droits insuffisants)"
// @Failure 404 {object} utils.AppError "Boutique ou demande KYC introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/merchant-kyc/{shop_id}/review [put]
func (h *MerchantKYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	shopID := chi.URLParam(r, "shop_id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	var req ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	if req.Action != "approve" && req.Action != "reject" {
		return utils.NewAppError("VALIDATION_ERROR", "action must be 'approve' or 'reject'", http.StatusBadRequest)
	}

	adminID, ok := utils.UserIDFromContext(ctx)
	if !ok || adminID == "" {
		return utils.ErrUnauthorized
	}

	docReviews := make([]merchantkycusecase.DocumentReview, len(req.DocumentReviews))
	for i, dr := range req.DocumentReviews {
		docReviews[i] = merchantkycusecase.DocumentReview{
			DocumentID:      dr.DocumentID,
			Action:          dr.Action,
			RejectionReason: dr.RejectionReason,
		}
	}

	ucReq := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:          shopID,
		Action:          req.Action,
		AdminID:         adminID,
		RejectionReason: req.RejectionReason,
		DocumentReviews: docReviews,
	}

	response, err := h.reviewUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec review
		metrics.MerchantKYCReviewTotal.WithLabelValues(req.Action, "error").Inc()
		metrics.MerchantKYCOperationDuration.WithLabelValues("review").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_review", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Str("action", req.Action).
			Str("admin_id", adminID).
			Int("documents_reviewed", len(docReviews)).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur revue KYC")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "not pending") ||
			strings.Contains(errMsg, "already verified") ||
			strings.Contains(errMsg, "already rejected") ||
			strings.Contains(errMsg, "unverified") ||
			strings.Contains(errMsg, "invalid status") {
			return utils.NewAppError("KYC_INVALID_STATUS", errMsg, http.StatusBadRequest)
		}
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no documents") {
			return utils.NewAppError("KYC_NOT_FOUND", errMsg, http.StatusNotFound)
		}

		return utils.NewAppError("REVIEW_KYC_FAILED", errMsg, http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès review
	metrics.MerchantKYCReviewTotal.WithLabelValues(req.Action, "success").Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("review").Observe(duration)

	logger.Info().
		Str("shop_id", shopID).
		Str("action", req.Action).
		Str("admin_id", adminID).
		Int("documents_reviewed", len(docReviews)).
		Float64("duration_seconds", duration).
		Msg("✅ KYC reviewed successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": response.Message,
		"data":    response,
	})

	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

func (h *MerchantKYCHandler) RegisterMerchantRoutes(r chi.Router) {
	r.Post("/submit", middl.ErrorHandler(h.SubmitKYC))
	r.Get("/status", middl.ErrorHandler(h.GetKYCStatus))
}

func (h *MerchantKYCHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/pending", middl.ErrorHandler(h.ListPendingKYC))
	r.Put("/{shop_id}/review", middl.ErrorHandler(h.ReviewKYC))
	r.Post("/{shop_id}/review", middl.ErrorHandler(h.ReviewKYC))
}
