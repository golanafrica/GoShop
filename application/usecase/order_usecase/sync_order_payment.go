package orderusecase

import (
	"context"
	"fmt"
	"time"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// SYNC ORDER PAYMENT USECASE
// ============================================================

// SyncOrderPaymentRequest représente la requête de synchronisation
type SyncOrderPaymentRequest struct {
	OrderID string `json:"order_id"`
}

// SyncOrderPaymentResponse représente la réponse après synchronisation
type SyncOrderPaymentResponse struct {
	OrderID       string `json:"order_id"`
	PaymentID     string `json:"payment_id"`
	OrderStatus   string `json:"order_status"`
	PaymentStatus string `json:"payment_status"`
	EscrowStatus  string `json:"escrow_status,omitempty"`
	Synced        bool   `json:"synced"`
	Message       string `json:"message"`
}

// SyncOrderPaymentUsecase synchronise le statut d'un paiement order
type SyncOrderPaymentUsecase struct {
	orderRepo         repository.OrderRepository
	paymentRepo       repository.PaymentRepository
	escrowRepo        repository.EscrowAccountRepository
	deliveryProofRepo repository.DeliveryProofRepository
	paymentRegistry   paymentusecase.PaymentRegistry // ✅ Bon type
	notifService      service.NotificationService
	txManager         repository.TxManager
}

// NewSyncOrderPaymentUsecase crée une nouvelle instance
func NewSyncOrderPaymentUsecase(
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	escrowRepo repository.EscrowAccountRepository,
	deliveryProofRepo repository.DeliveryProofRepository,
	paymentRegistry paymentusecase.PaymentRegistry, // ✅ Bon type
	notifService service.NotificationService,
	txManager repository.TxManager,
) *SyncOrderPaymentUsecase {
	return &SyncOrderPaymentUsecase{
		orderRepo:         orderRepo,
		paymentRepo:       paymentRepo,
		escrowRepo:        escrowRepo,
		deliveryProofRepo: deliveryProofRepo,
		paymentRegistry:   paymentRegistry,
		notifService:      notifService,
		txManager:         txManager,
	}
}

// Execute synchronise le statut du paiement et crée l'escrow si nécessaire
func (uc *SyncOrderPaymentUsecase) Execute(ctx context.Context, req *SyncOrderPaymentRequest) (*SyncOrderPaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le tenant (shop)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Récupérer l'order (FindByID attend string)
	order, err := uc.orderRepo.FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 3. Vérifier que l'order appartient au shop
	if order.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("access denied: order does not belong to tenant shop")
	}

	// 4. Récupérer les payments associés (FindByOrderID attend uuid.UUID, retourne slice)
	orderUUID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID format: %w", err)
	}

	payments, err := uc.paymentRepo.FindByOrderID(ctx, orderUUID)
	if err != nil {
		return nil, fmt.Errorf("payment not found for order: %w", err)
	}

	if len(payments) == 0 {
		return nil, fmt.Errorf("no payment found for order %s", req.OrderID)
	}

	// Prendre le premier paiement (le plus récent)
	payment := payments[0]

	// 5. 🆕 Phase 2 : Si le payment est déjà success, vérifier l'escrow ET confirmer l'order
	if payment.Status == entity.PaymentStatusSuccess {
		escrow, escrowErr := uc.escrowRepo.FindByOrderID(ctx, req.OrderID)
		if escrowErr == nil && escrow != nil {
			// 🆕 Phase 2 : Filet de sécurité - confirmer l'order si encore pending
			orderConfirmed := false
			if order.Status == string(entity.OrderStatusPending) {
				if markErr := order.MarkAccepted(); markErr == nil {
					if updateErr := uc.orderRepo.UpdateOrder(ctx, order); updateErr == nil {
						orderConfirmed = true
						logger.Info().
							Str("order_id", req.OrderID).
							Str("new_status", order.Status).
							Msg("✅ Order confirmed (payment already success, escrow exists)")
					} else {
						logger.Warn().Err(updateErr).Str("order_id", req.OrderID).
							Msg("Failed to confirm order during sync")
					}
				} else {
					logger.Warn().Err(markErr).Str("order_id", req.OrderID).
						Msg("Failed to mark order accepted during sync")
				}
			}

			return &SyncOrderPaymentResponse{
				OrderID:       req.OrderID,
				PaymentID:     payment.ID.String(),
				OrderStatus:   order.Status, // Sera "confirmed" si on vient de le confirmer
				PaymentStatus: string(payment.Status),
				EscrowStatus:  string(escrow.Status),
				Synced:        orderConfirmed, // 🆕 Phase 2 : true si on a confirmé l'order
				Message: func() string {
					if orderConfirmed {
						return "Payment already confirmed, escrow exists, order ensured confirmed"
					}
					return "Payment already confirmed, escrow exists"
				}(),
			}, nil
		}
	}

	// 6. Récupérer le provider
	provider, err := uc.paymentRegistry.GetAvailable(ctx, payment.Provider)
	if err != nil {
		return nil, fmt.Errorf("payment provider not available: %w", err)
	}

	// 7. Vérifier le statut auprès du provider
	providerRef := ""
	if payment.ProviderRef != nil {
		providerRef = *payment.ProviderRef
	}

	logger.Info().
		Str("provider_ref", providerRef).
		Str("order_id", req.OrderID).
		Msg("Checking payment status with provider")

	statusResp, err := provider.CheckStatus(ctx, providerRef)
	if err != nil {
		logger.Warn().Err(err).Str("provider_ref", providerRef).Msg("Failed to check status with provider")
		return &SyncOrderPaymentResponse{
			OrderID:       req.OrderID,
			PaymentID:     payment.ID.String(),
			OrderStatus:   order.Status,
			PaymentStatus: string(payment.Status),
			EscrowStatus:  "unknown",
			Synced:        false,
			Message:       "Failed to check status with provider: " + err.Error(),
		}, nil
	}

	logger.Info().
		Str("provider_status", string(statusResp.Status)).
		Str("order_status", order.Status).
		Msg("Payment status retrieved from provider")

	// 8. Si le statut est SUCCESS et que l'order n'est pas encore confirmée
	if statusResp.Status == entity.PaymentStatusSuccess && order.Status != string(entity.OrderStatusConfirmed) {
		logger.Info().
			Str("order_id", req.OrderID).
			Msg("Payment is SUCCESS, confirming order and creating escrow")

		// Démarrer une transaction
		tx, err := uc.txManager.BeginTx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to start transaction: %w", err)
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback()
			}
		}()

		orderRepoTx := uc.orderRepo.WithTX(tx)
		paymentRepoTx := uc.paymentRepo.WithTX(tx)
		escrowRepoTx := uc.escrowRepo.WithTX(tx)
		deliveryProofRepoTx := uc.deliveryProofRepo.WithTX(tx)

		// 8.1. Mettre à jour le payment
		now := time.Now().UTC()
		payment.Status = entity.PaymentStatusSuccess
		payment.CompletedAt = &now
		payment.UpdatedAt = now
		if err = paymentRepoTx.Update(ctx, payment); err != nil {
			return nil, fmt.Errorf("failed to update payment: %w", err)
		}

		// 8.2. Mettre à jour l'order → confirmed
		if err = order.MarkAccepted(); err != nil {
			return nil, fmt.Errorf("failed to mark order accepted: %w", err)
		}
		if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to update order: %w", err)
		}

		// 8.3. Créer l'escrow account
		commissionCents := calculateOrderCommission(payment.AmountCents, 250) // 2.5%
		orderIDStr := req.OrderID
		escrow := &entity.EscrowAccount{
			ID:                  uuid.New().String(),
			OrderID:             &orderIDStr,
			SourceType:          entity.EscrowSourceOrder,
			TotalAmountCents:    payment.AmountCents,
			CommissionCents:     commissionCents,
			ReleasedAmountCents: 0,
			Status:              entity.EscrowAccountFundsHeld,
			FundsHeldAt:         now,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err = escrowRepoTx.Create(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to create escrow: %w", err)
		}

		// 8.4. Créer la delivery proof (pour le marchand)
		proof := &entity.DeliveryProof{
			ID:           uuid.New().String(),
			OrderID:      &orderIDStr,
			EscrowStatus: entity.EscrowPendingShipment,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err = deliveryProofRepoTx.Create(ctx, proof); err != nil {
			return nil, fmt.Errorf("failed to create delivery proof: %w", err)
		}

		// 8.5. Commit transaction
		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		// 8.6. Notification (fire-and-forget)
		if uc.notifService != nil {
			go func() {
				bgCtx := context.Background()
				_ = uc.notifService.NotifyOrderStatusChange(bgCtx, order, order.CustomerID, shop.ID)
			}()
		}

		return &SyncOrderPaymentResponse{
			OrderID:       req.OrderID,
			PaymentID:     payment.ID.String(),
			OrderStatus:   order.Status,
			PaymentStatus: string(payment.Status),
			EscrowStatus:  string(escrow.Status),
			Synced:        true,
			Message:       "Payment confirmed, escrow created (funds_held)",
		}, nil
	}

	// 9. Sinon, retourner le statut actuel
	return &SyncOrderPaymentResponse{
		OrderID:       req.OrderID,
		PaymentID:     payment.ID.String(),
		OrderStatus:   order.Status,
		PaymentStatus: string(payment.Status),
		EscrowStatus:  string(statusResp.Status),
		Synced:        false,
		Message:       fmt.Sprintf("Payment status is %s, no action needed", statusResp.Status),
	}, nil
}

// calculateOrderCommission calcule la commission pour une order (en bps)
func calculateOrderCommission(amountCents int64, commissionBps int) int64 {
	return (amountCents * int64(commissionBps)) / 10000
}
