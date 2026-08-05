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
}

// NewSyncTontinePaymentUsecase crée une nouvelle instance
func NewSyncTontinePaymentUsecase(
	paymentRepo repository.TontinePaymentRepository,
	groupRepo repository.TontineGroupRepository,
	paymentRegistry paymentusecase.PaymentRegistry,
	tontineWebhookUC *paymentusecase.ProcessTontineWebhookUsecase,
) *SyncTontinePaymentUsecase {
	return &SyncTontinePaymentUsecase{
		paymentRepo:      paymentRepo,
		groupRepo:        groupRepo,
		paymentRegistry:  paymentRegistry,
		tontineWebhookUC: tontineWebhookUC,
	}
}

// SyncPaymentRequest représente la requête de synchronisation
type SyncPaymentRequest struct {
	PaymentID      string `json:"payment_id"`
	CustomerID     string `json:"customer_id"` // 🛡️ FIX C1 : Injecté par le handler depuis le JWT
	ProviderIntent string `json:"provider_intent,omitempty"`
}

// SyncPaymentResponse représente la réponse de la synchronisation
type SyncPaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	CurrentStatus string `json:"current_status"`
	Synced        bool   `json:"synced"`
	Message       string `json:"message"`
}

// Execute synchronise le statut d'un paiement tontine
func (uc *SyncTontinePaymentUsecase) Execute(ctx context.Context, req *SyncPaymentRequest) (*SyncPaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le paiement (Unscoped car on peut être dans un cron ou sync manuel)
	payment, err := uc.paymentRepo.FindByIDUnscoped(ctx, req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 🛡️ FIX C1 : Vérifier que le paiement appartient bien à ce client
	if payment.CustomerID != req.CustomerID {
		return nil, fmt.Errorf("payment does not belong to this customer")
	}

	// Si déjà DONE, on retourne immédiatement (idempotence)
	if payment.IsDone() {
		return &SyncPaymentResponse{
			PaymentID:     payment.ID,
			CurrentStatus: payment.Status,
			Synced:        false,
			Message:       "Payment already completed",
		}, nil
	}

	// 🆕 FIX C2 : Utiliser exclusivement le ProviderIntentID.
	// Pas de fallback sur la référence TONTINE car l'API CheckStatus de YengaPay exige un ID de transaction/intention valide.
	intentID := req.ProviderIntent
	if intentID == "" {
		intentID = payment.ProviderIntentID
	}

	if intentID == "" {
		return nil, fmt.Errorf("no provider intent ID available for sync. The payment may have been initiated via a flow that does not support programmatic status checking, or the intent ID was not saved")
	}

	// 3. Récupérer le provider YengaPay
	provider, err := uc.paymentRegistry.GetAvailable(ctx, entity.ProviderYengaPay)
	if err != nil {
		return nil, fmt.Errorf("payment provider not available: %w", err)
	}

	// 4. Interroger le statut auprès de YengaPay
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

	// 5. Si le statut est SUCCESS et que notre paiement n'est pas encore DONE, on déclenche la logique de webhook
	if statusResp.Status == entity.PaymentStatusSuccess && !payment.IsDone() {
		logger.Info().Str("payment_id", payment.ID).Msg("Payment is SUCCESS on provider, triggering webhook logic")

		// On a besoin de la référence TONTINE complète pour que le webhook retrouve le paiement
		ref := *payment.YengaPayReference

		// CheckStatus ne retourne pas toujours un ExternalID fiable, on utilise l'intentID en fallback
		externalID := statusResp.ProviderRef
		if externalID == "" {
			externalID = intentID
		}

		// On appelle le usecase de webhook pour réutiliser toute la logique (mark done, check cycle completion, credit wallet, etc.)
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

		// Re-fetch payment to get updated status
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

	// 6. Si le statut est FAILED
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

	// 7. Sinon, le statut est toujours en cours ou inchangé
	return &SyncPaymentResponse{
		PaymentID:     payment.ID,
		CurrentStatus: payment.Status,
		Synced:        false,
		Message:       fmt.Sprintf("Payment status is still %s on provider", statusResp.Status),
	}, nil
}
