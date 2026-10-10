package withdrawalhandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	"Goshop/application/metrics"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"
	"Goshop/domain/tenant"
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
	debitUC  *walletusecase.DebitWalletUsecase
}

// NewWithdrawalHandler crée une nouvelle instance
func NewWithdrawalHandler(
	createUC CreateWithdrawalUseCaseInterface,
	listUC ListWithdrawalsUseCaseInterface,
	debitUC *walletusecase.DebitWalletUsecase,
) *WithdrawalHandler {
	return &WithdrawalHandler{
		createUC: createUC,
		listUC:   listUC,
		debitUC:  debitUC,
	}
}

// @Summary Créer une demande de retrait
// @Description Crée une nouvelle demande de retrait de fonds depuis le portefeuille de la boutique vers un compte externe (Mobile Money ou Banque).
// @Tags Withdrawals
// @Accept json
// @Produce json
// @Param request body withdrawaldto.CreateWithdrawalRequest true "Détails du retrait (montant, méthode, etc.)"
// @Success 201 {object} withdrawaldto.WithdrawalResponse
// @Failure 400 {object} utils.AppError "Payload invalide, solde insuffisant ou held_cents bloque le retrait"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Compte gelé ou interdit"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/withdrawals [post]
func (h *WithdrawalHandler) CreateWithdrawal(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	var req withdrawaldto.CreateWithdrawalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.WithdrawalCreateTotal.WithLabelValues("validation_error").Inc()
		metrics.WithdrawalOperationDuration.WithLabelValues("create").Observe(time.Since(start).Seconds())

		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		metrics.WithdrawalCreateTotal.WithLabelValues("validation_error").Inc()
		metrics.WithdrawalOperationDuration.WithLabelValues("create").Observe(time.Since(start).Seconds())

		logger.Warn().Err(err).Msg("Validation failed")
		return utils.NewAppError("VALIDATION_FAILED", err.Error(), http.StatusBadRequest)
	}

	shop, tenantErr := tenant.FromContext(ctx)
	if tenantErr == nil && shop != nil && h.debitUC != nil {
		shopID := shop.ID.String()
		wallet, err := h.debitUC.GetWallet(ctx, shopID)
		if err == nil {
			availableCents := wallet.AvailableCents()
			if req.AmountCents > availableCents {
				metrics.WithdrawalHeldCentsRejections.Inc()
				metrics.WithdrawalCreateTotal.WithLabelValues("held_cents_rejected").Inc()
				metrics.WithdrawalOperationDuration.WithLabelValues("create").Observe(time.Since(start).Seconds())

				logger.Warn().
					Str("shop_id", shopID).
					Int64("requested", req.AmountCents).
					Int64("available", availableCents).
					Int64("held_cents", wallet.HeldCents).
					Int64("balance_cents", wallet.BalanceCents).
					Float64("duration_seconds", time.Since(start).Seconds()).
					Msg("❌ Withdrawal rejected: insufficient available balance (held_cents protection)")

				return utils.NewAppError(
					"INSUFFICIENT_AVAILABLE_BALANCE",
					fmt.Sprintf("insufficient available balance. held_cents=%d blocks withdrawal. available=%d, balance=%d",
						wallet.HeldCents, availableCents, wallet.BalanceCents),
					http.StatusBadRequest,
				)
			}
		}
	}

	resp, err := h.createUC.Execute(ctx, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.WithdrawalCreateTotal.WithLabelValues("error").Inc()
		metrics.WithdrawalOperationDuration.WithLabelValues("create").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("withdrawal_create", "withdrawal_handler").Inc()

		logger.Error().Err(err).
			Int64("amount_cents", req.AmountCents).
			Float64("duration_seconds", duration).
			Msg("Failed to create withdrawal")

		var outcome *withdrawalusecase.ErrWithdrawalOutcome
		if errors.As(err, &outcome) && outcome != nil {
			body := map[string]interface{}{
				"code":              "WITHDRAWAL_FAILED",
				"message":           outcome.Error(),
				"status":            http.StatusBadRequest,
				"withdrawal_id":     outcome.WithdrawalID,
				"withdrawal_status": outcome.Status, // "failed" | "processing"
			}
			utils.WriteJSON(w, http.StatusBadRequest, body)
			return nil
		}

		return utils.NewAppError("WITHDRAWAL_FAILED", err.Error(), http.StatusBadRequest)
	}

	metrics.WithdrawalCreateTotal.WithLabelValues("success").Inc()
	metrics.WithdrawalOperationDuration.WithLabelValues("create").Observe(duration)
	metrics.WithdrawalAmountCents.Observe(float64(req.AmountCents))

	logger.Info().
		Int64("amount_cents", req.AmountCents).
		Float64("duration_seconds", duration).
		Msg("✅ Withdrawal created successfully")

	utils.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// @Summary Lister les retraits de la boutique
// @Description Retourne la liste paginée des demandes de retrait pour la boutique active.
// @Tags Withdrawals
// @Accept json
// @Produce json
// @Param limit query int false "Nombre de résultats (défaut: 50, max: 100)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Success 200 {array} withdrawaldto.WithdrawalResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/withdrawals [get]
func (h *WithdrawalHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
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
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.WithdrawalListTotal.WithLabelValues("error").Inc()
		metrics.WithdrawalOperationDuration.WithLabelValues("list").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("withdrawal_list", "withdrawal_handler").Inc()

		logger.Error().Err(err).
			Int("limit", limit).
			Int("offset", offset).
			Float64("duration_seconds", duration).
			Msg("Failed to list withdrawals")

		return utils.ErrInternalServer
	}

	metrics.WithdrawalListTotal.WithLabelValues("success").Inc()
	metrics.WithdrawalOperationDuration.WithLabelValues("list").Observe(duration)
	metrics.WithdrawalListedCount.Observe(float64(len(responses)))

	logger.Info().
		Int("withdrawals_count", len(responses)).
		Int("limit", limit).
		Int("offset", offset).
		Float64("duration_seconds", duration).
		Msg("✅ Withdrawals listed successfully")

	utils.WriteJSON(w, http.StatusOK, responses)
	return nil
}

// @Summary Récupérer les détails d'un retrait
// @Description Retourne les informations complètes et le statut d'une demande de retrait spécifique par son ID.
// @Tags Withdrawals
// @Accept json
// @Produce json
// @Param id path string true "ID du retrait (UUID)"
// @Success 200 {object} withdrawaldto.WithdrawalResponse
// @Failure 400 {object} utils.AppError "ID de retrait manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Le retrait n'appartient pas à votre boutique"
// @Failure 404 {object} utils.AppError "Retrait introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/withdrawals/{id} [get]
func (h *WithdrawalHandler) GetWithdrawal(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	id := chi.URLParam(r, "id")
	if id == "" {
		metrics.WithdrawalGetTotal.WithLabelValues("validation_error").Inc()
		metrics.WithdrawalOperationDuration.WithLabelValues("get").Observe(time.Since(start).Seconds())
		return utils.ErrInvalidPayload
	}

	resp, err := h.listUC.GetWithdrawal(ctx, id)
	duration := time.Since(start).Seconds()

	if err != nil {
		errMsg := err.Error()

		switch {
		case errMsg == "withdrawal not found":
			metrics.WithdrawalGetTotal.WithLabelValues("not_found").Inc()
			metrics.WithdrawalOperationDuration.WithLabelValues("get").Observe(duration)

			logger.Warn().
				Str("withdrawal_id", id).
				Float64("duration_seconds", duration).
				Msg("Withdrawal not found")

			return utils.NewAppError("WITHDRAWAL_NOT_FOUND", errMsg, http.StatusNotFound)

		case errMsg == "withdrawal does not belong to this shop":
			metrics.WithdrawalGetTotal.WithLabelValues("forbidden").Inc()
			metrics.WithdrawalOperationDuration.WithLabelValues("get").Observe(duration)

			logger.Warn().
				Str("withdrawal_id", id).
				Float64("duration_seconds", duration).
				Msg("Withdrawal access forbidden")

			return utils.ErrForbidden

		default:
			metrics.WithdrawalGetTotal.WithLabelValues("error").Inc()
			metrics.WithdrawalOperationDuration.WithLabelValues("get").Observe(duration)
			metrics.ApplicationErrorsTotal.WithLabelValues("withdrawal_get", "withdrawal_handler").Inc()

			logger.Error().Err(err).
				Str("withdrawal_id", id).
				Float64("duration_seconds", duration).
				Msg("Failed to get withdrawal")

			return utils.ErrInternalServer
		}
	}

	metrics.WithdrawalGetTotal.WithLabelValues("success").Inc()
	metrics.WithdrawalOperationDuration.WithLabelValues("get").Observe(duration)

	logger.Info().
		Str("withdrawal_id", id).
		Float64("duration_seconds", duration).
		Msg("✅ Withdrawal retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}
