package merchantkychandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"Goshop/application/metrics"
	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"
	"Goshop/domain/tenant"
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

const maxUploadBytes = 5 * 1024 * 1024

// ============================================================
// ENDPOINTS MARCHAND
// ============================================================

// UploadDocument — multipart : path 100% serveur (J3)
//
// Form fields:
//   - file (required)
//   - document_type (required): identity_card | passport | business_registry | ...
//
// @Summary Upload un document KYC marchand
// @Tags Merchant KYC
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Fichier JPEG/PNG/PDF"
// @Param document_type formData string true "Type de document"
// @Success 201 {object} map[string]interface{}
// @Router /api/merchant/kyc/upload [post]
func (h *MerchantKYCHandler) UploadDocument(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return utils.NewAppError("TENANT_REQUIRED", "multi-tenant context required", http.StatusUnauthorized)
	}

	if err := r.ParseMultipartForm(maxUploadBytes + 512*1024); err != nil {
		return utils.NewAppError("INVALID_MULTIPART", "invalid multipart form", http.StatusBadRequest)
	}

	docType := strings.TrimSpace(r.FormValue("document_type"))
	if docType == "" {
		return utils.NewAppError("VALIDATION_ERROR", "document_type is required", http.StatusBadRequest)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return utils.NewAppError("VALIDATION_ERROR", "file is required", http.StatusBadRequest)
	}
	defer file.Close()

	if header.Size > maxUploadBytes {
		return utils.NewAppError("FILE_TOO_LARGE", "file exceeds maximum size (5 MB)", http.StatusBadRequest)
	}

	// Lire en memoire (plafond 5 Mo) pour magic number puis ecrire
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return utils.NewAppError("READ_FAILED", "failed to read upload", http.StatusBadRequest)
	}
	if int64(len(data)) > maxUploadBytes {
		return utils.NewAppError("FILE_TOO_LARGE", "file exceeds maximum size (5 MB)", http.StatusBadRequest)
	}

	allowedTypes := []utils.FileType{
		utils.FileTypeJPEG,
		utils.FileTypePNG,
		utils.FileTypePDF,
	}
	// bytes.NewReader implemente io.ReadSeeker
	contentResult, err := utils.ValidateFileContent(
		newBytesReadSeeker(data),
		maxUploadBytes,
		allowedTypes,
	)
	if err != nil {
		return utils.NewAppError("CONTENT_VALIDATION", err.Error(), http.StatusBadRequest)
	}
	if contentResult == nil || !contentResult.Valid {
		msg := "invalid file content"
		if contentResult != nil && contentResult.ErrorMessage != "" {
			msg = contentResult.ErrorMessage
		}
		return utils.NewAppError("CONTENT_VALIDATION", msg, http.StatusBadRequest)
	}

	originalName := filepath.Base(header.Filename)
	if originalName == "" || originalName == "." {
		originalName = "document." + string(contentResult.FileType)
	}
	if err := utils.ValidateFileMetadata(originalName, contentResult.MimeType, contentResult.FileType); err != nil {
		// Si le nom client a une mauvaise extension, on force le nom serveur plus bas
		logger.Warn().Err(err).Str("original", originalName).Msg("Client filename extension mismatch; using server name")
		originalName = "document." + extForFileType(contentResult.FileType)
	}

	serverPath := merchantkycusecase.GenerateMerchantKYCFilePath(
		shop.ID.String(),
		docType,
		contentResult.MimeType,
	)

	if err := os.MkdirAll(filepath.Dir(serverPath), 0755); err != nil {
		logger.Error().Err(err).Msg("mkdir upload failed")
		return utils.NewAppError("STORAGE_ERROR", "failed to prepare storage", http.StatusInternalServerError)
	}

	if err := os.WriteFile(serverPath, data, 0644); err != nil {
		logger.Error().Err(err).Str("path", serverPath).Msg("write upload failed")
		return utils.NewAppError("STORAGE_ERROR", "failed to store file", http.StatusInternalServerError)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("document_type", docType).
		Str("file_path", serverPath).
		Int64("size", int64(len(data))).
		Str("mime", contentResult.MimeType).
		Msg("Merchant KYC file uploaded (server path)")

	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"document_type":   docType,
			"file_path":       serverPath,
			"file_name":       filepath.Base(serverPath),
			"file_size_bytes": int64(len(data)),
			"mime_type":       contentResult.MimeType,
		},
	})
	return nil
}

func extForFileType(ft utils.FileType) string {
	switch ft {
	case utils.FileTypePNG:
		return "png"
	case utils.FileTypePDF:
		return "pdf"
	default:
		return "jpg"
	}
}

// bytesReadSeeker adapte []byte en io.ReadSeeker pour ValidateFileContent
type bytesReadSeeker struct {
	data []byte
	pos  int64
}

func newBytesReadSeeker(b []byte) *bytesReadSeeker {
	return &bytesReadSeeker{data: b}
}

func (r *bytesReadSeeker) Read(p []byte) (int, error) {
	if r.pos >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += int64(n)
	return n, nil
}

func (r *bytesReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.pos + offset
	case io.SeekEnd:
		abs = int64(len(r.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}
	if abs < 0 {
		return 0, fmt.Errorf("negative position")
	}
	r.pos = abs
	return abs, nil
}

// @Summary Soumettre une demande de KYC Marchand
// @Tags Merchant KYC
// @Accept json
// @Produce json
// @Router /api/merchant/kyc/submit [post]
func (h *MerchantKYCHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	var req SubmitKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Erreur decodage JSON")
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
		metrics.MerchantKYCSubmitTotal.WithLabelValues("error").Inc()
		metrics.MerchantKYCOperationDuration.WithLabelValues("submit").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_submit", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Int("documents_count", len(req.Documents)).
			Float64("duration_seconds", duration).
			Msg("Erreur soumission KYC")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "file path must be under") ||
			strings.Contains(errMsg, "file path must not") ||
			strings.Contains(errMsg, "content validation") {
			return utils.NewAppError("KYC_INVALID_FILE", errMsg, http.StatusBadRequest)
		}
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no documents") {
			return utils.NewAppError("KYC_NOT_FOUND", errMsg, http.StatusNotFound)
		}
		if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "pending") {
			return utils.NewAppError("KYC_INVALID_STATUS", errMsg, http.StatusBadRequest)
		}

		return utils.NewAppError("SUBMIT_KYC_FAILED", errMsg, http.StatusInternalServerError)
	}

	metrics.MerchantKYCSubmitTotal.WithLabelValues("success").Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("submit").Observe(duration)
	metrics.MerchantKYCDocumentsPerSubmit.Observe(float64(len(req.Documents)))

	logger.Info().
		Int("documents_count", len(req.Documents)).
		Float64("duration_seconds", duration).
		Msg("KYC submitted successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": response.Message,
		"data":    response,
	})

	return nil
}

// @Summary Obtenir le statut KYC du marchand
// @Tags Merchant KYC
// @Router /api/merchant/kyc/status [get]
func (h *MerchantKYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	response, err := h.statusUC.Execute(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.MerchantKYCOperationDuration.WithLabelValues("get_status").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_status", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Erreur recuperation statut KYC")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		return utils.NewAppError("GET_KYC_STATUS_FAILED", err.Error(), http.StatusInternalServerError)
	}

	metrics.MerchantKYCStatusCheckTotal.Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("get_status").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

// ============================================================
// ENDPOINTS ADMIN
// ============================================================

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
		metrics.MerchantKYCOperationDuration.WithLabelValues("list_pending").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_list", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Erreur liste shops pending")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		return utils.NewAppError("LIST_PENDING_KYC_FAILED", err.Error(), http.StatusInternalServerError)
	}

	metrics.MerchantKYCPendingListTotal.Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("list_pending").Observe(duration)

	pendingCount := 0
	if response != nil {
		responseValue := reflect.ValueOf(response).Elem()
		if shopsField := responseValue.FieldByName("Shops"); shopsField.IsValid() && shopsField.Kind() == reflect.Slice {
			pendingCount = shopsField.Len()
		} else if countField := responseValue.FieldByName("Count"); countField.IsValid() && countField.Kind() == reflect.Int {
			pendingCount = int(countField.Int())
		} else if totalField := responseValue.FieldByName("Total"); totalField.IsValid() && totalField.Kind() == reflect.Int {
			pendingCount = int(totalField.Int())
		}
	}

	metrics.MerchantKYCPendingCount.Observe(float64(pendingCount))

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    response,
	})

	return nil
}

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
		logger.Error().Err(err).Msg("Erreur decodage JSON")
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
		metrics.MerchantKYCReviewTotal.WithLabelValues(req.Action, "error").Inc()
		metrics.MerchantKYCOperationDuration.WithLabelValues("review").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_kyc_review", "merchant_kyc_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Str("action", req.Action).
			Float64("duration_seconds", duration).
			Msg("Erreur revue KYC")

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

	metrics.MerchantKYCReviewTotal.WithLabelValues(req.Action, "success").Inc()
	metrics.MerchantKYCOperationDuration.WithLabelValues("review").Observe(duration)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": response.Message,
		"data":    response,
	})

	return nil
}

// ============================================================
// ROUTES
// ============================================================

func (h *MerchantKYCHandler) RegisterMerchantRoutes(r chi.Router) {
	r.Post("/upload", middl.ErrorHandler(h.UploadDocument))
	r.Post("/submit", middl.ErrorHandler(h.SubmitKYC))
	r.Get("/status", middl.ErrorHandler(h.GetKYCStatus))
}

func (h *MerchantKYCHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/pending", middl.ErrorHandler(h.ListPendingKYC))
	r.Put("/{shop_id}/review", middl.ErrorHandler(h.ReviewKYC))
	r.Post("/{shop_id}/review", middl.ErrorHandler(h.ReviewKYC))
}
