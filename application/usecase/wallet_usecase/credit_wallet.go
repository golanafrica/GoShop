package walletusecase

import (
	"context"
	"database/sql"
	"errors"
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
	ShopID          string                       `json:"shop_id"`
	AmountCents     int64                        `json:"amount_cents"`
	TransactionType entity.WalletTransactionType `json:"transaction_type"`
	ReferenceType   *string                      `json:"reference_type,omitempty"`
	ReferenceID     *string                      `json:"reference_id,omitempty"`
	Description     *string                      `json:"description,omitempty"`
	// Phase 2 : si true, crédit ledger + gel immédiat (held_cents += amount).
	// Utilisé pour tontine : non retirable tant que preuve / redeem.
	HoldAfterCredit bool `json:"hold_after_credit,omitempty"`
}

// CreditWalletResponse représente la réponse après crédit
type CreditWalletResponse struct {
	ShopID            string                       `json:"shop_id"`
	BalanceCents      int64                        `json:"balance_cents"`
	HeldCents         int64                        `json:"held_cents"`
	AvailableCents    int64                        `json:"available_cents"`
	PreviousBalance   int64                        `json:"previous_balance"`
	IsFrozen          bool                         `json:"is_frozen"`
	TransactionID     string                       `json:"transaction_id"`
	TransactionType   entity.WalletTransactionType `json:"transaction_type"`
	AmountCents       int64                        `json:"amount_cents"`
	BalanceAfterCents int64                        `json:"balance_after_cents"`
	Held              bool                         `json:"held"`
	SweptCents        int64                        `json:"swept_cents,omitempty"`
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
	defer tx.Rollback() //nolint:errcheck

	// ============================================================
	// ✅ W2 FIX : SÉCURITÉ GATE ESCROW
	// Une erreur de lecture (findErr != nil) doit ABORTER, pas être ignorée.
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

			if findErr != nil {
				if !isWalletTxnNotFound(findErr) {
					logger.Error().
						Err(findErr).
						Str("reference_type", *req.ReferenceType).
						Str("reference_id", *req.ReferenceID).
						Msg("Failed to load escrow for credit gate")
					return nil, fmt.Errorf("failed to verify escrow before credit: %w", findErr)
				}
				// not found : pas d'escrow lié — on laisse le crédit (chemins hors escrow)
			} else if escrow != nil {
				if escrow.IsBlockedFromRelease() {
					logger.Error().
						Str("shop_id", req.ShopID).
						Str("escrow_id", escrow.ID).
						Str("escrow_status", string(escrow.Status)).
						Str("reference_type", *req.ReferenceType).
						Str("reference_id", *req.ReferenceID).
						Msg("BLOCKED: Attempted to credit wallet while escrow is still locked")
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
					Msg("Escrow verified and allows wallet credit")
			}
		}
	}

	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		// ✅ W3 FIX : Utilisation d'un helper robuste au lieu de == "string"
		if isMerchantWalletNotFound(err) {
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

	// Frozen : CreditWithDebtSweep est autorisé pour récupérer une dette.
	if wallet.IsFrozen && wallet.DebtCents == 0 && req.TransactionType != entity.WalletTxUnfreezeDeposit {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Bool("is_frozen", wallet.IsFrozen).
			Msg("Attempted to credit frozen wallet (no outstanding debt)")
		return nil, fmt.Errorf("wallet is frozen, cannot credit")
	}

	// ============================================================
	// ✅ W1 FIX & PR-C FIX : IDEMPOTENCE STRICTE AVANT toute mutation mémoire
	// ============================================================
	if req.ReferenceType != nil && req.ReferenceID != nil {
		existing, findErr := uc.txnRepo.WithTX(tx).FindByReferenceID(ctx, *req.ReferenceType, *req.ReferenceID)
		if findErr != nil {
			if !isWalletTxnNotFound(findErr) {
				logger.Error().
					Err(findErr).
					Str("ref_type", *req.ReferenceType).
					Str("ref_id", *req.ReferenceID).
					Msg("Failed to check existing wallet transaction for idempotency")
				return nil, fmt.Errorf("failed to check existing wallet transaction: %w", findErr)
			}
		} else if existing != nil && existing.Status == entity.WalletTxCompleted {
			if err := assertIdempotentCreditMatch(existing, req); err != nil {
				logger.Error().
					Err(err).
					Str("existing_txn", existing.ID).
					Str("ref_type", *req.ReferenceType).
					Str("ref_id", *req.ReferenceID).
					Msg("Idempotency key reuse with mismatched credit parameters")
				return nil, err
			}

			logger.Info().
				Str("ref_type", *req.ReferenceType).
				Str("ref_id", *req.ReferenceID).
				Str("existing_txn", existing.ID).
				Msg("Idempotent skip — credit already completed")
			if err := tx.Commit(); err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to commit after idempotent skip: %w", err)
			}
			// Soldes du wallet chargé (non muté) + txn existante
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

	// ============================================================
	// CRÉDIT MÉMOIRE (uniquement si pas de skip idempotent)
	// ============================================================
	previousBalance := wallet.BalanceCents
	grossCents := req.AmountCents
	var netToBalance int64
	var swept int64

	if req.HoldAfterCredit {
		if err := wallet.CreditAndHold(grossCents); err != nil {
			logger.Error().Err(err).Msg("Failed to credit-and-hold wallet")
			return nil, fmt.Errorf("failed to credit-and-hold wallet: %w", err)
		}
		netToBalance = grossCents
		swept = 0
	} else {
		netToBalance, swept, err = wallet.CreditWithDebtSweep(grossCents)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to credit wallet with debt sweep")
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}
	}

	// ============================================================
	// MISE À JOUR ET CRÉATION DE LA TRANSACTION
	// ============================================================
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		logger.Error().Err(err).Msg("Failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	txnID := uuid.New().String()
	desc := req.Description
	if req.HoldAfterCredit && desc != nil {
		heldNote := *desc + " (held until proof/redeem)"
		desc = &heldNote
	} else if swept > 0 {
		base := ""
		if desc != nil {
			base = *desc + " | "
		}
		adj := fmt.Sprintf("%sdebt_sweep %d cents (gross=%d net=%d)", base, swept, grossCents, netToBalance)
		desc = &adj
	}

	if netToBalance > 0 {
		txn := &entity.WalletTransaction{
			ID:                txnID,
			ShopID:            req.ShopID,
			TransactionType:   req.TransactionType,
			AmountCents:       netToBalance,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     req.ReferenceType,
			ReferenceID:       req.ReferenceID,
			Description:       desc,
			Status:            entity.WalletTxCompleted,
		}

		if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
			// ✅ W1 FIX (partie 2) : GESTION PROPRE DU CONFLIT UNIQUE (23505)
			if isUniqueViolation(err) {
				logger.Info().Msg("Unique constraint on wallet txn — reloading winner")
				_ = tx.Rollback()

				// Nouvelle lecture hors TX avortée (même tenant)
				winner, wErr := uc.txnRepo.FindByReferenceID(ctx, *req.ReferenceType, *req.ReferenceID)
				if wErr != nil {
					return nil, fmt.Errorf("concurrent credit: failed to reload existing transaction: %w", wErr)
				}
				if winner == nil || winner.Status != entity.WalletTxCompleted {
					return nil, fmt.Errorf("concurrent credit: unique conflict but no completed transaction found")
				}
				if err := assertIdempotentCreditMatch(winner, req); err != nil {
					return nil, err
				}

				// Recharger wallet pour soldes à jour (lecture simple)
				w2, w2err := uc.walletRepo.FindByShopID(ctx, req.ShopID)
				if w2err != nil {
					return nil, fmt.Errorf("concurrent credit: reload wallet: %w", w2err)
				}

				return &CreditWalletResponse{
					ShopID:            w2.ShopID,
					BalanceCents:      w2.BalanceCents,
					HeldCents:         w2.HeldCents,
					AvailableCents:    w2.AvailableCents(),
					PreviousBalance:   w2.BalanceCents,
					IsFrozen:          w2.IsFrozen,
					TransactionID:     winner.ID,
					TransactionType:   winner.TransactionType,
					AmountCents:       winner.AmountCents,
					BalanceAfterCents: winner.BalanceAfterCents,
					Held:              req.HoldAfterCredit,
				}, nil
			}
			logger.Error().Err(err).Msg("Failed to create transaction")
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	} else {
		// ✅ CORRECTION PR-C (P0-01) : Full sweep.
		// On enregistre grossCents (et non 0) pour que assertIdempotentCreditMatch
		// réussisse lors d'un retry (car req.AmountCents == grossCents).
		markerDesc := fmt.Sprintf("Crédit de %d FCFA intégralement absorbé par la dette", grossCents)
		markerTxn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            req.ShopID,
			TransactionType:   req.TransactionType,
			AmountCents:       grossCents, // ✅ FIX CRITIQUE : Utiliser grossCents ici
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     req.ReferenceType,
			ReferenceID:       req.ReferenceID,
			Description:       &markerDesc,
			Status:            entity.WalletTxCompleted,
		}

		if err := uc.txnRepo.WithTX(tx).Create(ctx, markerTxn); err != nil {
			if isUniqueViolation(err) {
				logger.Info().Msg("Unique constraint hit on debt sweep marker — already processed")
			} else {
				logger.Error().Err(err).Msg("Failed to create debt sweep marker transaction")
				return nil, fmt.Errorf("failed to create debt sweep marker: %w", err)
			}
		}
		txnID = markerTxn.ID
	}

	if swept > 0 {
		sweepRefType := "debt_sweep"
		var sweepRefID *string
		if req.ReferenceID != nil {
			sweepRefID = req.ReferenceID
		}
		sweepDesc := fmt.Sprintf(
			"Auto debt sweep on credit type=%s gross=%d swept=%d",
			req.TransactionType, grossCents, swept,
		)
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
				logger.Error().Err(err).Msg("Failed to create debt_sweep transaction")
				return nil, fmt.Errorf("failed to create debt_sweep transaction: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("gross_cents", grossCents).
		Int64("net_to_balance", netToBalance).
		Int64("swept_cents", swept).
		Int64("previous_balance", previousBalance).
		Int64("new_balance", wallet.BalanceCents).
		Int64("debt_cents", wallet.DebtCents).
		Int64("held_cents", wallet.HeldCents).
		Int64("available_cents", wallet.AvailableCents()).
		Bool("held", req.HoldAfterCredit).
		Str("transaction_type", string(req.TransactionType)).
		Str("transaction_id", txnID).
		Msg("Wallet credited successfully (with debt sweep if any)")

	return &CreditWalletResponse{
		ShopID:            wallet.ShopID,
		BalanceCents:      wallet.BalanceCents,
		HeldCents:         wallet.HeldCents,
		AvailableCents:    wallet.AvailableCents(),
		PreviousBalance:   previousBalance,
		IsFrozen:          wallet.IsFrozen,
		TransactionID:     txnID,
		TransactionType:   req.TransactionType,
		AmountCents:       netToBalance,
		BalanceAfterCents: wallet.BalanceCents,
		Held:              req.HoldAfterCredit,
		SweptCents:        swept,
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

// ============================================================
// HELPERS
// ============================================================

// isWalletTxnNotFound distingue « aucune transaction » d'une vraie erreur SQL.
func isWalletTxnNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no rows") ||
		strings.Contains(msg, "sql: no rows")
}

// isMerchantWalletNotFound distingue « aucun wallet » d'une vraie erreur SQL.
func isMerchantWalletNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "merchant wallet not found") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no rows")
}

// isUniqueViolation détecte une violation de contrainte unique (23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "uq_wallet_txn_ref_completed") ||
		strings.Contains(msg, "23505")
}

// ✅ PR-C FIX : Logique robuste pour gérer le debt sweep.
// On accepte que le montant enregistré soit INFÉRIEUR ou ÉGAL au montant demandé (sweep légitime).
// On rejette uniquement si le montant enregistré est SUPÉRIEUR (incohérence critique).
func assertIdempotentCreditMatch(existing *entity.WalletTransaction, req *CreditWalletRequest) error {
	if existing.ShopID != req.ShopID {
		return fmt.Errorf("idempotency conflict: shop_id mismatch")
	}
	if existing.TransactionType != req.TransactionType {
		return fmt.Errorf("idempotency conflict: transaction_type mismatch")
	}

	if existing.AmountCents > req.AmountCents {
		return fmt.Errorf("idempotency conflict: amount mismatch (existing=%d > requested=%d)", existing.AmountCents, req.AmountCents)
	}

	return nil
}
