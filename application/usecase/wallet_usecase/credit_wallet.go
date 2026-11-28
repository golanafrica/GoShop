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
	"github.com/lib/pq"
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

// Execute crédite le wallet d'un marchand (gère sa propre transaction)
func (uc *CreditWalletUsecase) Execute(ctx context.Context, req *CreditWalletRequest) (*CreditWalletResponse, error) {
	logger := zerolog.Ctx(ctx)

	// ✅ PR-1 FIX : Validation AVANT BeginTx (fail-fast, pas de transaction inutile)
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

	resp, err := uc.ProcessCreditWithinTx(ctx, tx, req)
	if err != nil {
		return nil, err // Le rollback est géré par le defer
	}

	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return resp, nil
}

// ProcessCreditWithinTx exécute la logique de crédit au sein d'une transaction existante.
// 🆕 PR-1 FIX : Permet au scheduler de compensation d'inclure le crédit dans sa propre transaction atomique.
func (uc *CreditWalletUsecase) ProcessCreditWithinTx(ctx context.Context, tx repository.Tx, req *CreditWalletRequest) (*CreditWalletResponse, error) {
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

	// ============================================================
	// ✅ W2 FIX : SÉCURITÉ GATE ESCROW (utilise la TX passée)
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
				if !isNotFound(findErr) {
					return nil, fmt.Errorf("failed to verify escrow before credit: %w", findErr)
				}
			} else if escrow != nil {
				if escrow.IsBlockedFromRelease() {
					return nil, fmt.Errorf(
						"cannot credit wallet: escrow %s is locked (status: %s). Funds must be released first",
						escrow.ID, escrow.Status,
					)
				}
				if !escrow.CanReleaseToMerchant() {
					return nil, fmt.Errorf("escrow %s cannot release funds (status: %s)", escrow.ID, escrow.Status)
				}
			}
		}
	}

	// 1. Trouver ou créer le wallet (avec verrou FOR UPDATE via la TX)
	wallet, err := uc.walletRepo.WithTX(tx).FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		if isNotFound(err) {
			wallet = entity.NewMerchantWallet(req.ShopID)
			if err := uc.walletRepo.WithTX(tx).Create(ctx, wallet); err != nil {
				return nil, fmt.Errorf("failed to create wallet: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to find wallet: %w", err)
		}
	}

	// Frozen check
	if wallet.IsFrozen && wallet.DebtCents == 0 && req.TransactionType != entity.WalletTxUnfreezeDeposit {
		return nil, fmt.Errorf("wallet is frozen, cannot credit")
	}

	// 2. Vérification d'idempotence (via la TX)
	if req.ReferenceType != nil && req.ReferenceID != nil {
		existing, findErr := uc.txnRepo.WithTX(tx).FindByReferenceID(ctx, *req.ReferenceType, *req.ReferenceID)
		if findErr != nil {
			if !isNotFound(findErr) {
				return nil, fmt.Errorf("failed to check existing wallet transaction: %w", findErr)
			}
		} else if existing != nil && existing.Status == entity.WalletTxCompleted {
			if err := assertIdempotentCreditMatch(existing, req); err != nil {
				return nil, err
			}

			// Calcul du previous balance historique pour une réponse précise
			previousBalance := existing.BalanceAfterCents - existing.AmountCents

			// Calcul du swept cents historique (gross - net ledger)
			var sweptCents int64
			if existing.GrossAmountCents != nil && *existing.GrossAmountCents > existing.AmountCents {
				sweptCents = *existing.GrossAmountCents - existing.AmountCents
			}

			// Succès idempotent. Le caller (scheduler ou Execute) fera le Commit de sa propre TX
			// ou retournera simplement la réponse sans nouvelle mutation.
			return &CreditWalletResponse{
				ShopID:            existing.ShopID,
				BalanceCents:      wallet.BalanceCents, // Balance actuelle réelle du wallet
				HeldCents:         wallet.HeldCents,
				AvailableCents:    wallet.AvailableCents(),
				PreviousBalance:   previousBalance,
				IsFrozen:          wallet.IsFrozen,
				TransactionID:     existing.ID,
				TransactionType:   existing.TransactionType,
				AmountCents:       existing.AmountCents,
				BalanceAfterCents: existing.BalanceAfterCents,
				Held:              existing.Description != nil && strings.Contains(*existing.Description, "(held until proof/redeem)"),
				SweptCents:        sweptCents,
			}, nil
		}
	}

	// 3. Crédit en mémoire
	previousBalance := wallet.BalanceCents
	grossCents := req.AmountCents
	var netToBalance int64
	var swept int64

	if req.HoldAfterCredit {
		if err := wallet.CreditAndHold(grossCents); err != nil {
			return nil, fmt.Errorf("failed to credit-and-hold wallet: %w", err)
		}
		netToBalance = grossCents
		swept = 0
	} else {
		netToBalance, swept, err = wallet.CreditWithDebtSweep(grossCents)
		if err != nil {
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}
	}

	// 4. Mise à jour du wallet (via la TX)
	if err := uc.walletRepo.WithTX(tx).Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// ============================================================
	// 5. Ledger : UNIQUEMENT l'effet net sur balance_cents
	//    ✅ PR-5 FIX :
	//    - debt_cents est hors ledger (colonne séparée sur merchant_wallets)
	//    - Full debt sweep  → AmountCents = 0 (balance inchangée)
	//    - Partial sweep    → AmountCents = netToBalance
	//    - UNE seule ligne ledger (plus de +gross / -sweep / double -sweep)
	// ============================================================
	txnID := uuid.New().String()
	desc := req.Description
	if req.HoldAfterCredit && desc != nil {
		heldNote := *desc + " (held until proof/redeem)"
		desc = &heldNote
	} else if swept > 0 && netToBalance == 0 {
		markerDesc := fmt.Sprintf("Crédit de %d centimes intégralement absorbé par la dette (full debt sweep)", grossCents)
		desc = &markerDesc
	} else if swept > 0 {
		base := ""
		if desc != nil {
			base = *desc + " | "
		}
		adj := fmt.Sprintf("%sdebt_sweep %d cents (gross=%d net=%d)", base, swept, grossCents, netToBalance)
		desc = &adj
	}

	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            req.ShopID,
		TransactionType:   req.TransactionType,
		AmountCents:       netToBalance, // 0 si full debt sweep
		GrossAmountCents:  &grossCents,
		BalanceAfterCents: wallet.BalanceCents,
		ReferenceType:     req.ReferenceType,
		ReferenceID:       req.ReferenceID,
		Description:       desc,
		Status:            entity.WalletTxCompleted,
	}

	if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
		if isUniqueViolation(err) {
			// 🆕 PR-1 FIX : Dans une TX partagée, on ne peut pas rollback et continuer.
			// On retourne l'erreur. Le scheduler fera un rollback de SA transaction.
			// Au prochain retry, la vérification d'idempotence (étape 2) trouvera le gagnant et réussira proprement.
			return nil, fmt.Errorf("concurrent credit unique violation, caller should rollback and retry: %w", err)
		}
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// ⚠️ IMPORTANT : Pas de tx.Commit() ici. Le caller (Execute ou Scheduler) est responsable du Commit.
	logger.Info().
		Str("shop_id", req.ShopID).
		Int64("gross_cents", grossCents).
		Int64("net_to_balance", netToBalance).
		Int64("swept_cents", swept).
		Str("transaction_id", txnID).
		Msg("Wallet credit logic processed successfully within transaction")

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

// isNotFound vérifie de manière robuste si une erreur correspond à un état "non trouvé".
// ✅ PR-3 FIX : Utilisation exclusive des erreurs sentinelles pour éviter les faux positifs
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	if errors.Is(err, repository.ErrWalletNotFound) || errors.Is(err, repository.ErrWalletTransactionNotFound) {
		return true
	}
	return false
}

// isUniqueViolation détecte une violation de contrainte unique (23505) de manière robuste.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "uq_wallet_txn_ref_completed") ||
		strings.Contains(msg, "23505")
}

// assertIdempotentCreditMatch vérifie qu'une requête de crédit correspond exactement
// à une transaction existante complétée, garantissant une idempotence stricte.
func assertIdempotentCreditMatch(existing *entity.WalletTransaction, req *CreditWalletRequest) error {
	if existing.ShopID != req.ShopID {
		return fmt.Errorf("idempotency conflict: shop_id mismatch (existing=%s, requested=%s)", existing.ShopID, req.ShopID)
	}
	if existing.TransactionType != req.TransactionType {
		return fmt.Errorf("idempotency conflict: transaction_type mismatch (existing=%s, requested=%s)", existing.TransactionType, req.TransactionType)
	}

	// 1. Vérification du montant brut (GrossAmountCents)
	if existing.GrossAmountCents != nil {
		if *existing.GrossAmountCents != req.AmountCents {
			return fmt.Errorf("idempotency conflict: gross amount mismatch (existing=%d, requested=%d)", *existing.GrossAmountCents, req.AmountCents)
		}
	} else {
		if existing.AmountCents != req.AmountCents {
			return fmt.Errorf("idempotency conflict: amount mismatch on legacy transaction (existing=%d, requested=%d)", existing.AmountCents, req.AmountCents)
		}
	}

	// 2. Vérification du flag HoldAfterCredit via le marqueur en description
	isHeldInDB := existing.Description != nil && strings.Contains(*existing.Description, "(held until proof/redeem)")
	if req.HoldAfterCredit && !isHeldInDB {
		return fmt.Errorf("idempotency conflict: request requires hold, but existing transaction is not held")
	}
	if !req.HoldAfterCredit && isHeldInDB {
		return fmt.Errorf("idempotency conflict: request is not held, but existing transaction is held")
	}

	return nil
}
