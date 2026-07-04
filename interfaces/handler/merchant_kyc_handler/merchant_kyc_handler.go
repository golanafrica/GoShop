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
//
// 🎯 Objectif :
//   Exposer les endpoints HTTP pour le workflow KYC marchand.
//
// 📋 Endpoints :
//   MARCHAND (côté client) :
//   - POST /api/merchant/kyc/submit       → Soumettre documents KYC
//   - GET  /api/merchant/kyc/status       → Consulter son statut KYC
//
//   ADMIN (côté admin) :
//   - GET  /api/admin/merchant-kyc/pending → Lister shops en attente
//   - PUT  /api/admin/merchant-kyc/{id}/review → Approuver/Rejeter
//
// 🔐 Protection :
//   - Routes marchand : AuthMiddleware + TenantResolver
//   - Routes admin    : AuthMiddleware + RequireRoles("super_admin", "admin")
//
// ============================================================

// MerchantKYCHandler gère les endpoints KYC marchand
type MerchantKYCHandler struct {
	submitUC *merchantkycusecase.SubmitMerchantKYCUsecase
	reviewUC *merchantkycusecase.ReviewMerchantKYCUsecase
	listUC   *merchantkycusecase.ListPendingMerchantKYCUsecase
	statusUC *merchantkycusecase.GetMerchantKYCStatusUsecase
}

// NewMerchantKYCHandler crée une nouvelle instance
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
// ENDPOINTS MARCHAND
// ============================================================

// SubmitKYCRequest représente la requête de soumission KYC
type SubmitKYCRequest struct {
	Documents []DocumentInput `json:"documents"`
}

// DocumentInput représente un document soumis
type DocumentInput struct {
	DocumentType  string `json:"document_type"`
	FilePath      string `json:"file_path"`
	FileName      string `json:"file_name"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	MimeType      string `json:"mime_type"`
}

// SubmitKYC gère POST /api/merchant/kyc/submit
//
// @Summary Soumettre documents KYC marchand
// @Description Le marchand soumet ses documents pour vérification
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Param request body SubmitKYCRequest true "Documents KYC"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError
// @Failure 401 {object} utils.AppError
// @Failure 403 {object} utils.AppError
// @Router /api/merchant/kyc/submit [post]
// @Security BearerAuth
func (h *MerchantKYCHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	// 1. Parser le body
	var req SubmitKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 2. Validation basique
	if len(req.Documents) == 0 {
		return utils.NewAppError("VALIDATION_ERROR", "at least one document is required", http.StatusBadRequest)
	}

	// 3. Convertir en format usecase
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

	// 4. Exécuter le usecase
	ucReq := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: ucDocs,
	}

	response, err := h.submitUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur soumission KYC")
		return err
	}

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": response.Message,
		"data":    response,
	})

	return nil
}

// GetKYCStatus gère GET /api/merchant/kyc/status
//
// @Summary Consulter son statut KYC
// @Description Le marchand consulte son statut KYC et ses documents
// @Tags Merchant KYC
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError
// @Router /api/merchant/kyc/status [get]
// @Security BearerAuth
func (h *MerchantKYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	// Exécuter le usecase
	response, err := h.statusUC.Execute(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération statut KYC")
		return err
	}

	// Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

// ============================================================
// ENDPOINTS ADMIN
// ============================================================

// ListPendingKYC gère GET /api/admin/merchant-kyc/pending
//
// @Summary Lister les shops KYC en attente
// @Description L'admin liste tous les shops en attente de vérification KYC
// @Tags Admin KYC
// @Produce json
// @Param limit query int false "Limite (défaut: 20)"
// @Param offset query int false "Offset (défaut: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError
// @Failure 403 {object} utils.AppError
// @Router /api/admin/merchant-kyc/pending [get]
// @Security BearerAuth
func (h *MerchantKYCHandler) ListPendingKYC(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	// Parser les paramètres de pagination
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

	// Exécuter le usecase
	ucReq := &merchantkycusecase.ListPendingRequest{
		Limit:  limit,
		Offset: offset,
	}

	response, err := h.listUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur liste shops pending")
		return err
	}

	// Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

// ReviewKYCRequest représente la requête de revue KYC (admin)
type ReviewKYCRequest struct {
	Action          string                `json:"action"` // "approve" ou "reject"
	RejectionReason *string               `json:"rejection_reason,omitempty"`
	DocumentReviews []DocumentReviewInput `json:"document_reviews,omitempty"`
}

// DocumentReviewInput représente la revue d'un document
type DocumentReviewInput struct {
	DocumentID      string  `json:"document_id"`
	Action          string  `json:"action"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// ReviewKYC gère PUT /api/admin/merchant-kyc/{shop_id}/review
//
// @Summary Approuver ou rejeter le KYC d'un shop
// @Description L'admin approuve ou rejette le KYC d'un marchand
// @Tags Admin KYC
// @Accept json
// @Produce json
// @Param shop_id path string true "ID du shop"
// @Param request body ReviewKYCRequest true "Action de revue"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError
// @Failure 401 {object} utils.AppError
// @Failure 403 {object} utils.AppError
// @Failure 404 {object} utils.AppError
// @Router /api/admin/merchant-kyc/{shop_id}/review [put]
// @Security BearerAuth
func (h *MerchantKYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop_id depuis l'URL
	shopID := chi.URLParam(r, "shop_id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	// 2. Parser le body
	var req ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 3. Récupérer l'admin_id depuis le contexte
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return utils.ErrUnauthorized
	}

	// 4. Convertir les document reviews
	docReviews := make([]merchantkycusecase.DocumentReview, len(req.DocumentReviews))
	for i, dr := range req.DocumentReviews {
		docReviews[i] = merchantkycusecase.DocumentReview{
			DocumentID:      dr.DocumentID,
			Action:          dr.Action,
			RejectionReason: dr.RejectionReason,
		}
	}

	// 5. Construire la requête usecase
	ucReq := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:          shopID,
		Action:          req.Action,
		AdminID:         adminID,
		RejectionReason: req.RejectionReason,
		DocumentReviews: docReviews,
	}

	// 6. Exécuter le usecase
	response, err := h.reviewUC.Execute(r.Context(), ucReq)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur revue KYC")
		return err
	}

	// 7. Retourner la réponse
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

// RegisterMerchantRoutes enregistre les routes côté marchand
// Les handlers sont enveloppés avec middl.ErrorHandler() pour gérer les erreurs
func (h *MerchantKYCHandler) RegisterMerchantRoutes(r chi.Router) {
	r.Post("/submit", middl.ErrorHandler(h.SubmitKYC))
	r.Get("/status", middl.ErrorHandler(h.GetKYCStatus))
}

// RegisterAdminRoutes enregistre les routes côté admin
func (h *MerchantKYCHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/pending", middl.ErrorHandler(h.ListPendingKYC))
	r.Put("/{shop_id}/review", middl.ErrorHandler(h.ReviewKYC))
}
