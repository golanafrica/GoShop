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

// TransactionResponse représente une transaction
type TransactionResponse struct {
	ID                string `json:"id"`
	TransactionType   string `json:"transaction_type"`
	AmountCents       int64  `json:"amount_cents"`
	AmountFormatted   string `json:"amount_formatted"`
	BalanceAfterCents int64  `json:"balance_after_cents"`
	ReferenceType     string `json:"reference_type,omitempty"`
	ReferenceID       string `json:"reference_id,omitempty"`
	Description       string `json:"description,omitempty"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
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

// GetWallet retourne les informations du wallet du marchand
// GET /api/wallet
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

// GetFreezeStatus retourne le statut de gel du compte
// GET /api/wallet/freeze-status
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

// Deposit crédite le wallet d'un marchand
// POST /api/wallet/deposit
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

// Withdraw débite le wallet d'un marchand (vers compte bancaire)
// POST /api/wallet/withdraw
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

	// Générer un payout_id simulé (à remplacer par vrai payout)
	payoutID := "payout_" + shopID[:8]

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
		Msg("Withdrawal successful")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           "Withdrawal successful",
		"transaction_id":    resp.TransactionID,
		"amount_cents":      resp.AmountCents,
		"amount_formatted":  formatMoney(resp.AmountCents),
		"balance_cents":     resp.BalanceAfterCents,
		"balance_formatted": formatMoney(resp.BalanceAfterCents),
	})
}

// ============================================================
// HANDLERS : UNFREEZE
// ============================================================

// Unfreeze dégèle un compte marchand
// POST /api/wallet/unfreeze
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
