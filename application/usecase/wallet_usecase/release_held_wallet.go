package walletusecase

import (
	"context"
	"fmt"
	"strings"

	"Goshop/domain/entity"
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
	SweptCents     int64  `json:"swept_cents,omitempty"`
	DebtCents      int64  `json:"debt_cents,omitempty"`
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

// Execute libère du held (disponible ↑), puis sweepe la dette résiduelle
// sur le disponible — même sémantique que installment release_escrow_funds :
// les fonds sont déjà dans balance, on ne crédite pas.
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
	defer func() { _ = tx.Rollback() }()

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

	// Debt sweep sur le disponible après libération du held
	var swept int64
	if wallet.DebtCents > 0 {
		available := wallet.AvailableCents()
		if available < 0 {
			available = 0
		}
		swept = wallet.DebtCents
		if available < swept {
			swept = available
		}
		if swept > 0 {
			wallet.DebtCents -= swept
			wallet.BalanceCents -= swept
			if wallet.DebtCents < 0 {
				wallet.DebtCents = 0
			}
		}
	}

	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// Audit ledger debt_sweep (si montant swept > 0)
	if swept > 0 && uc.txnRepo != nil {
		sweepRefType := "debt_sweep"
		var sweepRefID *string
		if req.ReferenceID != nil {
			sweepRefID = req.ReferenceID
		} else {
			id := uuid.New().String()
			sweepRefID = &id
		}
		if req.ReferenceType != nil && *req.ReferenceType != "" {
			// garde la ref métier si fournie, type audit = debt_sweep
			_ = req.ReferenceType
		}
		sweepDesc := fmt.Sprintf(
			"Debt sweep on release_held shop=%s released=%d swept=%d",
			req.ShopID, req.AmountCents, swept,
		)
		if req.Description != nil && *req.Description != "" {
			sweepDesc = *req.Description + " | " + sweepDesc
		}
		sweepAmt := -swept
		sweepTxn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            req.ShopID,
			TransactionType:   entity.WalletTxDebtSweep,
			AmountCents:       sweepAmt,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &sweepRefType,
			ReferenceID:       sweepRefID,
			Description:       &sweepDesc,
			Status:            entity.WalletTxCompleted,
		}
		if err := uc.txnRepo.WithTX(tx).Create(ctx, sweepTxn); err != nil {
			msg := strings.ToLower(err.Error())
			if !(strings.Contains(msg, "duplicate key") ||
				strings.Contains(msg, "unique constraint") ||
				strings.Contains(msg, "23505")) {
				logger.Error().Err(err).Msg("Failed to create debt_sweep on release_held")
				return nil, fmt.Errorf("failed to create debt_sweep transaction: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("released", req.AmountCents).
		Int64("swept_cents", swept).
		Int64("held_after", wallet.HeldCents).
		Int64("balance_cents", wallet.BalanceCents).
		Int64("debt_cents", wallet.DebtCents).
		Int64("available", wallet.AvailableCents()).
		Msg("Held funds released (debt sweep if any)")

	return &ReleaseHeldWalletResponse{
		ShopID:         wallet.ShopID,
		BalanceCents:   wallet.BalanceCents,
		HeldCents:      wallet.HeldCents,
		AvailableCents: wallet.AvailableCents(),
		ReleasedCents:  req.AmountCents,
		SweptCents:     swept,
		DebtCents:      wallet.DebtCents,
	}, nil
}
