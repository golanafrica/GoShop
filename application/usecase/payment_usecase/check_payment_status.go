package paymentusecase

import (
	"context"
	"fmt"
	"time"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

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

// Execute vérifie le statut
func (uc *CheckPaymentStatusUsecase) Execute(ctx context.Context, paymentID string) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérifier le shop (multi-tenant)
	_, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Récupérer le paiement
	id, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id: %w", err)
	}

	paymentEntity, err := uc.paymentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 3. Si le paiement est dans un état non-terminal, vérifier auprès du provider
	if !paymentEntity.IsTerminal() && paymentEntity.ProviderRef != nil {
		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err == nil {
			status, err := provider.CheckStatus(ctx, *paymentEntity.ProviderRef)
			if err == nil && status.Status != paymentEntity.Status {
				switch status.Status {
				case entity.PaymentStatusSuccess:
					if err := paymentEntity.MarkSuccess(*paymentEntity.ProviderRef); err == nil {
						// Aligné sur process_webhook : confirmed pour autoriser shipping proof
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

						// 🚨 FIX CRUCIAL : Créer le compte séquestre et mettre à jour la commission en fallback
						orderIDStr := paymentEntity.OrderID.String()
						_, escrowErr := uc.escrowRepo.FindByOrderID(ctx, orderIDStr)
						if escrowErr != nil {
							logger.Info().Str("order_id", orderIDStr).Msg("Escrow not found, creating it now (fallback for missing webhook)")

							order, orderErr := uc.orderRepo.FindByID(ctx, orderIDStr)
							if orderErr == nil && order != nil {
								rateBps := 250 // fallback par défaut (2.5%)
								if uc.commissionRateRepo != nil {
									rateBps = ResolveOnlineCommissionBps(ctx, uc.commissionRateRepo, order.ShopID, logger)
								}

								grossCents := paymentEntity.AmountCents
								commissionCents := (grossCents * int64(rateBps)) / 10000
								providerFeesCents := int64(0)
								escrowTotalCents := grossCents - providerFeesCents

								// ✅ CORRECTION : Mettre à jour les champs de commission sur l'entité en mémoire
								paymentEntity.ProviderFeesCents = providerFeesCents
								paymentEntity.CommissionRateBps = rateBps
								paymentEntity.CommissionCents = commissionCents

								// ✅ CORRECTION CRUCIALE : Sauvegarder les commissions calculées en base de données AVANT de créer l'escrow
								if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
									logger.Error().Err(updateErr).Msg("Failed to update payment with commission data in fallback")
								}

								escrow := &entity.EscrowAccount{
									OrderID:             &orderIDStr,
									SourceType:          entity.EscrowSourceOrder,
									TotalAmountCents:    escrowTotalCents,
									ReleasedAmountCents: 0,
									CommissionCents:     commissionCents,
									Status:              entity.EscrowAccountFundsHeld,
									FundsHeldAt:         time.Now().UTC(),
									CreatedAt:           time.Now().UTC(),
									UpdatedAt:           time.Now().UTC(),
								}

								if err := uc.escrowRepo.Create(ctx, escrow); err != nil {
									logger.Error().Err(err).Str("order_id", orderIDStr).Msg("Failed to create fallback escrow account")
								} else {
									logger.Info().
										Int64("gross_cents", grossCents).
										Int64("commission_cents", commissionCents).
										Int("rate_bps", rateBps).
										Str("order_id", orderIDStr).
										Msg("✅ Fallback escrow account created and payment commission updated")
								}
							}
						}
					}
				case entity.PaymentStatusFailed:
					if err := paymentEntity.MarkFailed(status.FailureReason); err == nil {
						_ = uc.paymentRepo.Update(ctx, paymentEntity)
					}
				}
			}
		}
	}

	logger.Debug().
		Str("payment_id", paymentID).
		Str("status", string(paymentEntity.Status)).
		Msg("Payment status checked")

	// 4. Construire la réponse
	return mapPaymentToResponse(paymentEntity), nil
}

// formatTimeUTC formate un time.Time en UTC
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
