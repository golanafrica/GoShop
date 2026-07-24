package paymentusecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Erreurs typées pour le traitement des webhooks
var (
	ErrWebhookValidation       = errors.New("webhook validation failed")
	ErrWebhookProcessing       = errors.New("webhook processing failed")
	ErrWebhookAlreadyProcessed = errors.New("webhook already processed") // 🛡️ IDEMPOTENCE
)

// CreditUpdater définit l'interface minimale pour mettre à jour les entités de crédit
type CreditUpdater interface {
	MarkInstallmentPaid(ctx context.Context, installmentID string, paymentID string) error
	MarkContractDownPaymentPaid(ctx context.Context, contractID string, paymentID string) error
	CreditMerchantWallet(ctx context.Context, shopID string, amountCents int64, contractID string) error
}

// WalletUpdater définit l'interface pour créditer le wallet du marchand
type WalletUpdater interface {
	CreditOrderPayment(ctx context.Context, shopID string, amountCents int64, orderID string) error
}

// ProcessWebhookUsecase traite les webhooks reçus des providers
type ProcessWebhookUsecase struct {
	paymentRepo      repository.PaymentRepository
	registry         PaymentRegistry
	db               repository.DBExecutor
	shopRepo         repository.ShopRepository
	shopSettingsRepo ShopPaymentSettingsRepository
	tontineWebhookUC *ProcessTontineWebhookUsecase
	creditUpdater    CreditUpdater
	walletUpdater    WalletUpdater
}

// NewProcessWebhookUsecase crée une nouvelle instance
func NewProcessWebhookUsecase(
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
	db repository.DBExecutor,
	shopRepo repository.ShopRepository,
	shopSettingsRepo ShopPaymentSettingsRepository,
	tontineWebhookUC *ProcessTontineWebhookUsecase,
	creditUpdater CreditUpdater,
	walletUpdater WalletUpdater,
) *ProcessWebhookUsecase {
	return &ProcessWebhookUsecase{
		paymentRepo:      paymentRepo,
		registry:         registry,
		db:               db,
		shopRepo:         shopRepo,
		shopSettingsRepo: shopSettingsRepo,
		tontineWebhookUC: tontineWebhookUC,
		creditUpdater:    creditUpdater,
		walletUpdater:    walletUpdater,
	}
}

// Execute traite un webhook
func (uc *ProcessWebhookUsecase) Execute(ctx context.Context, providerCode entity.PaymentProvider, payload []byte, signature string) error {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("provider", string(providerCode)).
		Int("payload_size", len(payload)).
		Msg("Processing webhook")

	// 1. Récupérer le provider
	provider, err := uc.registry.Get(providerCode)
	if err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	// 2. Valider et parser le webhook
	event, err := provider.ValidateWebhook(ctx, payload, signature)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid webhook")
		_ = uc.recordWebhook(ctx, providerCode, nil, payload, signature, false, err.Error())
		return fmt.Errorf("%w: %v", ErrWebhookValidation, err)
	}

	// 3. 🛡️ Enregistrer le webhook et vérifier l'idempotence via RowsAffected
	err = uc.recordWebhook(ctx, providerCode, event, payload, signature, true, "")
	if err != nil {
		if errors.Is(err, ErrWebhookAlreadyProcessed) {
			logger.Info().Msg("🛡️ Webhook already processed (idempotent)")
			return ErrWebhookAlreadyProcessed
		}
		logger.Error().Err(err).Msg("Failed to record webhook audit")
	}

	// 🆕 3b. v2.9.0 : Détecter si c'est un webhook tontine
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

	// 4. Trouver le paiement associé (flux standard)
	var paymentEntity *entity.Payment
	var findErr error

	// 4a. Essayer d'abord par provider_ref
	if event.ProviderRef != "" {
		paymentEntity, findErr = uc.paymentRepo.FindByProviderRef(ctx, providerCode, event.ProviderRef)
	}

	// 4b. 🛡️ FALLBACK ROBUSTE AVEC LOGS DÉTAILLÉS
	if findErr != nil || paymentEntity == nil {
		logger.Warn().
			Err(findErr).
			Str("provider_ref", event.ProviderRef).
			Interface("metadata_debug", event.Metadata).
			Msg("Payment not found by provider_ref, attempting fallback via metadata")

		var orderIDStr string

		// Essai 1 : order_id direct dans les métadonnées
		if oid, ok := event.Metadata["order_id"].(string); ok && oid != "" {
			orderIDStr = oid
			logger.Info().Str("found_order_id", orderIDStr).Msg("DEBUG: Found order_id directly in metadata")
		} else if ref, ok := event.Metadata["reference"].(string); ok {
			logger.Info().Str("found_reference", ref).Msg("DEBUG: Found reference in metadata")
			if strings.HasPrefix(ref, "ORDER-") {
				orderIDStr = strings.TrimPrefix(ref, "ORDER-")
				logger.Info().Str("extracted_order_id", orderIDStr).Msg("DEBUG: Extracted order_id from reference")
			}
		}

		if orderIDStr != "" {
			logger.Info().Str("fallback_order_id", orderIDStr).Msg("Tentative de fallback par order_id")
			if orderUUID, parseErr := uuid.Parse(orderIDStr); parseErr == nil {
				// Note: FindByOrderIDUnscoped est utilisé ici car le contexte tenant n'est pas encore injecté.
				payments, searchErr := uc.paymentRepo.FindByOrderIDUnscoped(ctx, orderUUID)
				if searchErr != nil {
					logger.Error().Err(searchErr).Msg("DEBUG: FindByOrderIDUnscoped returned an error")
				} else if len(payments) > 0 {
					paymentEntity = payments[0]
					findErr = nil
					logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("✅ Payment retrouvé avec succès via fallback order_id (Unscoped)")
				} else {
					logger.Warn().Msg("DEBUG: FindByOrderIDUnscoped returned 0 payments (empty slice)")
				}
			} else {
				logger.Error().Err(parseErr).Str("order_id_str", orderIDStr).Msg("DEBUG: Failed to parse orderUUID")
			}
		} else {
			logger.Warn().Msg("DEBUG: orderIDStr is empty, skipping fallback")
		}
	}

	// 4c. Si toujours pas trouvé, on échoue proprement
	if findErr != nil || paymentEntity == nil {
		logger.Warn().
			Err(findErr).
			Str("provider_ref", event.ProviderRef).
			Msg("Payment definitively not found for webhook")
		return fmt.Errorf("%w: payment not found for provider_ref %s", ErrWebhookProcessing, event.ProviderRef)
	}

	// 4d. Injecter le shop du paiement dans le contexte
	shop, err := uc.shopRepo.FindByID(ctx, paymentEntity.ShopID)
	if err != nil {
		return fmt.Errorf("%w: shop not found for payment: %v", ErrWebhookProcessing, err)
	}
	ctx = tenant.WithTenant(ctx, shop)

	logger.Debug().Str("shop_id", shop.ID.String()).Str("shop_slug", shop.Slug).Msg("Tenant context injected for webhook processing")

	// 5. Vérifier si le paiement est déjà dans un état terminal
	if paymentEntity.IsTerminal() {
		// 🛡️ VÉRIFICATION D'IDEMPOTENCE : Le wallet a-t-il déjà été crédité ?
		walletAlreadyCredited := false
		if paymentEntity.Metadata != nil {
			if credited, ok := paymentEntity.Metadata["wallet_credited"].(bool); ok && credited {
				walletAlreadyCredited = true
			}
		}

		if walletAlreadyCredited {
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment already terminal and wallet already credited, ignoring webhook")
			return nil
		}

		logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment is terminal but wallet not credited yet, proceeding with wallet credit")
	}

	// 6. Transition de statut selon l'événement (flux standard)
	switch event.Status {
	case entity.PaymentStatusSuccess:
		if err := paymentEntity.MarkSuccess(event.ProviderRef); err != nil {
			return fmt.Errorf("%w: mark success: %v", ErrWebhookProcessing, err)
		}
		logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as SUCCESS")

		// 🆕 CRÉDIT DU WALLET MARCHAND (Modèle Marketplace Centralisé)
		isCredit := paymentEntity.ReferenceType != nil && (*paymentEntity.ReferenceType == "credit_installment" || *paymentEntity.ReferenceType == "credit_down_payment")

		if !isCredit && paymentEntity.OrderID != uuid.Nil {
			commissionRate := 250 // Défaut 2.5% (250 basis points)
			if uc.shopSettingsRepo != nil {
				settings, err := uc.shopSettingsRepo.GetPaymentSettings(ctx, paymentEntity.ShopID)
				if err == nil && settings != nil {
					if settings.CashCommissionRate > 0 {
						commissionRate = settings.CashCommissionRate
					}
				}
			}

			commissionCents := (paymentEntity.AmountCents * int64(commissionRate)) / 10000
			netAmountCents := paymentEntity.AmountCents - commissionCents

			if netAmountCents > 0 && uc.walletUpdater != nil {
				err := uc.walletUpdater.CreditOrderPayment(ctx, paymentEntity.ShopID.String(), netAmountCents, paymentEntity.OrderID.String())
				if err != nil {
					logger.Error().Err(err).Msg("Failed to credit merchant wallet for order")
				} else {
					// 🛡️ MARQUER COMME CRÉDITÉ POUR L'IDEMPOTENCE
					if paymentEntity.Metadata == nil {
						paymentEntity.Metadata = make(map[string]interface{})
					}
					paymentEntity.Metadata["wallet_credited"] = true
					if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
						logger.Error().Err(updateErr).Msg("Failed to update payment metadata with wallet_credited flag")
					}

					logger.Info().
						Int64("gross_amount", paymentEntity.AmountCents).
						Int64("commission_cents", commissionCents).
						Int64("net_amount", netAmountCents).
						Msg("✅ Merchant wallet credited successfully")
				}
			}
		}

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

	// 7. Sauvegarder les changements
	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return fmt.Errorf("%w: update payment: %v", ErrWebhookProcessing, err)
	}

	logger.Info().Str("payment_id", paymentEntity.ID.String()).Str("status", string(paymentEntity.Status)).Msg("Webhook processed successfully")
	return nil
}

// recordWebhook enregistre le webhook dans la table d'audit
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
