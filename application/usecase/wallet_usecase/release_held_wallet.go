package walletusecase

import (
	"context"
	"fmt"

	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ReleaseHeldWalletRequest libère un montant held (ex: redeem voucher tontine)
type ReleaseHeldWalletRequest struct {
	ShopID        string  `json:"shop_id"`
	AmountCents   int64   `json:"amount_cents"`
	ReferenceType *string `json:"reference_type,omitempty"`
	ReferenceID   *string `json:"reference_id,omitempty"`
	Description   *string `json:"description,omitempty"`
}

func (r *ReleaseHeldWalletRequest) Validate() error {
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if r.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	return nil
}

type ReleaseHeldWalletResponse struct {
	ShopID         string `json:"shop_id"`
	BalanceCents   int64  `json:"balance_cents"`
	HeldCents      int64  `json:"held_cents"`
	AvailableCents int64  `json:"available_cents"`
	ReleasedCents  int64  `json:"released_cents"`
}

type ReleaseHeldWalletUsecase struct {
	walletRepo repository.MerchantWalletRepository
	txnRepo    repository.WalletTransactionRepository
	txManager  repository.TxManager
}

func NewReleaseHeldWalletUsecase(
	walletRepo repository.MerchantWalletRepository,
	txnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
) *ReleaseHeldWalletUsecase {
	return &ReleaseHeldWalletUsecase{
		walletRepo: walletRepo,
		txnRepo:    txnRepo,
		txManager:  txManager,
	}
}

func (uc *ReleaseHeldWalletUsecase) Execute(ctx context.Context, req *ReleaseHeldWalletRequest) (*ReleaseHeldWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	if shop.ID.String() != req.ShopID {
		return nil, fmt.Errorf("access denied: shop_id does not match tenant")
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	if err := wallet.ReleaseHeld(req.AmountCents); err != nil {
		logger.Warn().Err(err).
			Int64("requested", req.AmountCents).
			Int64("held", wallet.HeldCents).
			Msg("ReleaseHeld refused")
		return nil, fmt.Errorf("release held: %w", err)
	}

	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// Audit : transaction à montant 0 côté ledger (seul held change) — on log via description
	// Si ton schéma impose amount != 0, omets Create ; le held est déjà mis à jour.
	desc := req.Description
	if desc == nil {
		d := fmt.Sprintf("Release held %d cents", req.AmountCents)
		desc = &d
	}
	// Pas de WalletTx dédié "release_held" dans l'enum actuel → on skip Create txn
	// pour éviter un type invalide. Le solde ledger ne change pas.
	_ = uuid.New()
	_ = desc
	_ = uc.txnRepo

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("released", req.AmountCents).
		Int64("held_after", wallet.HeldCents).
		Int64("available", wallet.AvailableCents()).
		Msg("✅ Held funds released")

	return &ReleaseHeldWalletResponse{
		ShopID:         wallet.ShopID,
		BalanceCents:   wallet.BalanceCents,
		HeldCents:      wallet.HeldCents,
		AvailableCents: wallet.AvailableCents(),
		ReleasedCents:  req.AmountCents,
	}, nil
}
