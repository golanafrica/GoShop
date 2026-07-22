package walletusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// FREEZE ACCOUNT USECASE
// ============================================================

// FreezeAccountRequest représente la requête pour geler un compte
type FreezeAccountRequest struct {
	// Identifiants
	ShopID string `json:"shop_id"`

	// Raison du gel
	Reason entity.FreezeReason `json:"reason"`

	// Montant dû (en centimes, positif)
	AmountDueCents int64 `json:"amount_due_cents"`

	// Détails optionnels
	Details *string `json:"details,omitempty"`

	// Période de grâce personnalisée (défaut: 7 jours)
	GracePeriodDays *int `json:"grace_period_days,omitempty"`
}

// FreezeAccountResponse représente la réponse après gel
type FreezeAccountResponse struct {
	// Freeze créé
	FreezeID          string              `json:"freeze_id"`
	ShopID            string              `json:"shop_id"`
	Reason            entity.FreezeReason `json:"reason"`
	AmountDueCents    int64               `json:"amount_due_cents"`
	FrozenAt          time.Time           `json:"frozen_at"`
	GracePeriodEndsAt time.Time           `json:"grace_period_ends_at"`
	DaysRemaining     int                 `json:"days_remaining"`

	// Wallet mis à jour
	WalletBalanceCents int64 `json:"wallet_balance_cents"`
	WalletIsFrozen     bool  `json:"wallet_is_frozen"`
}

// Validate valide la requête
func (r *FreezeAccountRequest) Validate() error {
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if !r.Reason.IsValid() {
		return fmt.Errorf("invalid freeze reason: %s", r.Reason)
	}
	if r.AmountDueCents <= 0 {
		return fmt.Errorf("amount_due_cents must be positive")
	}
	if r.GracePeriodDays != nil && *r.GracePeriodDays <= 0 {
		return fmt.Errorf("grace_period_days must be positive")
	}
	return nil
}

// FreezeAccountUsecase gèle le compte d'un marchand
type FreezeAccountUsecase struct {
	walletRepo repository.MerchantWalletRepository
	freezeRepo repository.AccountFreezeRepository
	txManager  repository.TxManager
}

// NewFreezeAccountUsecase crée une nouvelle instance
func NewFreezeAccountUsecase(
	walletRepo repository.MerchantWalletRepository,
	freezeRepo repository.AccountFreezeRepository,
	txManager repository.TxManager,
) *FreezeAccountUsecase {
	return &FreezeAccountUsecase{
		walletRepo: walletRepo,
		freezeRepo: freezeRepo,
		txManager:  txManager,
	}
}

// Execute gèle le compte et crée un AccountFreeze
func (uc *FreezeAccountUsecase) Execute(ctx context.Context, req *FreezeAccountRequest) (*FreezeAccountResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid freeze account request")
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

	// 4. Vérifier qu'il n'y a pas déjà un gel actif
	existingFreeze, err := uc.freezeRepo.WithTX(tx).FindActiveByShopID(ctx, req.ShopID)
	if err == nil && existingFreeze != nil {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Str("existing_freeze_id", existingFreeze.ID).
			Msg("Account already has an active freeze")
		return nil, fmt.Errorf("account already has an active freeze: %s", existingFreeze.ID)
	}

	// 5. 🛡️ SÉCURITÉ : Récupérer le wallet avec verrouillage (FOR UPDATE)
	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find wallet")
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// 6. Vérifier que le wallet n'est pas déjà gelé
	if wallet.IsFrozen {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Msg("Attempted to freeze already frozen wallet")
		return nil, fmt.Errorf("wallet is already frozen")
	}

	// 7. Geler le wallet
	gracePeriodDays := entity.DefaultGracePeriodDays
	if req.GracePeriodDays != nil {
		gracePeriodDays = *req.GracePeriodDays
	}

	if err := wallet.Freeze(req.Reason, *req.Details); err != nil {
		logger.Error().Err(err).Msg("Failed to freeze wallet")
		return nil, fmt.Errorf("failed to freeze wallet: %w", err)
	}

	// 8. Mettre à jour le wallet dans la base
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// 9. Créer l'AccountFreeze
	freeze, err := entity.NewAccountFreeze(req.ShopID, req.Reason, req.AmountDueCents, req.Details)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create account freeze entity")
		return nil, fmt.Errorf("failed to create account freeze: %w", err)
	}

	// Personnaliser la période de grâce si demandée
	if req.GracePeriodDays != nil {
		freeze.GracePeriodDays = *req.GracePeriodDays
		freeze.GracePeriodEndsAt = freeze.FrozenAt.AddDate(0, 0, *req.GracePeriodDays)
	}

	// 10. Sauvegarder le freeze dans la base
	if err := uc.freezeRepo.WithTX(tx).Create(ctx, freeze); err != nil {
		logger.Error().Err(err).Msg("Failed to save account freeze")
		return nil, fmt.Errorf("failed to save account freeze: %w", err)
	}

	// 11. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Logger le succès
	logger.Warn().
		Str("shop_id", req.ShopID).
		Str("freeze_id", freeze.ID).
		Str("reason", string(req.Reason)).
		Int64("amount_due_cents", req.AmountDueCents).
		Time("grace_period_ends_at", freeze.GracePeriodEndsAt).
		Int("grace_period_days", gracePeriodDays).
		Msg("Account frozen successfully")

	// 13. Retourner la réponse
	return &FreezeAccountResponse{
		FreezeID:           freeze.ID,
		ShopID:             req.ShopID,
		Reason:             req.Reason,
		AmountDueCents:     req.AmountDueCents,
		FrozenAt:           freeze.FrozenAt,
		GracePeriodEndsAt:  freeze.GracePeriodEndsAt,
		DaysRemaining:      freeze.DaysUntilExpiration(),
		WalletBalanceCents: wallet.BalanceCents,
		WalletIsFrozen:     wallet.IsFrozen,
	}, nil
}

// ============================================================
// UNFREEZE ACCOUNT USECASE
// ============================================================

// UnfreezeAccountRequest représente la requête pour dégeler un compte
type UnfreezeAccountRequest struct {
	// Identifiants
	ShopID string `json:"shop_id"`

	// Montant payé pour dégeler (en centimes)
	DepositAmountCents int64 `json:"deposit_amount_cents"`

	// Résolution du gel
	Resolution entity.FreezeResolution `json:"resolution"`

	// ID de l'utilisateur qui effectue l'action
	ResolvedBy string `json:"resolved_by"`
}

// UnfreezeAccountResponse représente la réponse après dégel
type UnfreezeAccountResponse struct {
	// Freeze résolu
	FreezeID   string                  `json:"freeze_id"`
	ShopID     string                  `json:"shop_id"`
	Resolution entity.FreezeResolution `json:"resolution"`
	ResolvedAt time.Time               `json:"resolved_at"`

	// Wallet mis à jour
	WalletBalanceCents int64 `json:"wallet_balance_cents"`
	WalletIsFrozen     bool  `json:"wallet_is_frozen"`

	// Transaction créée (si dépôt)
	TransactionID      string `json:"transaction_id,omitempty"`
	DepositAmountCents int64  `json:"deposit_amount_cents,omitempty"`
}

// Validate valide la requête
func (r *UnfreezeAccountRequest) Validate() error {
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if r.ResolvedBy == "" {
		return fmt.Errorf("resolved_by is required")
	}
	if !r.Resolution.IsValid() {
		return fmt.Errorf("invalid resolution: %s", r.Resolution)
	}
	if r.Resolution == entity.FreezeResolutionPaid && r.DepositAmountCents <= 0 {
		return fmt.Errorf("deposit_amount_cents must be positive for paid resolution")
	}
	return nil
}

// UnfreezeAccountUsecase dégèle le compte d'un marchand
type UnfreezeAccountUsecase struct {
	walletRepo repository.MerchantWalletRepository
	freezeRepo repository.AccountFreezeRepository
	txnRepo    repository.WalletTransactionRepository
	txManager  repository.TxManager
}

// NewUnfreezeAccountUsecase crée une nouvelle instance
func NewUnfreezeAccountUsecase(
	walletRepo repository.MerchantWalletRepository,
	freezeRepo repository.AccountFreezeRepository,
	txnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
) *UnfreezeAccountUsecase {
	return &UnfreezeAccountUsecase{
		walletRepo: walletRepo,
		freezeRepo: freezeRepo,
		txnRepo:    txnRepo,
		txManager:  txManager,
	}
}

// Execute dégèle le compte et résout le gel
func (uc *UnfreezeAccountUsecase) Execute(ctx context.Context, req *UnfreezeAccountRequest) (*UnfreezeAccountResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid unfreeze account request")
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

	// 4. Récupérer le gel actif
	freeze, err := uc.freezeRepo.WithTX(tx).FindActiveByShopID(ctx, req.ShopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find active freeze")
		return nil, fmt.Errorf("no active freeze found for this shop")
	}

	// 5. 🛡️ SÉCURITÉ : Récupérer le wallet avec verrouillage (FOR UPDATE)
	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find wallet")
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// 6. Vérifier que le wallet est bien gelé
	if !wallet.IsFrozen {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Msg("Attempted to unfreeze non-frozen wallet")
		return nil, fmt.Errorf("wallet is not frozen")
	}

	// 7. 🛠️ CORRECTION CRITIQUE : Dégeler le wallet EN PREMIER pour permettre les opérations suivantes (comme le crédit)
	if err := wallet.ForceUnfreeze(); err != nil {
		logger.Error().Err(err).Msg("Failed to unfreeze wallet")
		return nil, fmt.Errorf("failed to unfreeze wallet: %w", err)
	}

	// 8. Si résolution par paiement, créditer le wallet
	var transactionID string
	if req.Resolution == entity.FreezeResolutionPaid && req.DepositAmountCents > 0 {
		// Vérifier que le dépôt couvre la dette
		if req.DepositAmountCents < freeze.AmountDueCents {
			logger.Warn().
				Int64("deposit", req.DepositAmountCents).
				Int64("amount_due", freeze.AmountDueCents).
				Msg("Deposit does not cover full debt")
			// On continue quand même, le marchand peut payer partiellement
		}

		// Créditer le wallet (maintenant que IsFrozen = false, cela fonctionnera)
		if err := wallet.Credit(req.DepositAmountCents); err != nil {
			logger.Error().Err(err).Msg("Failed to credit wallet")
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}

		// Créer la transaction
		transactionID = uuid.New().String()
		refType := "unfreeze_deposit"
		description := fmt.Sprintf("Deposit to unfreeze account: %d FCFA", req.DepositAmountCents/100)

		txn := &entity.WalletTransaction{
			ID:                transactionID,
			ShopID:            req.ShopID,
			TransactionType:   entity.WalletTxUnfreezeDeposit,
			AmountCents:       req.DepositAmountCents,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &refType,
			ReferenceID:       &freeze.ID,
			Description:       &description,
			Status:            entity.WalletTxCompleted,
		}

		if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
			logger.Error().Err(err).Msg("Failed to create transaction")
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	}

	// 9. Mettre à jour le wallet dans la base
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// 10. Résoudre le gel
	if err := uc.freezeRepo.WithTX(tx).UpdateResolution(ctx, freeze.ID, req.Resolution, req.ResolvedBy); err != nil {
		logger.Error().Err(err).Msg("Failed to resolve freeze")
		return nil, fmt.Errorf("failed to resolve freeze: %w", err)
	}

	// 11. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Logger le succès
	logger.Info().
		Str("shop_id", req.ShopID).
		Str("freeze_id", freeze.ID).
		Str("resolution", string(req.Resolution)).
		Int64("deposit_amount_cents", req.DepositAmountCents).
		Str("transaction_id", transactionID).
		Msg("Account unfrozen successfully")

	// 13. Retourner la réponse
	return &UnfreezeAccountResponse{
		FreezeID:           freeze.ID,
		ShopID:             req.ShopID,
		Resolution:         req.Resolution,
		ResolvedAt:         time.Now().UTC(),
		WalletBalanceCents: wallet.BalanceCents,
		WalletIsFrozen:     wallet.IsFrozen,
		TransactionID:      transactionID,
		DepositAmountCents: req.DepositAmountCents,
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// FreezeForNegativeBalance gèle le compte suite à un solde négatif
func (uc *FreezeAccountUsecase) FreezeForNegativeBalance(
	ctx context.Context,
	shopID string,
	amountDueCents int64,
) (*FreezeAccountResponse, error) {
	reason := entity.FreezeReasonNegativeBalance
	details := fmt.Sprintf("Wallet balance is negative: -%d FCFA", amountDueCents/100)

	req := &FreezeAccountRequest{
		ShopID:         shopID,
		Reason:         reason,
		AmountDueCents: amountDueCents,
		Details:        &details,
	}

	return uc.Execute(ctx, req)
}

// FreezeForUnpaidCommission gèle le compte suite à une commission impayée
func (uc *FreezeAccountUsecase) FreezeForUnpaidCommission(
	ctx context.Context,
	shopID string,
	amountDueCents int64,
	orderID string,
) (*FreezeAccountResponse, error) {
	reason := entity.FreezeReasonUnpaidCommission
	details := fmt.Sprintf("Commission for order %s could not be collected", orderID)

	req := &FreezeAccountRequest{
		ShopID:         shopID,
		Reason:         reason,
		AmountDueCents: amountDueCents,
		Details:        &details,
	}

	return uc.Execute(ctx, req)
}

// FreezeForFraud gèle le compte suite à une fraude suspectée
func (uc *FreezeAccountUsecase) FreezeForFraud(
	ctx context.Context,
	shopID string,
	amountDueCents int64,
	reason string,
) (*FreezeAccountResponse, error) {
	freezeReason := entity.FreezeReasonFraudSuspected

	req := &FreezeAccountRequest{
		ShopID:         shopID,
		Reason:         freezeReason,
		AmountDueCents: amountDueCents,
		Details:        &reason,
	}

	return uc.Execute(ctx, req)
}

// FreezeByAdmin gèle le compte par décision admin
func (uc *FreezeAccountUsecase) FreezeByAdmin(
	ctx context.Context,
	shopID string,
	amountDueCents int64,
	reason string,
) (*FreezeAccountResponse, error) {
	freezeReason := entity.FreezeReasonAdminDecision

	req := &FreezeAccountRequest{
		ShopID:         shopID,
		Reason:         freezeReason,
		AmountDueCents: amountDueCents,
		Details:        &reason,
	}

	return uc.Execute(ctx, req)
}

// ============================================================
// DEBIT WITH AUTO-FREEZE
// ============================================================

// DebitWithAutoFreeze débite et gèle automatiquement si nécessaire
func DebitWithAutoFreeze(
	ctx context.Context,
	debitUC *DebitWalletUsecase,
	freezeUC *FreezeAccountUsecase,
	req *DebitWalletRequest,
) (*DebitWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Effectuer le débit
	resp, err := debitUC.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// 2. Si le wallet doit être gelé, le geler
	if resp.ShouldFreeze && freezeUC != nil {
		logger.Info().
			Str("shop_id", req.ShopID).
			Int64("amount_due", -resp.BalanceCents).
			Msg("Auto-freezing wallet due to negative balance")

		if _, err := freezeUC.FreezeForNegativeBalance(ctx, req.ShopID, -resp.BalanceCents); err != nil {
			logger.Error().Err(err).Msg("Failed to auto-freeze wallet")
			// On ne retourne pas l'erreur car le débit a réussi
			// Le gel peut être fait manuellement plus tard
		}
	}

	return resp, nil
}
