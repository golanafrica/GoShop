package tontineusecase

import (
	"context"
	"fmt"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// SyncTontinePaymentUsecase permet de synchroniser manuellement ou via cron le statut d'un paiement tontine
type SyncTontinePaymentUsecase struct {
	paymentRepo      repository.TontinePaymentRepository
	groupRepo        repository.TontineGroupRepository
	paymentRegistry  paymentusecase.PaymentRegistry
	tontineWebhookUC *paymentusecase.ProcessTontineWebhookUsecase
	txManager        repository.TxManager // 🛡️ v4.11.0 : Pour FOR UPDATE
}

// NewSyncTontinePaymentUsecase crée une nouvelle instance
func NewSyncTontinePaymentUsecase(
	paymentRepo repository.TontinePaymentRepository,
	groupRepo repository.TontineGroupRepository,
	paymentRegistry paymentusecase.PaymentRegistry,
	tontineWebhookUC *paymentusecase.ProcessTontineWebhookUsecase,
	txManager repository.TxManager, // 🛡️ v4.11.0 : Ajouté
) *SyncTontinePaymentUsecase {
	return &SyncTontinePaymentUsecase{
		paymentRepo:      paymentRepo,
		groupRepo:        groupRepo,
		paymentRegistry:  paymentRegistry,
		tontineWebhookUC: tontineWebhookUC,
		txManager:        txManager,
	}
}

type SyncPaymentRequest struct {
	PaymentID      string `json:"payment_id"`
	CustomerID     string `json:"customer_id"`
	ProviderIntent string `json:"provider_intent,omitempty"`
}

type SyncPaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	CurrentStatus string `json:"current_status"`
	Synced        bool   `json:"synced"`
	Message       string `json:"message"`
}

// Execute synchronise le statut d'un paiement tontine avec protection anti-race condition
func (uc *SyncTontinePaymentUsecase) Execute(ctx context.Context, req *SyncPaymentRequest) (*SyncPaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// ============================================================
	// 🛡️ v4.11.0 : ANTI-RACE CONDITION
	// Démarrer une transaction et verrouiller le paiement avec FOR UPDATE
	// pour éviter les doubles syncs simultanés (cron + manuel)
	// ============================================================
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	paymentRepoTx := uc.paymentRepo.WithTX(tx)

	// Verrou pessimiste : FOR UPDATE bloque les autres syncs simultanés
	payment, err := paymentRepoTx.FindByIDUnscopedForUpdate(ctx, req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.CustomerID != req.CustomerID {
		return nil, fmt.Errorf("payment does not belong to this customer")
	}

	// Double-check après acquisition du verrou (idempotence)
	if payment.IsDone() {
		return &SyncPaymentResponse{
			PaymentID:     payment.ID,
			CurrentStatus: payment.Status,
			Synced:        false,
			Message:       "Payment already completed",
		}, nil
	}

	// Commit le verrou AVANT d'appeler le provider externe (évite deadlock)
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	intentID := req.ProviderIntent
	if intentID == "" {
		intentID = payment.ProviderIntentID
	}

	if intentID == "" {
		return nil, fmt.Errorf("no provider intent ID available for sync. The payment may have been initiated via a flow that does not support programmatic status checking, or the intent ID was not saved")
	}

	provider, err := uc.paymentRegistry.GetAvailable(ctx, entity.ProviderYengaPay)
	if err != nil {
		return nil, fmt.Errorf("payment provider not available: %w", err)
	}

	logger.Info().Str("intent_id", intentID).Msg("Checking payment status with YengaPay")
	statusResp, err := provider.CheckStatus(ctx, intentID)
	if err != nil {
		logger.Warn().Err(err).Str("intent_id", intentID).Msg("Failed to check status with provider")
		return &SyncPaymentResponse{
			PaymentID:     payment.ID,
			CurrentStatus: payment.Status,
			Synced:        false,
			Message:       "Failed to check status with provider: " + err.Error(),
		}, nil
	}

	// Si le statut est SUCCESS, déclencher la logique de webhook
	if statusResp.Status == entity.PaymentStatusSuccess && !payment.IsDone() {
		logger.Info().Str("payment_id", payment.ID).Msg("Payment is SUCCESS on provider, triggering webhook logic")

		ref := *payment.YengaPayReference
		externalID := statusResp.ProviderRef
		if externalID == "" {
			externalID = intentID
		}

		// ProcessTontineWebhookUsecase gère ses propres verrous (idempotent)
		err = uc.tontineWebhookUC.Execute(ctx, ref, externalID, entity.PaymentStatusSuccess)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to process tontine webhook logic during sync")
			return &SyncPaymentResponse{
				PaymentID:     payment.ID,
				CurrentStatus: payment.Status,
				Synced:        false,
				Message:       "Failed to process completion logic: " + err.Error(),
			}, nil
		}

		updatedPayment, err := uc.paymentRepo.FindByIDUnscoped(ctx, req.PaymentID)
		if err == nil {
			payment = updatedPayment
		}

		return &SyncPaymentResponse{
			PaymentID:     payment.ID,
			CurrentStatus: payment.Status,
			Synced:        true,
			Message:       "Payment successfully synced and completed",
		}, nil
	}

	if statusResp.Status == entity.PaymentStatusFailed && payment.Status != string(entity.TontinePaymentFailed) {
		logger.Info().Str("payment_id", payment.ID).Msg("Payment is FAILED on provider, updating status")
		_ = uc.paymentRepo.UpdateStatus(ctx, payment.ID, string(entity.TontinePaymentFailed))

		return &SyncPaymentResponse{
			PaymentID:     payment.ID,
			CurrentStatus: string(entity.TontinePaymentFailed),
			Synced:        true,
			Message:       "Payment marked as failed based on provider status",
		}, nil
	}

	return &SyncPaymentResponse{
		PaymentID:     payment.ID,
		CurrentStatus: payment.Status,
		Synced:        false,
		Message:       fmt.Sprintf("Payment status is still %s on provider", statusResp.Status),
	}, nil
}
