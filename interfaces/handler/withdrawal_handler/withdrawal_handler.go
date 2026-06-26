package withdrawalhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// CreateWithdrawalUseCaseInterface définit le contrat
type CreateWithdrawalUseCaseInterface interface {
	Execute(ctx context.Context, req *withdrawaldto.CreateWithdrawalRequest) (*withdrawaldto.WithdrawalResponse, error)
}

// ListWithdrawalsUseCaseInterface définit le contrat
type ListWithdrawalsUseCaseInterface interface {
	Execute(ctx context.Context, limit, offset int) ([]*withdrawaldto.WithdrawalResponse, error)
	GetWithdrawal(ctx context.Context, id string) (*withdrawaldto.WithdrawalResponse, error)
}

// WithdrawalHandler gère les requêtes HTTP pour les retraits
type WithdrawalHandler struct {
	createUC CreateWithdrawalUseCaseInterface
	listUC   ListWithdrawalsUseCaseInterface
}

func NewWithdrawalHandler(
	createUC CreateWithdrawalUseCaseInterface,
	listUC ListWithdrawalsUseCaseInterface,
) *WithdrawalHandler {
	return &WithdrawalHandler{createUC: createUC, listUC: listUC}
}

// CreateWithdrawal crée un nouveau retrait
func (h *WithdrawalHandler) CreateWithdrawal(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	var req withdrawaldto.CreateWithdrawalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.NewAppError("VALIDATION_FAILED", err.Error(), http.StatusBadRequest)
	}

	resp, err := h.createUC.Execute(ctx, &req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create withdrawal")
		return utils.NewAppError("WITHDRAWAL_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListWithdrawals liste les retraits de la boutique
func (h *WithdrawalHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	responses, err := h.listUC.Execute(ctx, limit, offset)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list withdrawals")
		return utils.ErrInternalServer
	}

	utils.WriteJSON(w, http.StatusOK, responses)
	return nil
}

// GetWithdrawal récupère un retrait par ID
func (h *WithdrawalHandler) GetWithdrawal(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	id := chi.URLParam(r, "id")
	if id == "" {
		return utils.ErrInvalidPayload
	}

	resp, err := h.listUC.GetWithdrawal(ctx, id)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get withdrawal")
		errMsg := err.Error()
		switch {
		case errMsg == "withdrawal not found":
			return utils.NewAppError("WITHDRAWAL_NOT_FOUND", errMsg, http.StatusNotFound)
		case errMsg == "withdrawal does not belong to this shop":
			return utils.ErrForbidden
		default:
			return utils.ErrInternalServer
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}
