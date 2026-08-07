package withdrawalhandler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	walletusecase "Goshop/application/usecase/wallet_usecase"
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
	debitUC  *walletusecase.DebitWalletUsecase // 🆕 Phase 2 : Pour vérification held_cents
}

// NewWithdrawalHandler crée une nouvelle instance
func NewWithdrawalHandler(
	createUC CreateWithdrawalUseCaseInterface,
	listUC ListWithdrawalsUseCaseInterface,
	debitUC *walletusecase.DebitWalletUsecase, // 🆕 Phase 2
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

	// 🛡️ Phase 2 : Vérification held_cents AVANT de procéder au retrait
	shop, tenantErr := tenant.FromContext(ctx)
	if tenantErr == nil && shop != nil && h.debitUC != nil {
		shopID := shop.ID.String()
		wallet, err := h.debitUC.GetWallet(ctx, shopID)
		if err == nil {
			availableCents := wallet.AvailableCents()
			if req.AmountCents > availableCents {
				logger.Warn().
					Str("shop_id", shopID).
					Int64("requested", req.AmountCents).
					Int64("available", availableCents).
					Int64("held_cents", wallet.HeldCents).
					Int64("balance_cents", wallet.BalanceCents).
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
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create withdrawal")
		return utils.NewAppError("WITHDRAWAL_FAILED", err.Error(), http.StatusBadRequest)
	}

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
