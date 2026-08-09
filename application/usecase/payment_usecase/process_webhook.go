package paymentusecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var (
	ErrWebhookValidation       = errors.New("webhook validation failed")
	ErrWebhookProcessing       = errors.New("webhook processing failed")
	ErrWebhookAlreadyProcessed = errors.New("webhook already processed")
)

type CreditUpdater interface {
	MarkInstallmentPaid(ctx context.Context, installmentID string, paymentID string) error
	MarkContractDownPaymentPaid(ctx context.Context, contractID string, paymentID string) error
	CreditMerchantWallet(ctx context.Context, shopID string, amountCents int64, contractID string) error
}

type ProcessWebhookUsecase struct {
	paymentRepo      repository.PaymentRepository
	registry         PaymentRegistry
	db               repository.DBExecutor
	shopRepo         repository.ShopRepository
	shopSettingsRepo ShopPaymentSettingsRepository
	tontineWebhookUC *ProcessTontineWebhookUsecase
	creditUpdater    CreditUpdater
	escrowRepo       repository.EscrowAccountRepository
	orderRepo        repository.OrderRepository // Phase 1 : confirmer order après SUCCESS
}

func NewProcessWebhookUsecase(
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
	db repository.DBExecutor,
	shopRepo repository.ShopRepository,
	shopSettingsRepo ShopPaymentSettingsRepository,
	tontineWebhookUC *ProcessTontineWebhookUsecase,
	creditUpdater CreditUpdater,
	escrowRepo repository.EscrowAccountRepository,
	orderRepo repository.OrderRepository, // Phase 1
) *ProcessWebhookUsecase {
	return &ProcessWebhookUsecase{
		paymentRepo:      paymentRepo,
		registry:         registry,
		db:               db,
		shopRepo:         shopRepo,
		shopSettingsRepo: shopSettingsRepo,
		tontineWebhookUC: tontineWebhookUC,
		creditUpdater:    creditUpdater,
		escrowRepo:       escrowRepo,
		orderRepo:        orderRepo,
	}
}

func (uc *ProcessWebhookUsecase) Execute(ctx context.Context, providerCode entity.PaymentProvider, payload []byte, signature string) error {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("provider", string(providerCode)).
		Int("payload_size", len(payload)).
		Msg("Processing webhook")

	provider, err := uc.registry.Get(providerCode)
	if err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	event, err := provider.ValidateWebhook(ctx, payload, signature)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid webhook")
		_ = uc.recordWebhook(ctx, providerCode, nil, payload, signature, false, err.Error())
		return fmt.Errorf("%w: %v", ErrWebhookValidation, err)
	}

	err = uc.recordWebhook(ctx, providerCode, event, payload, signature, true, "")
	if err != nil {
		if errors.Is(err, ErrWebhookAlreadyProcessed) {
			logger.Info().Msg("🛡️ Webhook already processed (idempotent)")
			return ErrWebhookAlreadyProcessed
		}
		logger.Error().Err(err).Msg("Failed to record webhook audit")
	}

	// ---------- Tontine ----------
	if reference, ok := event.Metadata["reference"].(string); ok && IsTontineReference(reference) {
		logger.Info().
			Str("reference", reference).
			Str("transaction_id", event.ExternalID).
			Str("status", string(event.Status)).
			Msg("🎯 Tontine webhook detected, delegating to ProcessTontineWebhookUsecase")

		if uc.tontineWebhookUC == nil {
			return fmt.Errorf("%w: tontine webhook handler not configured", ErrWebhookProcessing)
		}

		err := uc.tontineWebhookUC.Execute(ctx, reference, event.ExternalID, event.Status)
		if err != nil {
			logger.Error().Err(err).Msg("Tontine webhook processing failed")
			return fmt.Errorf("%w: tontine webhook: %v", ErrWebhookProcessing, err)
		}

		logger.Info().Str("reference", reference).Msg("✅ Tontine webhook processed successfully")
		return nil
	}

	// ---------- Order payment ----------
	var paymentEntity *entity.Payment
	var findErr error

	if event.ProviderRef != "" {
		paymentEntity, findErr = uc.paymentRepo.FindByProviderRef(ctx, providerCode, event.ProviderRef)
	}

	if findErr != nil || paymentEntity == nil {
		logger.Warn().
			Err(findErr).
			Str("provider_ref", event.ProviderRef).
			Interface("metadata_debug", event.Metadata).
			Msg("Payment not found by provider_ref, attempting fallback via metadata")

		var orderIDStr string

		if oid, ok := event.Metadata["order_id"].(string); ok && oid != "" {
			orderIDStr = oid
		} else if ref, ok := event.Metadata["reference"].(string); ok {
			if strings.HasPrefix(ref, "ORDER-") {
				orderIDStr = strings.TrimPrefix(ref, "ORDER-")
			}
		}

		if orderIDStr != "" {
			if orderUUID, parseErr := uuid.Parse(orderIDStr); parseErr == nil {
				payments, searchErr := uc.paymentRepo.FindByOrderIDUnscoped(ctx, orderUUID)
				if searchErr == nil && len(payments) > 0 {
					paymentEntity = payments[0]
					findErr = nil
					logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("✅ Payment retrouvé via fallback order_id")
				}
			}
		}
	}

	if findErr != nil || paymentEntity == nil {
		logger.Warn().
			Err(findErr).
			Str("provider_ref", event.ProviderRef).
			Msg("Payment definitively not found for webhook")
		return fmt.Errorf("%w: payment not found for provider_ref %s", ErrWebhookProcessing, event.ProviderRef)
	}

	shop, err := uc.shopRepo.FindByID(ctx, paymentEntity.ShopID)
	if err != nil {
		return fmt.Errorf("%w: shop not found for payment: %v", ErrWebhookProcessing, err)
	}
	ctx = tenant.WithTenant(ctx, shop)

	logger.Debug().Str("shop_id", shop.ID.String()).Str("shop_slug", shop.Slug).Msg("Tenant context injected for webhook processing")

	needsEscrowCreation := false

	if paymentEntity.IsTerminal() {
		escrowAlreadyCreated := false
		if paymentEntity.Metadata != nil {
			if created, ok := paymentEntity.Metadata["escrow_created"].(bool); ok && created {
				escrowAlreadyCreated = true
			}
		}

		if escrowAlreadyCreated {
			// Phase 1 : même si déjà terminal, s'assurer que l'order est confirmed
			uc.ensureOrderConfirmed(ctx, paymentEntity, logger)
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment already terminal and escrow already created, ignoring webhook")
			return nil
		}

		logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment is terminal but escrow not created yet, proceeding with escrow creation")
		needsEscrowCreation = true
	} else {
		switch event.Status {
		case entity.PaymentStatusSuccess:
			if err := paymentEntity.MarkSuccess(event.ProviderRef); err != nil {
				return fmt.Errorf("%w: mark success: %v", ErrWebhookProcessing, err)
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as SUCCESS")
			needsEscrowCreation = true

		case entity.PaymentStatusFailed:
			reason := "unknown"
			if r, ok := event.Metadata["failure_reason"]; ok {
				if s, ok := r.(string); ok {
					reason = s
				}
			}
			if err := paymentEntity.MarkFailed(reason); err != nil {
				return fmt.Errorf("%w: mark failed: %v", ErrWebhookProcessing, err)
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Str("reason", reason).Msg("Payment marked as FAILED")

		case entity.PaymentStatusCancelled:
			if err := paymentEntity.MarkCancelled(); err != nil {
				return fmt.Errorf("%w: mark cancelled: %v", ErrWebhookProcessing, err)
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as CANCELLED")

		case entity.PaymentStatusRefunded:
			if err := paymentEntity.MarkRefunded(); err != nil {
				return fmt.Errorf("%w: mark refunded: %v", ErrWebhookProcessing, err)
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as REFUNDED")

		default:
			logger.Warn().Str("status", string(event.Status)).Msg("Unknown webhook status, ignoring")
			return nil
		}
	}

	if needsEscrowCreation && (event.Status == entity.PaymentStatusSuccess || paymentEntity.Status == entity.PaymentStatusSuccess) {
		isCredit := paymentEntity.ReferenceType != nil && (*paymentEntity.ReferenceType == "credit_installment" || *paymentEntity.ReferenceType == "credit_down_payment")

		if !isCredit && paymentEntity.OrderID != uuid.Nil {
			commissionRate := 250
			if uc.shopSettingsRepo != nil {
				settings, err := uc.shopSettingsRepo.GetPaymentSettings(ctx, paymentEntity.ShopID)
				if err == nil && settings != nil {
					if settings.CashCommissionRate > 0 {
						commissionRate = settings.CashCommissionRate
					}
				}
			}

			commissionCents := (paymentEntity.AmountCents * int64(commissionRate)) / 10000

			providerFeesCents := int64(0)
			if fees, ok := event.Metadata["payment_fees"].(float64); ok {
				providerFeesCents = int64(fees * 100)
			}
			paymentEntity.ProviderFeesCents = providerFeesCents

			netAmountCents := paymentEntity.AmountCents - providerFeesCents - commissionCents

			if netAmountCents > 0 && uc.escrowRepo != nil {
				orderIDStr := paymentEntity.OrderID.String()
				now := time.Now().UTC()

				escrow := &entity.EscrowAccount{
					OrderID:             &orderIDStr,
					SourceType:          entity.EscrowSourceOrder,
					TotalAmountCents:    paymentEntity.AmountCents - providerFeesCents,
					ReleasedAmountCents: 0,
					CommissionCents:     commissionCents,
					Status:              entity.EscrowAccountFundsHeld,
					FundsHeldAt:         now,
					CreatedAt:           now,
					UpdatedAt:           now,
				}

				if err := uc.escrowRepo.Create(ctx, escrow); err != nil {
					logger.Error().Err(err).Msg("Failed to create escrow account")
					return fmt.Errorf("failed to create escrow: %w", err)
				}

				if paymentEntity.Metadata == nil {
					paymentEntity.Metadata = make(map[string]interface{})
				}
				paymentEntity.Metadata["escrow_created"] = true
				if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
					logger.Error().Err(updateErr).Msg("Failed to update payment metadata with escrow_created flag")
				}

				logger.Info().
					Int64("gross_amount", paymentEntity.AmountCents).
					Int64("provider_fees_cents", providerFeesCents).
					Int64("commission_cents", commissionCents).
					Int64("escrow_amount", netAmountCents).
					Str("escrow_id", escrow.ID).
					Msg("✅ Funds successfully locked in Escrow")
			}

			// ============================================================
			// Phase 1 : confirmer l'order (pending → confirmed)
			// Débloque submit_shipping_proof (exige confirmed | out_for_delivery)
			// ============================================================
			uc.ensureOrderConfirmed(ctx, paymentEntity, logger)
		}
	}

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return fmt.Errorf("%w: update payment: %v", ErrWebhookProcessing, err)
	}

	logger.Info().Str("payment_id", paymentEntity.ID.String()).Str("status", string(paymentEntity.Status)).Msg("Webhook processed successfully")
	return nil
}

// ensureOrderConfirmed passe l'order de pending → confirmed si besoin.
// Idempotent : no-op si déjà confirmed ou orderRepo nil.
func (uc *ProcessWebhookUsecase) ensureOrderConfirmed(ctx context.Context, paymentEntity *entity.Payment, logger *zerolog.Logger) {
	if uc.orderRepo == nil || paymentEntity.OrderID == uuid.Nil {
		return
	}

	orderIDStr := paymentEntity.OrderID.String()
	order, err := uc.orderRepo.FindByID(ctx, orderIDStr)
	if err != nil || order == nil {
		logger.Warn().Err(err).Str("order_id", orderIDStr).Msg("Failed to load order for confirmation after webhook")
		return
	}

	if order.Status != string(entity.OrderStatusPending) {
		return
	}

	if err := order.MarkAccepted(); err != nil {
		logger.Warn().Err(err).Str("order_id", orderIDStr).Msg("Failed to MarkAccepted after webhook SUCCESS")
		return
	}

	if err := uc.orderRepo.UpdateOrder(ctx, order); err != nil {
		logger.Warn().Err(err).Str("order_id", orderIDStr).Msg("Failed to UpdateOrder after webhook SUCCESS")
		return
	}

	logger.Info().
		Str("order_id", orderIDStr).
		Msg("✅ Order confirmed after payment webhook SUCCESS")
}

func (uc *ProcessWebhookUsecase) recordWebhook(
	ctx context.Context,
	provider entity.PaymentProvider,
	event *payment.WebhookEvent,
	payload []byte,
	signature string,
	signatureValid bool,
	processingError string,
) error {
	if uc.db == nil {
		return nil
	}

	var eventType, externalID string
	if event != nil {
		eventType = event.EventType
		externalID = event.ExternalID
	}

	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"raw": string(payload),
	})

	query := `
		INSERT INTO payment_webhooks (
			provider, event_type, external_id, payload, signature,
			signature_validated, processing_error, processed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, external_id) DO NOTHING
	`

	processed := signatureValid && processingError == ""

	res, err := uc.db.ExecContext(ctx, query,
		provider,
		eventType,
		externalID,
		payloadJSON,
		signature,
		signatureValid,
		processingError,
		processed,
	)

	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to record webhook")
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 && externalID != "" {
		return ErrWebhookAlreadyProcessed
	}

	return nil
}
