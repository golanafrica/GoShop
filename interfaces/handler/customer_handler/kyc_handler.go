package customerhandler

import (
	"encoding/json"
	"net/http"

	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
)

// KYCHandler gère les routes KYC
type KYCHandler struct {
	uploadKYCUC      *customerusecase.UploadKYCDocumentUsecase
	getKYCStatusUC   *customerusecase.GetKYCStatusUsecase
	reviewKYCUC      *customerusecase.ReviewKYCUsecase
	listPendingKYCUC *customerusecase.ListPendingKYCUsecase
}

// NewKYCHandler crée une nouvelle instance
func NewKYCHandler(
	uploadKYCUC *customerusecase.UploadKYCDocumentUsecase,
	getKYCStatusUC *customerusecase.GetKYCStatusUsecase,
	reviewKYCUC *customerusecase.ReviewKYCUsecase,
	listPendingKYCUC *customerusecase.ListPendingKYCUsecase,
) *KYCHandler {
	return &KYCHandler{
		uploadKYCUC:      uploadKYCUC,
		getKYCStatusUC:   getKYCStatusUC,
		reviewKYCUC:      reviewKYCUC,
		listPendingKYCUC: listPendingKYCUC,
	}
}

// UploadKYC gère POST /api/customers/kyc/upload
func (h *KYCHandler) UploadKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req customerusecase.UploadKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	doc, err := h.uploadKYCUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("UPLOAD_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, doc)
	return nil
}

// GetKYCStatus gère GET /api/customers/{customer_id}/kyc/status
func (h *KYCHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		return utils.ErrInvalidPayload
	}

	response, err := h.getKYCStatusUC.Execute(ctx, customerID)
	if err != nil {
		return utils.NewAppError("GET_KYC_STATUS_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ReviewKYC gère POST /api/merchant/kyc/{customer_id}/review
func (h *KYCHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		return utils.ErrInvalidPayload
	}

	var req customerusecase.ReviewKYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	req.CustomerID = customerID

	response, err := h.reviewKYCUC.Execute(ctx, &req)
	if err != nil {
		return utils.NewAppError("REVIEW_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ListPendingKYC gère GET /api/merchant/kyc/pending
func (h *KYCHandler) ListPendingKYC(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	items, err := h.listPendingKYCUC.Execute(ctx)
	if err != nil {
		return utils.NewAppError("LIST_PENDING_KYC_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, items)
	return nil
}
