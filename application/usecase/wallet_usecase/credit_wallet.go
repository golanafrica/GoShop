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

// ============================================================
// CREDIT WALLET USECASE
// ============================================================

// CreditWalletRequest représente la requête pour créditer un wallet
type CreditWalletRequest struct {
	ShopID string `json:"shop_id"`

	AmountCents int64 `json:"amount_cents"`

	TransactionType entity.WalletTransactionType `json:"transaction_type"`

	ReferenceType *string `json:"reference_type,omitempty"`
	ReferenceID   *string `json:"reference_id,omitempty"`
	Description   *string `json:"description,omitempty"`

	// Phase 2 : si true, crédit ledger + gel immédiat (held_cents += amount).
	// Utilisé pour tontine : non retirable tant que preuve / redeem.
	HoldAfterCredit bool `json:"hold_after_credit,omitempty"`
}

// CreditWalletResponse représente la réponse après crédit
type CreditWalletResponse struct {
	ShopID          string `json:"shop_id"`
	BalanceCents    int64  `json:"balance_cents"`
	HeldCents       int64  `json:"held_cents"`
	AvailableCents  int64  `json:"available_cents"`
	PreviousBalance int64  `json:"previous_balance"`
	IsFrozen        bool   `json:"is_frozen"`

	TransactionID     string                       `json:"transaction_id"`
	TransactionType   entity.WalletTransactionType `json:"transaction_type"`
	AmountCents       int64                        `json:"amount_cents"`
	BalanceAfterCents int64                        `json:"balance_after_cents"`
	Held              bool                         `json:"held"` // true si HoldAfterCredit
}

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
	escrowRepo repository.EscrowAccountRepository
	txManager  repository.TxManager
}

// NewCreditWalletUsecase crée une nouvelle instance
func NewCreditWalletUsecase(
	walletRepo repository.MerchantWalletRepository,
	txnRepo repository.WalletTransactionRepository,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *CreditWalletUsecase {
	return &CreditWalletUsecase{
		walletRepo: walletRepo,
		txnRepo:    txnRepo,
		escrowRepo: escrowRepo,
		txManager:  txManager,
	}
}

func (uc *CreditWalletUsecase) Execute(ctx context.Context, req *CreditWalletRequest) (*CreditWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid credit wallet request")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	if shop.ID.String() != req.ShopID {
		return nil, fmt.Errorf("access denied: shop_id does not match tenant")
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// ============================================================
	// 🛡️ SÉCURITÉ : Vérifier l'état de l'escrow (orders / credit only)
	// tontine_cycle / tontine_group : crédit held en fin de cycle — PAS de gate escrow
	// ============================================================
	if req.ReferenceType != nil && req.ReferenceID != nil && uc.escrowRepo != nil {
		switch *req.ReferenceType {
		case "order", "credit_contract", "escrow_release":
			var escrow *entity.EscrowAccount
			var findErr error

			switch *req.ReferenceType {
			case "order":
				escrow, findErr = uc.escrowRepo.WithTX(tx).FindByOrderID(ctx, *req.ReferenceID)
			case "credit_contract":
				escrow, findErr = uc.escrowRepo.WithTX(tx).FindByCreditContractID(ctx, *req.ReferenceID)
			case "escrow_release":
				escrow, findErr = uc.escrowRepo.WithTX(tx).FindByID(ctx, *req.ReferenceID)
			}

			if findErr == nil && escrow != nil {
				if escrow.IsBlockedFromRelease() {
					logger.Error().
						Str("shop_id", req.ShopID).
						Str("escrow_id", escrow.ID).
						Str("escrow_status", string(escrow.Status)).
						Str("reference_type", *req.ReferenceType).
						Str("reference_id", *req.ReferenceID).
						Msg("🚨 BLOCKED: Attempted to credit wallet while escrow is still locked")

					return nil, fmt.Errorf(
						"cannot credit wallet: escrow %s is locked (status: %s). Funds must be released first via delivery confirmation or auto-release after 3 days",
						escrow.ID,
						escrow.Status,
					)
				}

				if !escrow.CanReleaseToMerchant() {
					logger.Error().
						Str("escrow_id", escrow.ID).
						Str("escrow_status", string(escrow.Status)).
						Msg("Escrow cannot release funds to merchant")

					return nil, fmt.Errorf(
						"escrow %s cannot release funds (status: %s)",
						escrow.ID,
						escrow.Status,
					)
				}

				logger.Info().
					Str("escrow_id", escrow.ID).
					Str("escrow_status", string(escrow.Status)).
					Int64("escrow_total", escrow.TotalAmountCents).
					Int64("escrow_released", escrow.ReleasedAmountCents).
					Msg("✅ Escrow verified and allows wallet credit")
			}
		}
	}

	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
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

	if wallet.IsFrozen {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Bool("is_frozen", wallet.IsFrozen).
			Msg("Attempted to credit frozen wallet")
		return nil, fmt.Errorf("wallet is frozen, cannot credit")
	}

	// ============================================================
	// IDEMPOTENCE : déjà crédité pour cette référence ?
	// ============================================================
	if req.ReferenceType != nil && req.ReferenceID != nil {
		existing, findErr := uc.txnRepo.WithTX(tx).FindByReferenceID(ctx, *req.ReferenceType, *req.ReferenceID)
		if findErr == nil && existing != nil && existing.Status == entity.WalletTxCompleted {
			logger.Info().
				Str("ref_type", *req.ReferenceType).
				Str("ref_id", *req.ReferenceID).
				Str("existing_txn", existing.ID).
				Msg("⏭️ Idempotent skip — credit already completed")
			// Commit vide (rien modifié) pour libérer le FOR UPDATE proprement
			if err := tx.Commit(); err != nil {
				_ = tx.Rollback()
			}
			return &CreditWalletResponse{
				ShopID:            wallet.ShopID,
				BalanceCents:      wallet.BalanceCents,
				HeldCents:         wallet.HeldCents,
				AvailableCents:    wallet.AvailableCents(),
				PreviousBalance:   wallet.BalanceCents,
				IsFrozen:          wallet.IsFrozen,
				TransactionID:     existing.ID,
				TransactionType:   existing.TransactionType,
				AmountCents:       existing.AmountCents,
				BalanceAfterCents: existing.BalanceAfterCents,
				Held:              req.HoldAfterCredit,
			}, nil
		}
	}

	previousBalance := wallet.BalanceCents
	var swept int64 = 0

	if req.HoldAfterCredit {
		if err := wallet.CreditAndHold(req.AmountCents); err != nil {
			logger.Error().Err(err).Msg("Failed to credit-and-hold wallet")
			return nil, fmt.Errorf("failed to credit-and-hold wallet: %w", err)
		}
	} else {
		// 🆕 CORRECTION AUDIT : Tout crédit standard doit d'abord éponger la dette
		var netToBalance int64
		netToBalance, swept, err = wallet.CreditWithDebtSweep(req.AmountCents)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to credit wallet with debt sweep")
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}

		// On met à jour req.AmountCents pour la transaction principale afin qu'elle reflète le net ajouté au solde
		// (Le montant "swept" sera enregistré dans une transaction debt_sweep séparée)
		req.AmountCents = netToBalance
	}

	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	txnID := uuid.New().String()
	desc := req.Description
	if req.HoldAfterCredit && desc != nil {
		heldNote := *desc + " (held until proof/redeem)"
		desc = &heldNote
	} else if swept > 0 && desc != nil {
		adjDesc := fmt.Sprintf("%s (Dette réduite: %d XOF)", *desc, swept/100)
		desc = &adjDesc
	}

	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            req.ShopID,
		TransactionType:   req.TransactionType,
		AmountCents:       req.AmountCents, // Net après sweep si applicable
		BalanceAfterCents: wallet.BalanceCents,
		ReferenceType:     req.ReferenceType,
		ReferenceID:       req.ReferenceID,
		Description:       desc,
		Status:            entity.WalletTxCompleted,
	}

	if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate key") ||
			strings.Contains(msg, "unique constraint") ||
			strings.Contains(msg, "uq_wallet_txn_ref_completed") ||
			strings.Contains(msg, "23505") {
			logger.Info().Msg("⏭️ Unique constraint — concurrent credit, treat as success")
			_ = tx.Rollback()
			return &CreditWalletResponse{
				ShopID:          req.ShopID,
				AmountCents:     req.AmountCents,
				TransactionType: req.TransactionType,
				Held:            req.HoldAfterCredit,
			}, nil
		}
		logger.Error().Err(err).Msg("Failed to create transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 🆕 Si une dette a été réduite, on enregistre une transaction d'audit "debt_sweep"
	if swept > 0 {
		sweepRefType := "debt_sweep"
		sweepDesc := fmt.Sprintf("Prélèvement auto sur dette pour crédit %s", req.TransactionType)
		sweepTxn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            req.ShopID,
			TransactionType:   entity.WalletTxDebtSweep,
			AmountCents:       -swept,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &sweepRefType,
			ReferenceID:       req.ReferenceID,
			Description:       &sweepDesc,
			Status:            entity.WalletTxCompleted,
		}
		if err := uc.txnRepo.WithTX(tx).Create(ctx, sweepTxn); err != nil {
			logger.Warn().Err(err).Msg("Failed to create debt_sweep audit transaction")
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("amount_cents", req.AmountCents).
		Int64("previous_balance", previousBalance).
		Int64("new_balance", wallet.BalanceCents).
		Int64("held_cents", wallet.HeldCents).
		Int64("available_cents", wallet.AvailableCents()).
		Bool("held", req.HoldAfterCredit).
		Str("transaction_type", string(req.TransactionType)).
		Str("transaction_id", txnID).
		Msg("Wallet credited successfully")

	return &CreditWalletResponse{
		ShopID:            wallet.ShopID,
		BalanceCents:      wallet.BalanceCents,
		HeldCents:         wallet.HeldCents,
		AvailableCents:    wallet.AvailableCents(),
		PreviousBalance:   previousBalance,
		IsFrozen:          wallet.IsFrozen,
		TransactionID:     txnID,
		TransactionType:   req.TransactionType,
		AmountCents:       req.AmountCents,
		BalanceAfterCents: wallet.BalanceCents,
		Held:              req.HoldAfterCredit,
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

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

// CreditFromTontine crédite le NET du cycle et le met en held (non retirable).
// reference_id = "{groupID}:cycle:{N}" pour uq_wallet_txn_ref_completed
// (un groupe a plusieurs cycles → un crédit distinct par cycle).
func (uc *CreditWalletUsecase) CreditFromTontine(
	ctx context.Context,
	shopID string,
	amountCents int64,
	groupID string,
	cycleNumber int,
) (*CreditWalletResponse, error) {
	refType := "tontine_cycle"
	refID := fmt.Sprintf("%s:cycle:%d", groupID, cycleNumber)
	description := fmt.Sprintf("Tontine cycle %d settlement (net) group %s", cycleNumber, groupID)

	req := &CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     amountCents,
		TransactionType: entity.WalletTxSaleTontine,
		ReferenceType:   &refType,
		ReferenceID:     &refID,
		Description:     &description,
		HoldAfterCredit: true,
	}

	return uc.Execute(ctx, req)
}

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
