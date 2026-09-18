package withdrawalusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// ProcessPayoutWebhookUsecase traite les webhooks de retrait (Cash-Out) de YengaPay
type ProcessPayoutWebhookUsecase struct {
	withdrawalRepo repository.WithdrawalRepository
	walletRepo     repository.MerchantWalletRepository
	txManager      repository.TxManager
}

// NewProcessPayoutWebhookUsecase crée une nouvelle instance
func NewProcessPayoutWebhookUsecase(
	withdrawalRepo repository.WithdrawalRepository,
	walletRepo repository.MerchantWalletRepository,
	txManager repository.TxManager,
) *ProcessPayoutWebhookUsecase {
	return &ProcessPayoutWebhookUsecase{
		withdrawalRepo: withdrawalRepo,
		walletRepo:     walletRepo,
		txManager:      txManager,
	}
}

// Execute traite l'événement de webhook (payout.success ou payout.failed)
func (uc *ProcessPayoutWebhookUsecase) Execute(ctx context.Context, providerRef string, eventType string, yengaPayload map[string]interface{}) error {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("provider_ref", providerRef).
		Str("event_type", eventType).
		Msg("Processing YengaPay payout webhook")

	// 1. Trouver le retrait via la référence YengaPay
	withdrawal, err := uc.withdrawalRepo.FindByProviderRef(ctx, providerRef)
	if err != nil {
		logger.Error().Err(err).Str("provider_ref", providerRef).Msg("Withdrawal not found for webhook")
		return fmt.Errorf("withdrawal not found: %w", err)
	}

	// 2. Si déjà traité (idempotence), on ignore
	if withdrawal.Status == entity.WithdrawalStatusSuccess || withdrawal.Status == entity.WithdrawalStatusFailed {
		logger.Warn().Str("withdrawal_id", withdrawal.ID.String()).Msg("Withdrawal already processed, ignoring webhook")
		return nil
	}

	// 3. Démarrer une transaction pour mettre à jour le statut (et rembourser si échec)
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	withdrawalRepoTx := uc.withdrawalRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)

	switch eventType {
	case "payout.success":
		// SUCCES : l'argent est parti du compte GoShop vers le marchand
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
		// ECHEC : YengaPay a recrédité le compte principal GoShop.
		// On recrédite le wallet virtuel du marchand pour rester cohérent.
		reason := extractErrorMessage(yengaPayload)
		if err := withdrawal.MarkFailed(reason); err != nil {
			return fmt.Errorf("failed to mark withdrawal as failed: %w", err)
		}

		if err := withdrawalRepoTx.Update(ctx, withdrawal); err != nil {
			return fmt.Errorf("failed to update withdrawal status: %w", err)
		}

		refundAmount := withdrawal.AmountCents

		wallet, err := walletRepoTx.FindByShopIDForUpdate(ctx, withdrawal.ShopID.String())
		if err != nil {
			return fmt.Errorf("failed to find wallet for refund: %w", err)
		}

		wallet.BalanceCents += refundAmount
		wallet.UpdatedAt = time.Now().UTC()

		if err := walletRepoTx.Update(ctx, wallet); err != nil {
			return fmt.Errorf("failed to refund wallet: %w", err)
		}

		logger.Warn().
			Str("withdrawal_id", withdrawal.ID.String()).
			Int64("refunded_amount", refundAmount).
			Str("reason", reason).
			Msg("Withdrawal FAILED, merchant wallet refunded")

	default:
		logger.Warn().Str("event_type", eventType).Msg("Unknown payout event type, ignoring")
		return nil
	}

	// 4. Commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// extractOperatorTransID lit l'ID opérateur depuis le metadata / payload YengaPay
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
