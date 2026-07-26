package paymentusecase

import (
	"context"
	"fmt"
	"time"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CompletePaymentUsecase struct {
	paymentRepo      repository.PaymentRepository
	orderRepo        repository.OrderRepository
	registry         PaymentRegistry
	shopSettingsRepo ShopPaymentSettingsRepository
	escrowRepo       repository.EscrowAccountRepository
}

func NewCompletePaymentUsecase(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	registry PaymentRegistry,
	shopSettingsRepo ShopPaymentSettingsRepository,
	escrowRepo repository.EscrowAccountRepository,
) *CompletePaymentUsecase {
	return &CompletePaymentUsecase{
		paymentRepo:      paymentRepo,
		orderRepo:        orderRepo,
		registry:         registry,
		shopSettingsRepo: shopSettingsRepo,
		escrowRepo:       escrowRepo,
	}
}

func (uc *CompletePaymentUsecase) Execute(ctx context.Context, req *paymentdto.CompletePaymentRequest) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("payment_id", req.PaymentID).
		Msg("Completing payment with OTP")

	paymentUUID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id format: %w", err)
	}

	paymentEntity, err := uc.paymentRepo.FindByID(ctx, paymentUUID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if paymentEntity.ShopID != shop.ID {
		return nil, fmt.Errorf("payment does not belong to current shop")
	}

	if paymentEntity.Status != entity.PaymentStatusProcessing {
		return nil, fmt.Errorf("payment is not in processing state (current: %s)", paymentEntity.Status)
	}

	provider, err := uc.registry.GetAvailable(ctx, paymentEntity.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}

	completable, ok := provider.(payment.ProviderCompletable)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support payment completion", paymentEntity.Provider)
	}

	if paymentEntity.Metadata == nil {
		return nil, fmt.Errorf("payment has no metadata (operator info missing)")
	}

	operatorCode, _ := paymentEntity.Metadata["operator"].(string)
	if operatorCode == "" {
		return nil, fmt.Errorf("operator code not found in payment metadata")
	}

	customerPhone := ""
	if paymentEntity.CustomerPhone != nil {
		customerPhone = *paymentEntity.CustomerPhone
	}
	if customerPhone == "" {
		return nil, fmt.Errorf("customer phone not found")
	}

	providerRef := ""
	if paymentEntity.ProviderRef != nil {
		providerRef = *paymentEntity.ProviderRef
	}
	if providerRef == "" {
		return nil, fmt.Errorf("provider reference not found")
	}

	completeResp, err := completable.CompletePayment(ctx, providerRef, operatorCode, customerPhone, req.OTP)
	if err != nil {
		if markErr := paymentEntity.MarkFailed(err.Error()); markErr != nil {
			logger.Error().Err(markErr).Msg("Failed to mark payment as failed")
		}
		if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
			logger.Error().Err(updateErr).Msg("Failed to update payment")
		}
		return nil, fmt.Errorf("complete payment with provider: %w", err)
	}

	if completeResp.Status == "DONE" {
		if err := paymentEntity.MarkSuccess(completeResp.TransactionID); err != nil {
			return nil, fmt.Errorf("mark payment as success: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Str("transaction_id", completeResp.TransactionID).
			Msg("Payment completed successfully")

		escrowAlreadyCreated := false
		if paymentEntity.Metadata != nil {
			if created, ok := paymentEntity.Metadata["escrow_created"].(bool); ok && created {
				escrowAlreadyCreated = true
			}
		}

		if !escrowAlreadyCreated && paymentEntity.OrderID != uuid.Nil {
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
			providerFeesCents := completeResp.Fees * 100
			paymentEntity.ProviderFeesCents = providerFeesCents
			netAmountCents := paymentEntity.AmountCents - providerFeesCents - commissionCents

			if netAmountCents > 0 && uc.escrowRepo != nil {
				orderIDStr := paymentEntity.OrderID.String()
				now := time.Now().UTC()

				escrow := &entity.EscrowAccount{
					OrderID:             &orderIDStr,
					SourceType:          entity.EscrowSourceOrder,
					TotalAmountCents:    paymentEntity.AmountCents - providerFeesCents, // 🆕 CORRECTION : Le montant séquestré est le Brut moins les frais opérateur
					ReleasedAmountCents: 0,
					CommissionCents:     commissionCents,
					Status:              entity.EscrowAccountFundsHeld,
					FundsHeldAt:         now,
					CreatedAt:           now,
					UpdatedAt:           now,
				}

				if err := uc.escrowRepo.Create(ctx, escrow); err != nil {
					logger.Error().Err(err).Msg("Failed to create escrow account in /complete")
					return nil, fmt.Errorf("failed to create escrow: %w", err)
				}

				if paymentEntity.Metadata == nil {
					paymentEntity.Metadata = make(map[string]interface{})
				}
				paymentEntity.Metadata["escrow_created"] = true

				logger.Info().
					Int64("gross_amount", paymentEntity.AmountCents).
					Int64("provider_fees_cents", providerFeesCents).
					Int64("commission_cents", commissionCents).
					Int64("escrow_amount", netAmountCents).
					Str("escrow_id", escrow.ID).
					Msg("✅ Funds successfully locked in Escrow via /complete")

				if uc.orderRepo != nil {
					order, err := uc.orderRepo.FindByID(ctx, orderIDStr)
					if err == nil && order != nil && order.Status == string(entity.OrderStatusPending) {
						order.Status = string(entity.OrderStatusConfirmed)
						acceptedAt := time.Now().UTC()
						order.AcceptedAt = &acceptedAt
						order.UpdatedAt = acceptedAt

						if err := uc.orderRepo.UpdateOrder(ctx, order); err != nil {
							logger.Error().Err(err).Msg("Failed to update order status to confirmed")
						} else {
							logger.Info().
								Str("order_id", orderIDStr).
								Msg("✅ Order status automatically updated to 'confirmed' (payment guaranteed)")
						}
					}
				}
			}
		}
	} else {
		return nil, fmt.Errorf("unexpected status from provider: %s", completeResp.Status)
	}

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	description := ""
	if paymentEntity.Description != nil {
		description = *paymentEntity.Description
	}

	return &paymentdto.PaymentResponse{
		ID:            paymentEntity.ID.String(),
		OrderID:       paymentEntity.OrderID.String(),
		Provider:      paymentEntity.Provider,
		ProviderRef:   providerRef,
		AmountCents:   paymentEntity.AmountCents,
		Currency:      paymentEntity.Currency,
		Status:        paymentEntity.Status,
		CustomerPhone: customerPhone,
		Description:   description,
		CreatedAt:     paymentEntity.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	}, nil
}
