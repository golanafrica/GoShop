package wallet_handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// WALLET HANDLER
// ============================================================

// WalletHandler gère les endpoints liés au wallet marchand
type WalletHandler struct {
	creditUC   *walletusecase.CreditWalletUsecase
	debitUC    *walletusecase.DebitWalletUsecase
	freezeUC   *walletusecase.FreezeAccountUsecase
	unfreezeUC *walletusecase.UnfreezeAccountUsecase
}

// NewWalletHandler crée une nouvelle instance du handler
func NewWalletHandler(
	creditUC *walletusecase.CreditWalletUsecase,
	debitUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	unfreezeUC *walletusecase.UnfreezeAccountUsecase,
) *WalletHandler {
	return &WalletHandler{
		creditUC:   creditUC,
		debitUC:    debitUC,
		freezeUC:   freezeUC,
		unfreezeUC: unfreezeUC,
	}
}

// ============================================================
// REQUEST/RESPONSE TYPES
// ============================================================

// WalletResponse représente la réponse standard pour le wallet
type WalletResponse struct {
	ShopID           string `json:"shop_id"`
	BalanceCents     int64  `json:"balance_cents"`
	BalanceFormatted string `json:"balance_formatted"` // "50 000 FCFA"
	IsFrozen         bool   `json:"is_frozen"`
	FrozenReason     string `json:"frozen_reason,omitempty"`
	FrozenUntil      string `json:"frozen_until,omitempty"`
	MaxNegativeCents int64  `json:"max_negative_cents"`
	TotalSalesCents  int64  `json:"total_sales_cents"`
	TotalCommissions int64  `json:"total_commissions_cents"`
	TotalPayouts     int64  `json:"total_payouts_cents"`
}

// DepositRequest représente la requête pour un dépôt
type DepositRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description,omitempty"`
}

// WithdrawRequest représente la requête pour un retrait
type WithdrawRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description,omitempty"`
}

// UnfreezeRequest représente la requête pour dégeler un compte
type UnfreezeRequest struct {
	DepositAmountCents int64  `json:"deposit_amount_cents"`
	Resolution         string `json:"resolution"` // "paid", "waived", "escalated"
}

// FreezeStatusResponse représente le statut de gel
type FreezeStatusResponse struct {
	IsFrozen        bool   `json:"is_frozen"`
	FrozenAt        string `json:"frozen_at,omitempty"`
	FrozenReason    string `json:"frozen_reason,omitempty"`
	FrozenUntil     string `json:"frozen_until,omitempty"`
	DaysRemaining   int    `json:"days_remaining,omitempty"`
	AmountDueCents  int64  `json:"amount_due_cents,omitempty"`
	GracePeriodDays int    `json:"grace_period_days,omitempty"`
}

// ============================================================
// HANDLERS : WALLET INFO
// ============================================================

// @Summary Obtenir les informations du wallet
// @Description Retourne le solde et le statut actuel du portefeuille de la boutique active.
// @Tags Merchant Wallet
// @Accept json
// @Produce json
// @Success 200 {object} wallet_handler.WalletResponse
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/wallet [get]
func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop du contexte multi-tenant
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Récupérer le wallet via le usecase debit (qui a GetWallet)
	wallet, err := h.debitUC.GetWallet(r.Context(), shopID)
	if err != nil {
		// Wallet n'existe pas, retourner un wallet vide
		logger.Info().
			Str("shop_id", shopID).
			Msg("Wallet not found, returning empty wallet")

		utils.WriteJSON(w, http.StatusOK, WalletResponse{
			ShopID:           shopID,
			BalanceCents:     0,
			BalanceFormatted: "0 FCFA",
			IsFrozen:         false,
			MaxNegativeCents: entity.DefaultMaxNegativeBalanceCents,
		})
		return
	}

	// 3. Construire la réponse
	response := WalletResponse{
		ShopID:           wallet.ShopID,
		BalanceCents:     wallet.BalanceCents,
		BalanceFormatted: formatMoney(wallet.BalanceCents),
		IsFrozen:         wallet.IsFrozen,
		MaxNegativeCents: wallet.MaxNegativeBalanceCents,
		TotalSalesCents:  wallet.TotalSalesCents,
		TotalCommissions: wallet.TotalCommissionsCents,
		TotalPayouts:     wallet.TotalPayoutsCents,
	}

	// Ajouter les infos de gel si gelé
	if wallet.IsFrozen && wallet.FrozenReason != nil {
		response.FrozenReason = *wallet.FrozenReason
	}
	if wallet.IsFrozen && wallet.FrozenUntil != nil {
		response.FrozenUntil = wallet.FrozenUntil.Format("2006-01-02T15:04:05Z")
	}

	// 4. Logger et retourner
	logger.Debug().
		Str("shop_id", shopID).
		Int64("balance_cents", wallet.BalanceCents).
		Bool("is_frozen", wallet.IsFrozen).
		Msg("Wallet retrieved")

	utils.WriteJSON(w, http.StatusOK, response)
}

// @Summary Obtenir le statut de gel du compte
// @Description Vérifie si le wallet de la boutique est actuellement gelé et retourne les détails (raison, durée, montant dû).
// @Tags Merchant Wallet
// @Accept json
// @Produce json
// @Success 200 {object} wallet_handler.FreezeStatusResponse
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/wallet/freeze-status [get]
func (h *WalletHandler) GetFreezeStatus(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	wallet, err := h.debitUC.GetWallet(r.Context(), shopID)
	if err != nil {
		// Pas de wallet = pas de gel
		utils.WriteJSON(w, http.StatusOK, FreezeStatusResponse{
			IsFrozen: false,
		})
		return
	}

	response := FreezeStatusResponse{
		IsFrozen:        wallet.IsFrozen,
		GracePeriodDays: entity.DefaultGracePeriodDays,
	}

	if wallet.IsFrozen {
		if wallet.FrozenAt != nil {
			response.FrozenAt = wallet.FrozenAt.Format("2006-01-02T15:04:05Z")
		}
		if wallet.FrozenReason != nil {
			response.FrozenReason = *wallet.FrozenReason
		}
		if wallet.FrozenUntil != nil {
			response.FrozenUntil = wallet.FrozenUntil.Format("2006-01-02T15:04:05Z")
			response.DaysRemaining = wallet.DaysUntilSuspension()
		}
		if wallet.BalanceCents < 0 {
			response.AmountDueCents = -wallet.BalanceCents
		}
	}

	logger.Debug().
		Str("shop_id", shopID).
		Bool("is_frozen", wallet.IsFrozen).
		Msg("Freeze status retrieved")

	utils.WriteJSON(w, http.StatusOK, response)
}

// ============================================================
// HANDLERS : DEPOSIT / WITHDRAW
// ============================================================

// @Summary Effectuer un dépôt manuel dans le wallet
// @Description Crédite manuellement le portefeuille de la boutique d'un montant spécifié.
// @Tags Merchant Wallet
// @Accept json
// @Produce json
// @Param request body wallet_handler.DepositRequest true "Montant du dépôt et description optionnelle"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Montant invalide ou payload incorrect"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/wallet/deposit [post]
func (h *WalletHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Parser la requête
	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider le montant
	if req.AmountCents <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	// 4. Appeler le usecase
	description := req.Description
	if description == "" {
		description = "Manual deposit by merchant"
	}

	resp, err := h.creditUC.CreditFromDeposit(r.Context(), shopID, req.AmountCents, description)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to deposit")
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Deposit failed: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("shop_id", shopID).
		Int64("amount_cents", req.AmountCents).
		Str("transaction_id", resp.TransactionID).
		Msg("Deposit successful")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Deposit successful",
		"transaction_id":    resp.TransactionID,
		"amount_cents":      resp.AmountCents,
		"amount_formatted":  formatMoney(resp.AmountCents),
		"balance_cents":     resp.BalanceAfterCents,
		"balance_formatted": formatMoney(resp.BalanceAfterCents),
	})
}

// @Summary Effectuer un retrait depuis le wallet
// @Description Débite le portefeuille de la boutique pour initier un virement vers un compte bancaire ou Mobile Money.
// @Tags Merchant Wallet
// @Accept json
// @Produce json
// @Param request body wallet_handler.WithdrawRequest true "Montant du retrait et description optionnelle"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Montant invalide, solde insuffisant ou payload incorrect"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/wallet/withdraw [post]
func (h *WalletHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Parser la requête
	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider le montant
	if req.AmountCents <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	// 4. Appeler le usecase
	description := req.Description
	if description == "" {
		description = "Withdrawal to bank account"
	}

	// 🆕 v3.0.1 : Générer un UUID valide pour le payout (reference_id doit être un UUID PostgreSQL)
	payoutID := uuid.New().String()

	resp, err := h.debitUC.DebitPayout(r.Context(), shopID, req.AmountCents, payoutID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to withdraw")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Withdrawal failed: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("shop_id", shopID).
		Int64("amount_cents", req.AmountCents).
		Str("transaction_id", resp.TransactionID).
		Str("payout_id", payoutID).
		Msg("Withdrawal successful")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Withdrawal successful",
		"transaction_id":    resp.TransactionID,
		"payout_id":         payoutID,
		"amount_cents":      resp.AmountCents,
		"amount_formatted":  formatMoney(resp.AmountCents),
		"balance_cents":     resp.BalanceAfterCents,
		"balance_formatted": formatMoney(resp.BalanceAfterCents),
	})
}

// ============================================================
// HANDLERS : UNFREEZE
// ============================================================

// @Summary Dégeler un compte marchand
// @Description Lève le gel du portefeuille après régularisation de la situation (paiement, annulation ou escalade).
// @Tags Merchant Wallet
// @Accept json
// @Produce json
// @Param request body wallet_handler.UnfreezeRequest true "Montant de régularisation et type de résolution (paid, waived, escalated)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Résolution invalide ou payload incorrect"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/wallet/unfreeze [post]
func (h *WalletHandler) Unfreeze(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	// 1. Récupérer le shop
	shop, err := tenant.FromContext(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Multi-tenant context required")
		return
	}
	shopID := shop.ID.String()

	// 2. Parser la requête
	var req UnfreezeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	defer r.Body.Close()

	// 3. Valider la résolution
	var resolution entity.FreezeResolution
	switch req.Resolution {
	case "paid":
		resolution = entity.FreezeResolutionPaid
		if req.DepositAmountCents <= 0 {
			utils.WriteError(w, http.StatusBadRequest, "Deposit amount required for paid resolution")
			return
		}
	case "waived":
		resolution = entity.FreezeResolutionWaived
	case "escalated":
		resolution = entity.FreezeResolutionEscalated
	default:
		utils.WriteError(w, http.StatusBadRequest, "Resolution must be: paid, waived, or escalated")
		return
	}

	// 4. Appeler le usecase
	unfreezeReq := &walletusecase.UnfreezeAccountRequest{
		ShopID:             shopID,
		DepositAmountCents: req.DepositAmountCents,
		Resolution:         resolution,
		ResolvedBy:         shopID, // Le marchand se dégèle lui-même
	}

	resp, err := h.unfreezeUC.Execute(r.Context(), unfreezeReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to unfreeze account")
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Unfreeze failed: %v", err))
		return
	}

	// 5. Logger et retourner
	logger.Info().
		Str("shop_id", shopID).
		Str("freeze_id", resp.FreezeID).
		Str("resolution", string(resp.Resolution)).
		Int64("deposit_amount_cents", resp.DepositAmountCents).
		Msg("Account unfrozen successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Account unfrozen successfully",
		"freeze_id":         resp.FreezeID,
		"resolution":        resp.Resolution,
		"balance_cents":     resp.WalletBalanceCents,
		"balance_formatted": formatMoney(resp.WalletBalanceCents),
		"is_frozen":         resp.WalletIsFrozen,
	})
}

// ============================================================
// HELPERS
// ============================================================

// formatMoney formate un montant en centimes pour affichage
// Exemple: 50000 → "500 FCFA"
func formatMoney(cents int64) string {
	if cents < 0 {
		return "-" + formatMoney(-cents)
	}
	fcfa := cents / 100
	return formatNumber(fcfa) + " FCFA"
}

// formatNumber formate un nombre avec séparateurs de milliers
// Exemple: 50000 → "50 000"
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Conversion en string avec fmt
	str := fmt.Sprintf("%d", n)
	result := ""
	count := 0

	// Parcourir de droite à gauche
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

// RegisterRoutes enregistre les routes du wallet handler
func (h *WalletHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetWallet)
	r.Get("/freeze-status", h.GetFreezeStatus)
	r.Post("/deposit", h.Deposit)
	r.Post("/withdraw", h.Withdraw)
	r.Post("/unfreeze", h.Unfreeze)
}
