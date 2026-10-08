package withdrawalusecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ProcessPayoutWebhookUsecase traite les webhooks de retrait (Cash-Out) de YengaPay.
//
// payout.success  → MarkSuccess (debit wallet déjà fait à la création).
// payout.failed   → MarkFailed + reverse balance + ledger withdrawal_reversal.
//
// Coexistence P0-A : si CashOut a déjà échoué en synchrone (status=failed + reverse),
// ce webhook est no-op (idempotence statut).
type ProcessPayoutWebhookUsecase struct {
	withdrawalRepo repository.WithdrawalRepository
	walletRepo     repository.MerchantWalletRepository
	txnRepo        repository.WalletTransactionRepository // audit ledger
	txManager      repository.TxManager
}

func NewProcessPayoutWebhookUsecase(
	withdrawalRepo repository.WithdrawalRepository,
	walletRepo repository.MerchantWalletRepository,
	txnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
) *ProcessPayoutWebhookUsecase {
	return &ProcessPayoutWebhookUsecase{
		withdrawalRepo: withdrawalRepo,
		walletRepo:     walletRepo,
		txnRepo:        txnRepo,
		txManager:      txManager,
	}
}

// Execute traite payout.success | payout.failed.
func (uc *ProcessPayoutWebhookUsecase) Execute(
	ctx context.Context,
	providerRef string,
	eventType string,
	yengaPayload map[string]interface{},
) error {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("provider_ref", providerRef).
		Str("event_type", eventType).
		Msg("Processing YengaPay payout webhook")

	if providerRef == "" {
		return fmt.Errorf("provider_ref is required")
	}

	// 1. Lookup hors TX (fast path)
	withdrawal, err := uc.withdrawalRepo.FindByProviderRef(ctx, providerRef)
	if err != nil {
		logger.Error().Err(err).Str("provider_ref", providerRef).Msg("Withdrawal not found for webhook")
		return fmt.Errorf("withdrawal not found: %w", err)
	}

	// 2. Idempotence statut (P0-A peut déjà avoir failed + reverse)
	if withdrawal.Status == entity.WithdrawalStatusSuccess ||
		withdrawal.Status == entity.WithdrawalStatusFailed ||
		withdrawal.Status == entity.WithdrawalStatusCancelled {
		logger.Info().
			Str("withdrawal_id", withdrawal.ID.String()).
			Str("status", string(withdrawal.Status)).
			Msg("Withdrawal already terminal, ignoring payout webhook")
		return nil
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	withdrawalRepoTx := uc.withdrawalRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)

	// 3. Relecture sous TX (réduit les courses double-webhook)
	withdrawal, err = withdrawalRepoTx.FindByProviderRef(ctx, providerRef)
	if err != nil {
		return fmt.Errorf("withdrawal not found in tx: %w", err)
	}
	if withdrawal.Status == entity.WithdrawalStatusSuccess ||
		withdrawal.Status == entity.WithdrawalStatusFailed ||
		withdrawal.Status == entity.WithdrawalStatusCancelled {
		logger.Info().
			Str("withdrawal_id", withdrawal.ID.String()).
			Msg("Withdrawal became terminal concurrently, skipping")
		return nil
	}

	switch eventType {
	case "payout.success":
		operatorTxID := extractOperatorTransID(yengaPayload)
		if err := withdrawal.MarkSuccess(
			withdrawal.FeesCents,
			withdrawal.NetAmountCents,
			operatorTxID,
		); err != nil {
			return fmt.Errorf("failed to mark withdrawal as success: %w", err)
		}
		if err := withdrawalRepoTx.Update(ctx, withdrawal); err != nil {
			return fmt.Errorf("failed to update withdrawal status: %w", err)
		}
		logger.Info().
			Str("withdrawal_id", withdrawal.ID.String()).
			Str("operator_transaction_id", operatorTxID).
			Msg("Withdrawal marked as SUCCESS")

	case "payout.failed":
		if err := uc.handlePayoutFailed(ctx, logger, withdrawal, yengaPayload, withdrawalRepoTx, walletRepoTx, tx); err != nil {
			return err
		}

	default:
		logger.Warn().Str("event_type", eventType).Msg("Unknown payout event type, ignoring")
		return nil
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (uc *ProcessPayoutWebhookUsecase) handlePayoutFailed(
	ctx context.Context,
	logger *zerolog.Logger,
	withdrawal *entity.Withdrawal,
	yengaPayload map[string]interface{},
	withdrawalRepoTx repository.WithdrawalRepository,
	walletRepoTx repository.MerchantWalletRepository,
	tx repository.Tx,
) error {
	reason := extractErrorMessage(yengaPayload)
	withdrawalID := withdrawal.ID.String()
	shopID := withdrawal.ShopID.String()
	refundAmount := withdrawal.AmountCents

	if refundAmount <= 0 {
		return fmt.Errorf("invalid withdrawal amount_cents for refund: %d", refundAmount)
	}

	// Idempotence ledger : reverse déjà posé (ex: P0-A synchrone avec même ref)
	if uc.txnRepo != nil {
		refType := "withdrawal_reversal"
		existing, findErr := uc.txnRepo.WithTX(tx).FindByReferenceID(ctx, refType, withdrawalID)
		if findErr == nil && existing != nil && existing.Status == entity.WalletTxCompleted {
			logger.Info().
				Str("withdrawal_id", withdrawalID).
				Str("existing_txn", existing.ID).
				Msg("Ledger withdrawal_reversal already present — skip balance refund")

			if err := withdrawal.MarkFailed(reason); err != nil {
				// déjà terminal concurrent
				logger.Warn().Err(err).Msg("MarkFailed after existing reversal")
			} else if err := withdrawalRepoTx.Update(ctx, withdrawal); err != nil {
				return fmt.Errorf("failed to update withdrawal status: %w", err)
			}
			return nil
		}
	}

	if err := withdrawal.MarkFailed(reason); err != nil {
		return fmt.Errorf("failed to mark withdrawal as failed: %w", err)
	}
	if err := withdrawalRepoTx.Update(ctx, withdrawal); err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}

	wallet, err := walletRepoTx.FindByShopIDForUpdate(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to find wallet for refund: %w", err)
	}

	// Reverse brut (pas CreditWithDebtSweep) : on restaure exactement le débit payout.
	// Autorisé même si frozen : le marchand ne doit pas perdre des fonds sur un payout failed.
	wallet.BalanceCents += refundAmount
	wallet.UpdatedAt = time.Now().UTC()

	if err := walletRepoTx.Update(ctx, wallet); err != nil {
		return fmt.Errorf("failed to refund wallet: %w", err)
	}

	if uc.txnRepo != nil {
		refType := "withdrawal_reversal"
		refID := withdrawalID
		desc := fmt.Sprintf("Payout webhook reverse for withdrawal %s (%s)", withdrawalID, reason)
		txn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            shopID,
			TransactionType:   entity.WalletTxDeposit,
			AmountCents:       refundAmount,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &refType,
			ReferenceID:       &refID,
			Description:       &desc,
			Status:            entity.WalletTxCompleted,
		}
		if err := uc.txnRepo.WithTX(tx).Create(ctx, txn); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "duplicate key") ||
				strings.Contains(msg, "unique constraint") ||
				strings.Contains(msg, "23505") {
				logger.Info().Msg("Ledger withdrawal_reversal unique hit — treat as idempotent success")
			} else {
				return fmt.Errorf("failed to create withdrawal_reversal ledger: %w", err)
			}
		}
	}

	logger.Warn().
		Str("withdrawal_id", withdrawalID).
		Str("shop_id", shopID).
		Int64("refunded_amount", refundAmount).
		Int64("balance_after", wallet.BalanceCents).
		Str("reason", reason).
		Msg("Withdrawal FAILED, merchant wallet refunded + ledger withdrawal_reversal")

	return nil
}

func extractOperatorTransID(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	keys := []string{
		"operatorTransId",
		"operator_transaction_id",
		"operator_trans_id",
		"operatorTransactionId",
	}
	for _, k := range keys {
		if id, ok := payload[k].(string); ok && id != "" {
			return id
		}
	}
	return ""
}

func extractErrorMessage(payload map[string]interface{}) string {
	if payload == nil {
		return "Unknown error from provider"
	}
	if msg, ok := payload["errorMessage"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}
	return "Unknown error from provider"
}
