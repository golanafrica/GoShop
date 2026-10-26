package paymentusecase

import (
	"context"
	"fmt"
	"time"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	paymentinfra "Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// CheckPaymentStatusUsecase vérifie le statut d'un paiement
type CheckPaymentStatusUsecase struct {
	paymentRepo        repository.PaymentRepository
	orderRepo          repository.OrderRepository
	registry           PaymentRegistry
	escrowRepo         repository.EscrowAccountRepository
	commissionRateRepo repository.CommissionRateRepository
}

// NewCheckPaymentStatusUsecase crée une nouvelle instance
func NewCheckPaymentStatusUsecase(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	registry PaymentRegistry,
	escrowRepo repository.EscrowAccountRepository,
	commissionRateRepo repository.CommissionRateRepository,
) *CheckPaymentStatusUsecase {
	return &CheckPaymentStatusUsecase{
		paymentRepo:        paymentRepo,
		orderRepo:          orderRepo,
		registry:           registry,
		escrowRepo:         escrowRepo,
		commissionRateRepo: commissionRateRepo,
	}
}

// Execute vérifie le statut auprès du provider et enrichit le canal pay-in
// (payment_source, operator, MSISDN réel) pour les refunds customer_wins.
func (uc *CheckPaymentStatusUsecase) Execute(ctx context.Context, paymentID string) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	_, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	id, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id: %w", err)
	}

	paymentEntity, err := uc.paymentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 🆕 C3-B : Lookup par référence marchande si provider_ref est manquant
	lookupKey := ""
	if paymentEntity.ProviderRef != nil && *paymentEntity.ProviderRef != "" {
		lookupKey = *paymentEntity.ProviderRef
	} else if !paymentEntity.IsTerminal() {
		lookupKey = paymentEntity.ID.String() // Utiliser l'ID du paiement comme référence marchande
		logger.Info().
			Str("payment_id", lookupKey).
			Msg("C3-B: CheckStatus by merchant reference (missing provider_ref)")
	}

	needsProviderPoll := lookupKey != "" && (!paymentEntity.IsTerminal() || paymentNeedsChannelEnrichment(paymentEntity))

	if needsProviderPoll {
		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err == nil {
			status, err := provider.CheckStatus(ctx, lookupKey)
			if err == nil && status != nil {
				// Si on a récupéré un vrai provider_ref via le lookup, on le sauvegarde
				if paymentEntity.ProviderRef == nil || *paymentEntity.ProviderRef == "" {
					if status.ProviderRef != "" {
						ref := status.ProviderRef
						paymentEntity.ProviderRef = &ref
					} else if status.Metadata != nil {
						if idStr, ok := status.Metadata["id"].(string); ok && idStr != "" {
							paymentEntity.ProviderRef = &idStr
						} else if idStr, ok := status.Metadata["paymentIntentId"].(string); ok && idStr != "" {
							paymentEntity.ProviderRef = &idStr
						}
					}
					// Fallback: garder la merchant ref si rien d'autre n'est trouvé
					if paymentEntity.ProviderRef == nil || *paymentEntity.ProviderRef == "" {
						ref := lookupKey
						paymentEntity.ProviderRef = &ref
					}
				}

				// Toujours tenter d'appliquer le canal (source + vrai MSISDN)
				channelChanged := ApplyPayInChannelFromStatus(paymentEntity, status, logger)

				if status.Status != paymentEntity.Status {
					switch status.Status {
					case entity.PaymentStatusSuccess:
						if err := paymentEntity.MarkSuccess(*paymentEntity.ProviderRef); err == nil {
							_ = ApplyPayInChannelFromStatus(paymentEntity, status, logger)

							if err := uc.orderRepo.UpdateStatus(ctx, paymentEntity.OrderID, string(entity.OrderStatusConfirmed)); err != nil {
								logger.Error().Err(err).
									Str("order_id", paymentEntity.OrderID.String()).
									Msg("Failed to update order status to confirmed")
							} else {
								logger.Info().
									Str("order_id", paymentEntity.OrderID.String()).
									Str("payment_id", paymentID).
									Msg("Order status updated to confirmed after provider success")
							}

							// Fallback escrow si webhook manquant
							uc.ensureEscrowOnSuccess(ctx, paymentEntity, status, logger)
						}
					case entity.PaymentStatusFailed:
						if err := paymentEntity.MarkFailed(status.FailureReason); err == nil {
							if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
								logger.Error().Err(err).Msg("Failed to persist payment after provider failed")
							}
						}
					}
				} else if channelChanged || (paymentEntity.ProviderRef != nil && *paymentEntity.ProviderRef == lookupKey && lookupKey == paymentEntity.ID.String()) {
					// Déjà success : MSISDN / source viennent d'arriver → persister
					// OU on vient de récupérer le provider_ref via merchant reference
					if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
						logger.Error().Err(err).Msg("Failed to persist pay-in channel or recovered provider_ref")
					} else {
						if channelChanged {
							logger.Info().Str("payment_id", paymentID).Msg("Pay-in channel persisted on already-success payment")
						} else {
							logger.Info().Str("provider_ref", *paymentEntity.ProviderRef).Msg("C3-B: provider_ref recovered via merchant reference")
						}
					}
				}
			} else if err != nil {
				// 404 / unknown → ne pas MarkFailed ; le scheduler C3-A a déjà flaggé ou réessaiera
				logger.Warn().Err(err).
					Str("payment_id", lookupKey).
					Msg("C3-B: lookup by merchant reference failed — leave for manual recon or retry")
			}
		} else {
			logger.Warn().Err(err).Msg("Provider not available for status check")
		}
	}

	logger.Debug().
		Str("payment_id", paymentID).
		Str("status", string(paymentEntity.Status)).
		Msg("Payment status checked")

	return mapPaymentToResponse(paymentEntity), nil
}

// ensureEscrowOnSuccess crée l'escrow + commission si absent (aligné process_webhook).
func (uc *CheckPaymentStatusUsecase) ensureEscrowOnSuccess(
	ctx context.Context,
	paymentEntity *entity.Payment,
	status *paymentinfra.PaymentStatus,
	logger *zerolog.Logger,
) {
	if uc.escrowRepo == nil || paymentEntity.OrderID == uuid.Nil {
		return
	}

	orderIDStr := paymentEntity.OrderID.String()

	if _, err := uc.escrowRepo.FindByOrderID(ctx, orderIDStr); err == nil {
		logger.Info().
			Str("order_id", orderIDStr).
			Msg("Escrow already exists — skip fallback creation")
		if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
			logger.Error().Err(updateErr).Msg("Failed to update payment after success (escrow already existed)")
		}
		return
	}

	shopIDStr := paymentEntity.ShopID.String()
	rateBps := ResolveOnlineCommissionBps(ctx, uc.commissionRateRepo, shopIDStr, logger)

	providerFeesCents := extractProviderFeesCents(status)

	settlement := ComputeOrderSettlement(
		paymentEntity.AmountCents,
		providerFeesCents,
		rateBps,
	)

	paymentEntity.ProviderFeesCents = settlement.ProviderFeesCents
	paymentEntity.CommissionRateBps = settlement.CommissionRateBps
	paymentEntity.CommissionCents = settlement.CommissionCents
	if paymentEntity.Metadata == nil {
		paymentEntity.Metadata = make(map[string]interface{})
	}
	paymentEntity.Metadata["escrow_created"] = true

	if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
		logger.Error().Err(updateErr).Msg("Failed to update payment with commission data in fallback")
		return
	}

	if settlement.MerchantNetCents <= 0 {
		logger.Warn().
			Str("order_id", orderIDStr).
			Int64("merchant_net_cents", settlement.MerchantNetCents).
			Msg("Merchant net <= 0 — skip escrow creation")
		return
	}

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

	if err := uc.escrowRepo.Create(ctx, escrow); err != nil {
		logger.Error().Err(err).Str("order_id", orderIDStr).Msg("Failed to create fallback escrow account")
		return
	}

	logger.Info().
		Int64("gross_cents", settlement.GrossCents).
		Int64("provider_fees_cents", settlement.ProviderFeesCents).
		Int("commission_rate_bps", settlement.CommissionRateBps).
		Int64("commission_cents", settlement.CommissionCents).
		Int64("escrow_total_cents", settlement.EscrowTotalCents).
		Int64("merchant_net_cents", settlement.MerchantNetCents).
		Str("order_id", orderIDStr).
		Msg("✅ Fallback escrow created (ComputeOrderSettlement, aligned with webhook)")
}

func extractProviderFeesCents(status *paymentinfra.PaymentStatus) int64 {
	if status == nil || status.Metadata == nil {
		return 0
	}
	if v, ok := status.Metadata["provider_fees_cents"]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	if v, ok := status.Metadata["payment_fees"]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n * 100)
		case int64:
			return n * 100
		case int:
			return int64(n) * 100
		}
	}
	return 0
}

func formatTimeUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func mapPaymentToResponse(p *entity.Payment) *paymentdto.PaymentResponse {
	resp := &paymentdto.PaymentResponse{
		ID:          p.ID.String(),
		OrderID:     p.OrderID.String(),
		Provider:    p.Provider,
		AmountCents: p.AmountCents,
		Currency:    p.Currency,
		Status:      p.Status,
		CreatedAt:   formatTimeUTC(p.CreatedAt),
	}

	if p.ProviderRef != nil {
		resp.ProviderRef = *p.ProviderRef
	}
	if p.CustomerPhone != nil {
		resp.CustomerPhone = *p.CustomerPhone
	}
	if p.Description != nil {
		resp.Description = *p.Description
	}
	if p.InitiatedAt != nil {
		resp.InitiatedAt = formatTimeUTC(*p.InitiatedAt)
	}
	if p.CompletedAt != nil {
		resp.CompletedAt = formatTimeUTC(*p.CompletedAt)
	}

	return resp
}
