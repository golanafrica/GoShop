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
// CREDIT WALLET USECASE
// ============================================================

// CreditWalletRequest représente la requête pour créditer un wallet
type CreditWalletRequest struct {
	// Identifiants
	ShopID string `json:"shop_id"`

	// Montant à créditer (en centimes)
	AmountCents int64 `json:"amount_cents"`

	// Type de transaction
	TransactionType entity.WalletTransactionType `json:"transaction_type"`

	// Référence optionnelle (order, credit, tontine, etc.)
	ReferenceType *string `json:"reference_type,omitempty"`
	ReferenceID   *string `json:"reference_id,omitempty"`

	// Description optionnelle
	Description *string `json:"description,omitempty"`
}

// CreditWalletResponse représente la réponse après crédit
type CreditWalletResponse struct {
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
}

// Validate valide la requête
func (r *CreditWalletRequest) Validate() error {
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if r.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	if !r.TransactionType.IsValid() {
		return fmt.Errorf("invalid transaction type: %s", r.TransactionType)
	}
	if !r.TransactionType.IsCredit() {
		return fmt.Errorf("transaction type must be a credit type, got: %s", r.TransactionType)
	}
	return nil
}

// CreditWalletUsecase crédite le wallet d'un marchand
type CreditWalletUsecase struct {
	walletRepo repository.MerchantWalletRepository
	txnRepo    repository.WalletTransactionRepository
	txManager  repository.TxManager
}

// NewCreditWalletUsecase crée une nouvelle instance
func NewCreditWalletUsecase(
	walletRepo repository.MerchantWalletRepository,
	txnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
) *CreditWalletUsecase {
	return &CreditWalletUsecase{
		walletRepo: walletRepo,
		txnRepo:    txnRepo,
		txManager:  txManager,
	}
}

// Execute crédite le wallet et enregistre la transaction
func (uc *CreditWalletUsecase) Execute(ctx context.Context, req *CreditWalletRequest) (*CreditWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid credit wallet request")
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
		// Wallet n'existe pas, le créer
		if err.Error() == "merchant wallet not found" {
			wallet = entity.NewMerchantWallet(req.ShopID)
			if err := uc.walletRepo.WithTX(tx).Create(ctx, wallet); err != nil {
				logger.Error().Err(err).Msg("Failed to create wallet")
				return nil, fmt.Errorf("failed to create wallet: %w", err)
			}
			logger.Info().Str("shop_id", req.ShopID).Msg("Wallet created")
		} else {
			logger.Error().Err(err).Msg("Failed to find wallet")
			return nil, fmt.Errorf("failed to find wallet: %w", err)
		}
	}

	// 5. Vérifier que le wallet n'est pas gelé
	if wallet.IsFrozen {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Bool("is_frozen", wallet.IsFrozen).
			Msg("Attempted to credit frozen wallet")
		return nil, fmt.Errorf("wallet is frozen, cannot credit")
	}

	// 6. Sauvegarder le solde précédent
	previousBalance := wallet.BalanceCents

	// 7. Créditer le wallet
	if err := wallet.Credit(req.AmountCents); err != nil {
		logger.Error().Err(err).Msg("Failed to credit wallet")
		return nil, fmt.Errorf("failed to credit wallet: %w", err)
	}

	// 8. Mettre à jour le wallet dans la base
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// 9. Créer la transaction
	txnID := uuid.New().String()
	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            req.ShopID,
		TransactionType:   req.TransactionType,
		AmountCents:       req.AmountCents,
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

	// 10. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Logger le succès
	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("amount_cents", req.AmountCents).
		Int64("previous_balance", previousBalance).
		Int64("new_balance", wallet.BalanceCents).
		Str("transaction_type", string(req.TransactionType)).
		Str("transaction_id", txnID).
		Msg("Wallet credited successfully")

	// 12. Retourner la réponse
	return &CreditWalletResponse{
		ShopID:            wallet.ShopID,
		BalanceCents:      wallet.BalanceCents,
		PreviousBalance:   previousBalance,
		IsFrozen:          wallet.IsFrozen,
		TransactionID:     txnID,
		TransactionType:   req.TransactionType,
		AmountCents:       req.AmountCents,
		BalanceAfterCents: wallet.BalanceCents,
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// CreditFromSale crédite le wallet suite à une vente
func (uc *CreditWalletUsecase) CreditFromSale(
	ctx context.Context,
	shopID string,
	amountCents int64,
	orderID string,
) (*CreditWalletResponse, error) {
	refType := "order"
	description := fmt.Sprintf("Sale from order %s", orderID)

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxSaleCredit,
		ReferenceType:   &refType,
		ReferenceID:     &orderID,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}

// CreditFromCOD crédite le wallet suite à une vente COD
func (uc *CreditWalletUsecase) CreditFromCOD(
	ctx context.Context,
	shopID string,
	amountCents int64,
	orderID string,
) (*CreditWalletResponse, error) {
	refType := "order"
	description := fmt.Sprintf("COD sale from order %s", orderID)

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxCOD,
		ReferenceType:   &refType,
		ReferenceID:     &orderID,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}

// CreditFromTontine crédite le wallet suite à une vente tontine
func (uc *CreditWalletUsecase) CreditFromTontine(
	ctx context.Context,
	shopID string,
	amountCents int64,
	groupID string,
) (*CreditWalletResponse, error) {
	refType := "tontine_group"
	description := fmt.Sprintf("Tontine sale from group %s", groupID)

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxSaleTontine,
		ReferenceType:   &refType,
		ReferenceID:     &groupID,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}

// CreditFromCreditPlan crédite le wallet suite à un paiement de plan de crédit
func (uc *CreditWalletUsecase) CreditFromCreditPlan(
	ctx context.Context,
	shopID string,
	amountCents int64,
	contractID string,
) (*CreditWalletResponse, error) {
	refType := "credit_contract"
	description := fmt.Sprintf("Credit plan payment from contract %s", contractID)

	var refID *string
	if contractID != "" {
		refID = &contractID
	}

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxSaleCreditPlan,
		ReferenceType:   &refType,
		ReferenceID:     refID,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}

// CreditFromDeposit crédite le wallet suite à un dépôt manuel
func (uc *CreditWalletUsecase) CreditFromDeposit(
	ctx context.Context,
	shopID string,
	amountCents int64,
	description string,
) (*CreditWalletResponse, error) {
	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxDeposit,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}

// CreditFromUnfreeze crédite le wallet suite à un dépôt pour dégeler
func (uc *CreditWalletUsecase) CreditFromUnfreeze(
	ctx context.Context,
	shopID string,
	amountCents int64,
) (*CreditWalletResponse, error) {
	description := fmt.Sprintf("Deposit to unfreeze wallet: %d FCFA", amountCents/100)

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxUnfreezeDeposit,
		Description:     &description,
	}

	return uc.Execute(ctx, req)
}
