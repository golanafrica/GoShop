package walletusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// DEBIT WALLET USECASE
// ============================================================

// DebitWalletRequest représente la requête pour débiter un wallet
type DebitWalletRequest struct {
	// Identifiants
	ShopID string `json:"shop_id"`

	// Montant à débiter (en centimes, positif)
	AmountCents int64 `json:"amount_cents"`

	// Type de transaction (doit être un débit)
	TransactionType entity.WalletTransactionType `json:"transaction_type"`

	// Référence optionnelle (order, credit, tontine, etc.)
	ReferenceType *string `json:"reference_type,omitempty"`
	ReferenceID   *string `json:"reference_id,omitempty"`

	// Description optionnelle
	Description *string `json:"description,omitempty"`

	// Option : autoriser le wallet à devenir négatif (défaut: true)
	AllowNegative bool `json:"allow_negative"`
}

// DebitWalletResponse représente la réponse après débit
type DebitWalletResponse struct {
	// Wallet mis à jour
	ShopID          string `json:"shop_id"`
	BalanceCents    int64  `json:"balance_cents"`
	PreviousBalance int64  `json:"previous_balance"`
	IsFrozen        bool   `json:"is_frozen"`

	// Transaction créée
	TransactionID     string                       `json:"transaction_id"`
	TransactionType   entity.WalletTransactionType `json:"transaction_type"`
	AmountCents       int64                        `json:"amount_cents"`
	BalanceAfterCents int64                        `json:"balance_after_cents"`

	// Indicateurs importants
	IsNowNegative bool `json:"is_now_negative"`
	ShouldFreeze  bool `json:"should_freeze"` // Le caller doit geler le wallet
}

// Validate valide la requête
func (r *DebitWalletRequest) Validate() error {
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if r.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	if !r.TransactionType.IsValid() {
		return fmt.Errorf("invalid transaction type: %s", r.TransactionType)
	}
	if !r.TransactionType.IsDebit() {
		return fmt.Errorf("transaction type must be a debit type, got: %s", r.TransactionType)
	}
	return nil
}

// DebitWalletUsecase débite le wallet d'un marchand
type DebitWalletUsecase struct {
	walletRepo repository.MerchantWalletRepository
	txnRepo    repository.WalletTransactionRepository
	txManager  repository.TxManager
}

// NewDebitWalletUsecase crée une nouvelle instance
func NewDebitWalletUsecase(
	walletRepo repository.MerchantWalletRepository,
	txnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
) *DebitWalletUsecase {
	return &DebitWalletUsecase{
		walletRepo: walletRepo,
		txnRepo:    txnRepo,
		txManager:  txManager,
	}
}

// Execute débite le wallet et enregistre la transaction
func (uc *DebitWalletUsecase) Execute(ctx context.Context, req *DebitWalletRequest) (*DebitWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid debit wallet request")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	if shop.ID.String() != req.ShopID {
		return nil, fmt.Errorf("access denied: shop_id does not match tenant")
	}

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. 🛡️ SÉCURITÉ : Récupérer le wallet avec verrouillage (FOR UPDATE)
	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find wallet")
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// 5. Vérifier que le wallet n'est pas gelé
	if wallet.IsFrozen {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Bool("is_frozen", wallet.IsFrozen).
			Msg("Attempted to debit frozen wallet")
		return nil, fmt.Errorf("wallet is frozen, cannot debit")
	}

	// 6bis. 🛡️ SÉCURITÉ Phase 2 : un retrait (payout) ne peut jamais entamer les fonds "held"
	// (ex: net cycle tontine en attente de redeem). Vérifié ICI, dans la transaction verrouillée,
	// pour éviter toute race condition avec un check fait en amont hors transaction.
	if req.TransactionType == entity.WalletTxPayout {
		available := wallet.AvailableCents()
		if req.AmountCents > available {
			logger.Warn().
				Str("shop_id", req.ShopID).
				Int64("requested", req.AmountCents).
				Int64("balance_cents", wallet.BalanceCents).
				Int64("held_cents", wallet.HeldCents).
				Int64("available_cents", available).
				Msg("❌ Débit payout refusé : dépasserait les fonds disponibles (held exclu)")
			return nil, fmt.Errorf(
				"insufficient available balance: requested=%d available=%d (balance=%d held=%d)",
				req.AmountCents, available, wallet.BalanceCents, wallet.HeldCents,
			)
		}
	}

	// 6. Sauvegarder le solde précédent
	previousBalance := wallet.BalanceCents

	// 7. Vérifier si le débit est autorisé
	newBalance := wallet.BalanceCents - req.AmountCents
	if newBalance < wallet.MaxNegativeBalanceCents && !req.AllowNegative {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Int64("amount_cents", req.AmountCents).
			Int64("current_balance", wallet.BalanceCents).
			Int64("max_negative", wallet.MaxNegativeBalanceCents).
			Msg("Debit would exceed max negative balance")
		return nil, fmt.Errorf("debit would exceed max negative balance: %d < %d",
			newBalance, wallet.MaxNegativeBalanceCents)
	}

	// 8. Débiter le wallet
	if err := wallet.Debit(req.AmountCents); err != nil {
		logger.Error().Err(err).Msg("Failed to debit wallet")
		return nil, fmt.Errorf("failed to debit wallet: %w", err)
	}

	// 9. Mettre à jour le wallet dans la base
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// 10. Créer la transaction
	txnID := uuid.New().String()
	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            req.ShopID,
		TransactionType:   req.TransactionType,
		AmountCents:       -req.AmountCents, // Négatif pour un débit
		BalanceAfterCents: wallet.BalanceCents,
		ReferenceType:     req.ReferenceType,
		ReferenceID:       req.ReferenceID,
		Description:       req.Description,
		Status:            entity.WalletTxCompleted,
	}

	if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
		logger.Error().Err(err).Msg("Failed to create transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 11. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Déterminer si le wallet doit être gelé
	shouldFreeze := wallet.BalanceCents < 0 && previousBalance >= 0

	// 13. Logger le résultat
	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("amount_cents", req.AmountCents).
		Int64("previous_balance", previousBalance).
		Int64("new_balance", wallet.BalanceCents).
		Str("transaction_type", string(req.TransactionType)).
		Str("transaction_id", txnID).
		Bool("is_now_negative", wallet.BalanceCents < 0).
		Bool("should_freeze", shouldFreeze).
		Msg("Wallet debited successfully")

	// 14. Retourner la réponse
	return &DebitWalletResponse{
		ShopID:            wallet.ShopID,
		BalanceCents:      wallet.BalanceCents,
		PreviousBalance:   previousBalance,
		IsFrozen:          wallet.IsFrozen,
		TransactionID:     txnID,
		TransactionType:   req.TransactionType,
		AmountCents:       req.AmountCents,
		BalanceAfterCents: wallet.BalanceCents,
		IsNowNegative:     wallet.BalanceCents < 0,
		ShouldFreeze:      shouldFreeze,
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// DebitCommission débite le wallet pour prélever une commission GoShop
func (uc *DebitWalletUsecase) DebitCommission(
	ctx context.Context,
	shopID string,
	amountCents int64,
	referenceType, referenceID string,
) (*DebitWalletResponse, error) {
	refType := referenceType
	refID := referenceID
	description := fmt.Sprintf("GoShop commission for %s %s", referenceType, referenceID)

	req := &DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxCommissionDebit,
		ReferenceType:   &refType,
		ReferenceID:     &refID,
		Description:     &description,
		AllowNegative:   true, // Commission peut créer une dette
	}

	return uc.Execute(ctx, req)
}

// DebitRefund débite le wallet pour un remboursement client
func (uc *DebitWalletUsecase) DebitRefund(
	ctx context.Context,
	shopID string,
	amountCents int64,
	orderID string,
) (*DebitWalletResponse, error) {
	refType := "order"
	description := fmt.Sprintf("Refund for order %s", orderID)

	req := &DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxRefund,
		ReferenceType:   &refType,
		ReferenceID:     &orderID,
		Description:     &description,
		AllowNegative:   true, // Remboursement peut créer une dette
	}

	return uc.Execute(ctx, req)
}

// DebitPayout débite le wallet pour un virement vers compte bancaire
func (uc *DebitWalletUsecase) DebitPayout(
	ctx context.Context,
	shopID string,
	amountCents int64,
	payoutID string,
) (*DebitWalletResponse, error) {
	refType := "payout"
	description := fmt.Sprintf("Payout to bank account: %d FCFA", amountCents/100)

	req := &DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxPayout,
		ReferenceType:   &refType,
		ReferenceID:     &payoutID,
		Description:     &description,
		AllowNegative:   false, // Payout ne peut pas créer de dette
	}

	return uc.Execute(ctx, req)
}

// DebitPenalty débite le wallet pour une pénalité
func (uc *DebitWalletUsecase) DebitPenalty(
	ctx context.Context,
	shopID string,
	amountCents int64,
	reason string,
) (*DebitWalletResponse, error) {
	refType := "penalty"
	description := fmt.Sprintf("Penalty: %s", reason)

	req := &DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxFreezePenalty,
		ReferenceType:   &refType,
		Description:     &description,
		AllowNegative:   true, // Pénalité peut créer une dette
	}

	return uc.Execute(ctx, req)
}

// ============================================================
// MÉTHODES D'INFORMATION
// ============================================================

// GetWallet retourne le wallet d'un shop (pour information)
func (uc *DebitWalletUsecase) GetWallet(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	return uc.walletRepo.FindByShopID(ctx, shopID)
}
