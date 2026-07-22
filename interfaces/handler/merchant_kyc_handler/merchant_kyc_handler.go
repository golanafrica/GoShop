package merchantkychandler

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.submitUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur soumission KYC")
		return err
	}

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
	logger := zerolog.Ctx(r.Context())

	response, err := h.statusUC.Execute(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération statut KYC")
		return err
	}

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
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.listUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur liste shops pending")
		return err
	}

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
	logger := zerolog.Ctx(r.Context())

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

	// ✅ FIX : Valider l'action avant d'appeler le usecase pour éviter une erreur 500 "Unhandled"
	if req.Action != "approve" && req.Action != "reject" {
		return utils.NewAppError("VALIDATION_ERROR", "action must be 'approve' or 'reject'", http.StatusBadRequest)
	}

	adminID, ok := utils.UserIDFromContext(r.Context())
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

	response, err := h.reviewUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur revue KYC")
		// Si l'erreur est déjà une AppError, on la retourne telle quelle
		if _, ok := err.(*utils.AppError); ok {
			return err
		}
		return utils.NewAppError("REVIEW_KYC_FAILED", err.Error(), http.StatusInternalServerError)
	}

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
}
