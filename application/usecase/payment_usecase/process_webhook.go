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
	paymentRepo        repository.PaymentRepository
	registry           PaymentRegistry
	txManager          repository.TxManager // 🆕 AJOUT : Pour l'atomicité des opérations financières
	db                 repository.DBExecutor
	shopRepo           repository.ShopRepository
	shopSettingsRepo   ShopPaymentSettingsRepository
	tontineWebhookUC   *ProcessTontineWebhookUsecase
	creditUpdater      CreditUpdater
	escrowRepo         repository.EscrowAccountRepository
	orderRepo          repository.OrderRepository
	commissionRateRepo repository.CommissionRateRepository
}

func NewProcessWebhookUsecase(
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
	txManager repository.TxManager, // 🆕 AJOUT
	db repository.DBExecutor,
	shopRepo repository.ShopRepository,
	shopSettingsRepo ShopPaymentSettingsRepository,
	tontineWebhookUC *ProcessTontineWebhookUsecase,
	creditUpdater CreditUpdater,
	escrowRepo repository.EscrowAccountRepository,
	orderRepo repository.OrderRepository,
) *ProcessWebhookUsecase {
	return &ProcessWebhookUsecase{
		paymentRepo:      paymentRepo,
		registry:         registry,
		txManager:        txManager, // 🆕 AJOUT
		db:               db,
		shopRepo:         shopRepo,
		shopSettingsRepo: shopSettingsRepo,
		tontineWebhookUC: tontineWebhookUC,
		creditUpdater:    creditUpdater,
		escrowRepo:       escrowRepo,
		orderRepo:        orderRepo,
	}
}

func (uc *ProcessWebhookUsecase) WithCommissionRateRepo(r repository.CommissionRateRepository) *ProcessWebhookUsecase {
	uc.commissionRateRepo = r
	return uc
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

	// P1-C: audit INSERT processed=false ; skip only if already processed=true
	if err := uc.recordWebhook(ctx, providerCode, event, payload, signature, true, ""); err != nil {
		if errors.Is(err, ErrWebhookAlreadyProcessed) {
			logger.Info().Msg("Webhook already processed (idempotent)")
			return ErrWebhookAlreadyProcessed
		}
		logger.Error().Err(err).Msg("Failed to record webhook audit")
		// continue: métier prioritaire si audit KO
	}

	var processErr error

	defer func() {
		if processErr != nil {
			_ = uc.markWebhookFailed(ctx, providerCode, event, processErr.Error())
			return
		}
		_ = uc.markWebhookProcessed(ctx, providerCode, event)
	}()

	if reference, ok := event.Metadata["reference"].(string); ok && IsTontineReference(reference) {
		logger.Info().
			Str("reference", reference).
			Str("transaction_id", event.ExternalID).
			Str("status", string(event.Status)).
			Msg("Tontine webhook detected, delegating to ProcessTontineWebhookUsecase")

		if uc.tontineWebhookUC == nil {
			processErr = fmt.Errorf("%w: tontine webhook handler not configured", ErrWebhookProcessing)
			return processErr
		}

		if err := uc.tontineWebhookUC.Execute(ctx, reference, event.ExternalID, event.Status); err != nil {
			logger.Error().Err(err).Msg("Tontine webhook processing failed")
			processErr = fmt.Errorf("%w: tontine webhook: %v", ErrWebhookProcessing, err)
			return processErr
		}

		logger.Info().Str("reference", reference).Msg("Tontine webhook processed successfully")
		return nil
	}

	paymentEntity, matchedBy, findErr := uc.resolvePaymentFromWebhookEvent(ctx, providerCode, event, logger)
	if findErr != nil || paymentEntity == nil {
		logger.Warn().
			Err(findErr).
			Str("provider_ref", event.ProviderRef).
			Interface("metadata_debug", event.Metadata).
			Msg("Payment definitively not found for webhook")
		processErr = fmt.Errorf("%w: payment not found for provider_ref %s", ErrWebhookProcessing, event.ProviderRef)
		return processErr
	}
	logger.Info().
		Str("payment_id", paymentEntity.ID.String()).
		Str("matched_by", matchedBy).
		Msg("Payment resolved for webhook")

	shop, err := uc.shopRepo.FindByID(ctx, paymentEntity.ShopID)
	if err != nil {
		processErr = fmt.Errorf("%w: shop not found for payment: %v", ErrWebhookProcessing, err)
		return processErr
	}
	ctx = tenant.WithTenant(ctx, shop)

	logger.Debug().Str("shop_id", shop.ID.String()).Str("shop_slug", shop.Slug).Msg("Tenant context injected for webhook processing")

	persistYengaIDs(paymentEntity, event)

	needsEscrowCreation := false

	if paymentEntity.IsTerminal() {
		escrowAlreadyCreated := false
		if paymentEntity.Metadata != nil {
			if created, ok := paymentEntity.Metadata["escrow_created"].(bool); ok && created {
				escrowAlreadyCreated = true
			}
		}

		if escrowAlreadyCreated {
			channelChanged := ApplyPayInChannelFromMeta(paymentEntity, event.Metadata, logger)
			if channelChanged {
				if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
					logger.Error().Err(updateErr).Msg("Failed to persist pay-in channel on already-processed payment")
				}
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment already terminal and escrow already created, ignoring webhook")
			uc.ensureOrderConfirmed(ctx, paymentEntity, logger)
			return nil
		}

		logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment is terminal but escrow not created yet, proceeding with escrow creation")
		needsEscrowCreation = true
	} else {
		switch event.Status {
		case entity.PaymentStatusSuccess:
			successRef := markSuccessProviderRef(paymentEntity, event)
			if err := paymentEntity.MarkSuccess(successRef); err != nil {
				processErr = fmt.Errorf("%w: mark success: %v", ErrWebhookProcessing, err)
				return processErr
			}
			ApplyPayInChannelFromMeta(paymentEntity, event.Metadata, logger)
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
				processErr = fmt.Errorf("%w: mark failed: %v", ErrWebhookProcessing, err)
				return processErr
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Str("reason", reason).Msg("Payment marked as FAILED")

		case entity.PaymentStatusCancelled:
			if err := paymentEntity.MarkCancelled(); err != nil {
				processErr = fmt.Errorf("%w: mark cancelled: %v", ErrWebhookProcessing, err)
				return processErr
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as CANCELLED")

		case entity.PaymentStatusRefunded:
			if err := paymentEntity.MarkRefunded(); err != nil {
				processErr = fmt.Errorf("%w: mark refunded: %v", ErrWebhookProcessing, err)
				return processErr
			}
			logger.Info().Str("payment_id", paymentEntity.ID.String()).Msg("Payment marked as REFUNDED")

		default:
			logger.Warn().Str("status", string(event.Status)).Msg("Unknown webhook status, ignoring")
			return nil
		}
	}

	if needsEscrowCreation && (event.Status == entity.PaymentStatusSuccess || paymentEntity.Status == entity.PaymentStatusSuccess) {
		ApplyPayInChannelFromMeta(paymentEntity, event.Metadata, logger)

		isCredit := paymentEntity.ReferenceType != nil && (*paymentEntity.ReferenceType == "credit_installment" || *paymentEntity.ReferenceType == "credit_down_payment")

		if !isCredit && paymentEntity.OrderID != uuid.Nil {
			shopIDStr := paymentEntity.ShopID.String()
			rateBps := ResolveOnlineCommissionBps(ctx, uc.commissionRateRepo, shopIDStr, logger)

			providerFeesCents := int64(0)
			if fees, ok := event.Metadata["payment_fees"].(float64); ok {
				providerFeesCents = int64(fees * 100)
			}

			settlement := ComputeOrderSettlement(
				paymentEntity.AmountCents,
				providerFeesCents,
				rateBps,
			)

			paymentEntity.ProviderFeesCents = settlement.ProviderFeesCents
			paymentEntity.CommissionRateBps = settlement.CommissionRateBps
			paymentEntity.CommissionCents = settlement.CommissionCents

			if settlement.MerchantNetCents > 0 && uc.escrowRepo != nil {
				orderIDStr := paymentEntity.OrderID.String()
				now := time.Now().UTC()

				escrow := &entity.EscrowAccount{
					OrderID:             &orderIDStr,
					SourceType:          entity.EscrowSourceOrder,
					TotalAmountCents:    settlement.EscrowTotalCents,
					ReleasedAmountCents: 0,
					CommissionCents:     settlement.CommissionCents,
					Status:              entity.EscrowAccountFundsHeld,
					FundsHeldAt:         now,
					CreatedAt:           now,
					UpdatedAt:           now,
				}

				// 🛡️ ATOMICITÉ : Transaction pour lier la création d'escrow et la mise à jour du paiement
				if uc.txManager != nil {
					tx, txErr := uc.txManager.BeginTx(ctx)
					if txErr != nil {
						processErr = fmt.Errorf("failed to begin tx: %w", txErr)
						return processErr
					}
					defer tx.Rollback()

					if err := uc.escrowRepo.WithTX(tx).Create(ctx, escrow); err != nil {
						processErr = fmt.Errorf("failed to create escrow: %w", err)
						return processErr
					}

					if paymentEntity.Metadata == nil {
						paymentEntity.Metadata = make(map[string]interface{})
					}
					paymentEntity.Metadata["escrow_created"] = true

					if err := uc.paymentRepo.WithTX(tx).Update(ctx, paymentEntity); err != nil {
						processErr = fmt.Errorf("failed to update payment with escrow flag: %w", err)
						return processErr
					}

					if err := tx.Commit(); err != nil {
						processErr = fmt.Errorf("failed to commit tx: %w", err)
						return processErr
					}
				} else {
					// Fallback si txManager n'est pas injecté (évite le panic)
					if err := uc.escrowRepo.Create(ctx, escrow); err != nil {
						processErr = fmt.Errorf("failed to create escrow: %w", err)
						return processErr
					}
					if paymentEntity.Metadata == nil {
						paymentEntity.Metadata = make(map[string]interface{})
					}
					paymentEntity.Metadata["escrow_created"] = true
					if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
						logger.Error().Err(updateErr).Msg("Failed to update payment metadata with escrow_created flag")
					}
				}

				logger.Info().
					Int64("gross_cents", settlement.GrossCents).
					Int64("provider_fees_cents", settlement.ProviderFeesCents).
					Int("commission_rate_bps", settlement.CommissionRateBps).
					Int64("commission_cents", settlement.CommissionCents).
					Int64("escrow_total_cents", settlement.EscrowTotalCents).
					Int64("merchant_net_cents", settlement.MerchantNetCents).
					Str("escrow_id", escrow.ID).
					Msg("Funds locked in Escrow atomically (Phase 2 settlement)")

				// Mise à jour de la commande si nécessaire
				uc.ensureOrderConfirmed(ctx, paymentEntity, logger)

				// Tout est réussi, on retourne nil directement pour éviter le double update en fin de fonction
				return nil
			}
		}
	}

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		processErr = fmt.Errorf("%w: update payment: %v", ErrWebhookProcessing, err)
		return processErr
	}

	logger.Info().Str("payment_id", paymentEntity.ID.String()).Str("status", string(paymentEntity.Status)).Msg("Webhook processed successfully")
	return nil
}

func (uc *ProcessWebhookUsecase) resolvePaymentFromWebhookEvent(
	ctx context.Context,
	providerCode entity.PaymentProvider,
	event *payment.WebhookEvent,
	logger *zerolog.Logger,
) (*entity.Payment, string, error) {
	tryRef := func(ref, label string) (*entity.Payment, string, error) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil, "", nil
		}
		p, err := uc.paymentRepo.FindByProviderRef(ctx, providerCode, ref)
		if err == nil && p != nil {
			return p, label, nil
		}
		return nil, "", err
	}

	if p, label, _ := tryRef(event.ProviderRef, "provider_ref"); p != nil {
		return p, label, nil
	}

	intentKeys := []string{
		"payment_intent_id", "paymentIntentId", "paymentIntentID",
		"payment_intent", "intent_id", "intentId",
		"transId", "transaction_id", "transactionId",
		"id",
	}
	seen := map[string]struct{}{}
	if event.ProviderRef != "" {
		seen[strings.TrimSpace(event.ProviderRef)] = struct{}{}
	}
	for _, k := range intentKeys {
		v := metaString(event.Metadata, k)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		if p, label, _ := tryRef(v, "metadata."+k); p != nil {
			return p, label, nil
		}
	}

	if ref := metaString(event.Metadata, "reference"); ref != "" {
		if pid, err := uuid.Parse(ref); err == nil {
			p, err := uc.paymentRepo.FindByID(ctx, pid)
			if err == nil && p != nil {
				return p, "reference_as_payment_id", nil
			}
			if logger != nil {
				logger.Debug().Err(err).Str("reference", ref).Msg("FindByID(payment) via reference failed")
			}
		}
	}

	orderIDStr := metaString(event.Metadata, "order_id")
	if orderIDStr == "" {
		if ref := metaString(event.Metadata, "reference"); strings.HasPrefix(ref, "ORDER-") {
			orderIDStr = strings.TrimPrefix(ref, "ORDER-")
		}
	}
	if orderIDStr != "" {
		if orderUUID, err := uuid.Parse(orderIDStr); err == nil {
			payments, searchErr := uc.paymentRepo.FindByOrderIDUnscoped(ctx, orderUUID)
			if searchErr == nil && len(payments) > 0 {
				// P1-D: prefer non-terminal, then most recent success candidate
				chosen := pickBestPaymentForOrder(payments)
				return chosen, "order_id", nil
			}
			if logger != nil && searchErr != nil {
				logger.Debug().Err(searchErr).Msg("FindByOrderIDUnscoped failed")
			}
		}
	}

	return nil, "", fmt.Errorf("no payment matched webhook identifiers")
}

// pickBestPaymentForOrder: avoid blind payments[0] (P1-D light).
func pickBestPaymentForOrder(payments []*entity.Payment) *entity.Payment {
	if len(payments) == 0 {
		return nil
	}
	if len(payments) == 1 {
		return payments[0]
	}
	// Prefer non-terminal (still open)
	for _, p := range payments {
		if p != nil && !p.IsTerminal() {
			return p
		}
	}
	// Else prefer SUCCESS already waiting for escrow
	for _, p := range payments {
		if p != nil && p.Status == entity.PaymentStatusSuccess {
			return p
		}
	}
	return payments[0]
}

func persistYengaIDs(p *entity.Payment, event *payment.WebhookEvent) {
	if p.Metadata == nil {
		p.Metadata = make(map[string]interface{})
	}
	if p.ProviderRef != nil && *p.ProviderRef != "" {
		if _, ok := p.Metadata["payment_intent_id"]; !ok {
			p.Metadata["payment_intent_id"] = *p.ProviderRef
		}
	}
	ref := strings.TrimSpace(event.ProviderRef)
	if ref != "" && strings.HasPrefix(strings.ToUpper(ref), "YP") {
		p.Metadata["yenga_trans_id"] = ref
	}
	if intent := metaString(event.Metadata, "payment_intent_id", "paymentIntentId"); intent != "" {
		p.Metadata["payment_intent_id"] = intent
	}
}

func markSuccessProviderRef(p *entity.Payment, event *payment.WebhookEvent) string {
	if p.ProviderRef != nil && strings.TrimSpace(*p.ProviderRef) != "" {
		return *p.ProviderRef
	}
	if intent := metaString(event.Metadata, "payment_intent_id", "paymentIntentId"); intent != "" {
		return intent
	}
	return event.ProviderRef
}

func (uc *ProcessWebhookUsecase) ensureOrderConfirmed(ctx context.Context, paymentEntity *entity.Payment, logger *zerolog.Logger) {
	if uc.orderRepo == nil || paymentEntity.OrderID == uuid.Nil {
		return
	}

	orderIDStr := paymentEntity.OrderID.String()
	order, err := uc.orderRepo.FindByID(ctx, orderIDStr)
	if err != nil || order == nil {
		if logger != nil {
			logger.Warn().Err(err).Str("order_id", orderIDStr).Msg("ensureOrderConfirmed: order not found")
		}
		return
	}

	if order.Status != string(entity.OrderStatusPending) {
		return
	}

	order.MarkAccepted()
	if err := uc.orderRepo.UpdateOrder(ctx, order); err != nil {
		if logger != nil {
			logger.Error().Err(err).Str("order_id", orderIDStr).Msg("ensureOrderConfirmed: failed to update order")
		}
		return
	}

	if logger != nil {
		logger.Info().Str("order_id", orderIDStr).Msg("Order confirmed after payment SUCCESS (webhook)")
	}
}

// recordWebhook: INSERT audit. processed starts false when signature is valid.
// ON CONFLICT: if already processed=true → ErrWebhookAlreadyProcessed; if false → allow retry.
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

	// P1-C: never mark processed=true at insert time
	processed := false

	query := `
		INSERT INTO payment_webhooks (
			provider, event_type, external_id, payload, signature,
			signature_validated, processing_error, processed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, external_id) DO NOTHING
	`

	res, err := uc.db.ExecContext(ctx, query,
		provider,
		eventType,
		externalID,
		payloadJSON,
		signature,
		signatureValid,
		nullIfEmpty(processingError),
		processed,
	)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to record webhook")
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 && externalID != "" {
		var already bool
		q := `SELECT processed FROM payment_webhooks WHERE provider = $1 AND external_id = $2`
		if scanErr := uc.db.QueryRowContext(ctx, q, provider, externalID).Scan(&already); scanErr == nil && already {
			return ErrWebhookAlreadyProcessed
		}
		// processed=false → retry path, not an error
		zerolog.Ctx(ctx).Info().
			Str("provider", string(provider)).
			Str("external_id", externalID).
			Msg("Webhook row exists with processed=false — retry allowed")
	}

	return nil
}

func (uc *ProcessWebhookUsecase) markWebhookProcessed(ctx context.Context, provider entity.PaymentProvider, event *payment.WebhookEvent) error {
	if uc.db == nil || event == nil || strings.TrimSpace(event.ExternalID) == "" {
		return nil
	}
	_, err := uc.db.ExecContext(ctx, `
		UPDATE payment_webhooks
		SET processed = true,
		    processing_error = NULL
		WHERE provider = $1 AND external_id = $2
	`, provider, event.ExternalID)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).
			Str("external_id", event.ExternalID).
			Msg("Failed to mark webhook processed")
	}
	return err
}

func (uc *ProcessWebhookUsecase) markWebhookFailed(ctx context.Context, provider entity.PaymentProvider, event *payment.WebhookEvent, errMsg string) error {
	if uc.db == nil || event == nil || strings.TrimSpace(event.ExternalID) == "" {
		return nil
	}
	// Keep processed=false so provider can retry
	_, err := uc.db.ExecContext(ctx, `
		UPDATE payment_webhooks
		SET processed = false,
		    processing_error = $3
		WHERE provider = $1 AND external_id = $2
	`, provider, event.ExternalID, truncateErr(errMsg, 500))
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).
			Str("external_id", event.ExternalID).
			Msg("Failed to mark webhook processing_error")
	}
	return err
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func truncateErr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
