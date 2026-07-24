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

// ShopPaymentSettingsRepository définit l'interface pour récupérer les settings
type ShopPaymentSettingsRepository interface {
	GetPaymentSettings(ctx context.Context, shopID uuid.UUID) (*entity.ShopPaymentSettings, error)
}

// InitiatePaymentUsecase initie un paiement pour une commande
type InitiatePaymentUsecase struct {
	paymentRepo     repository.PaymentRepository
	orderRepo       repository.OrderRepository
	registry        PaymentRegistry               // ✅ Interface
	shopPaymentRepo ShopPaymentSettingsRepository // Conservé pour compatibilité, mais non utilisé pour les clés YengaPay
}

// NewInitiatePaymentUsecase crée une nouvelle instance
func NewInitiatePaymentUsecase(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	registry PaymentRegistry, // ✅ Interface
) *InitiatePaymentUsecase {
	return &InitiatePaymentUsecase{
		paymentRepo: paymentRepo,
		orderRepo:   orderRepo,
		registry:    registry,
	}
}

// NewInitiatePaymentUsecaseWithShopSettings crée une instance avec support config boutique
func NewInitiatePaymentUsecaseWithShopSettings(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	registry PaymentRegistry, // ✅ Interface
	shopPaymentRepo ShopPaymentSettingsRepository,
) *InitiatePaymentUsecase {
	return &InitiatePaymentUsecase{
		paymentRepo:     paymentRepo,
		orderRepo:       orderRepo,
		registry:        registry,
		shopPaymentRepo: shopPaymentRepo,
	}
}

// Execute initie le paiement
func (uc *InitiatePaymentUsecase) Execute(ctx context.Context, req *paymentdto.InitiatePaymentRequest) (*paymentdto.InitiatePaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("order_id", req.OrderID).
		Str("provider", string(req.Provider)).
		Msg("Initiating payment")

	// 2. Valider et convertir l'OrderID en UUID
	orderUUID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order_id: %w", err)
	}

	// 3. Récupérer la commande
	order, err := uc.orderRepo.FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 4. Vérifier que la commande n'a pas déjà un paiement réussi
	existingPayments, err := uc.paymentRepo.FindByOrderID(ctx, orderUUID)
	if err != nil {
		return nil, fmt.Errorf("check existing payments: %w", err)
	}

	for _, p := range existingPayments {
		if p.Status == entity.PaymentStatusSuccess || p.Status == entity.PaymentStatusProcessing {
			return nil, fmt.Errorf("order already has an active payment")
		}
	}

	// 5. Calculer le montant total de la commande
	totalCents := calculateOrderTotal(order)
	if totalCents <= 0 {
		return nil, fmt.Errorf("order total must be positive")
	}

	// 6. Créer l'entité Payment
	newPayment, err := entity.NewPayment(shop.ID, orderUUID, req.Provider, totalCents)
	if err != nil {
		return nil, fmt.Errorf("create payment entity: %w", err)
	}

	newPayment.CustomerPhone = &req.PhoneNumber
	if req.CustomerEmail != "" {
		newPayment.CustomerEmail = &req.CustomerEmail
	}
	if req.Description != "" {
		newPayment.Description = &req.Description
	}

	// 7. Sauvegarder en statut PENDING
	if err := uc.paymentRepo.Create(ctx, newPayment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	// 8. Récupérer le provider (Utilise TOUJOURS la configuration globale de GoShop enregistrée dans app.go)
	provider, err := uc.registry.GetAvailable(ctx, req.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}

	// 9. Initier le paiement auprès du provider
	providerReq := &payment.PaymentRequest{
		PaymentID:   newPayment.ID.String(),
		AmountCents: totalCents,
		Currency:    entity.CurrencyXOF,
		PhoneNumber: req.PhoneNumber,
		CustomerRef: shop.OwnerID,
		Description: req.Description,
		CallbackURL: req.CallbackURL,
		Metadata: map[string]interface{}{
			"order_id": order.ID,
			"shop_id":  shop.ID.String(),
		},
	}

	// Fusionner les metadata de la requête (ex: flow, operator)
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			providerReq.Metadata[k] = v
		}
	}

	providerResp, err := provider.InitiatePayment(ctx, providerReq)
	if err != nil {
		if markErr := newPayment.MarkFailed(err.Error()); markErr != nil {
			logger.Error().Err(markErr).Msg("Failed to mark payment as failed")
		}
		if updateErr := uc.paymentRepo.Update(ctx, newPayment); updateErr != nil {
			logger.Error().Err(updateErr).Msg("Failed to update payment")
		}
		return nil, fmt.Errorf("initiate payment with provider: %w", err)
	}

	// 10. Mettre à jour le paiement avec la référence provider
	if providerResp.ProviderRef != "" {
		newPayment.ProviderRef = &providerResp.ProviderRef

		if err := newPayment.MarkProcessing(); err != nil {
			return nil, fmt.Errorf("mark payment as processing: %w", err)
		}
	}

	if providerResp.Metadata != nil {
		newPayment.Metadata = providerResp.Metadata
	}

	if err := uc.paymentRepo.Update(ctx, newPayment); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	logger.Info().
		Str("payment_id", newPayment.ID.String()).
		Str("provider_ref", providerResp.ProviderRef).
		Int64("amount_cents", totalCents).
		Msg("Payment initiated successfully with GLOBAL GoShop provider")

	// 11. Construire la réponse
	var expiresAt int64
	if newPayment.ExpiresAt != nil {
		expiresAt = newPayment.ExpiresAt.Unix()
	} else {
		expiresAt = time.Now().Add(30 * time.Minute).Unix()
	}

	return &paymentdto.InitiatePaymentResponse{
		PaymentID:   newPayment.ID.String(),
		ProviderRef: providerResp.ProviderRef,
		Status:      newPayment.Status,
		USSDCode:    providerResp.USSDCode,
		RedirectURL: providerResp.RedirectURL,
		ExpiresAt:   expiresAt,
		Message:     getMessageFromMetadata(providerResp.Metadata),
		Metadata:    providerResp.Metadata,
	}, nil
}

// Helpers

func calculateOrderTotal(order *entity.Order) int64 {
	var total int64
	for _, item := range order.Items {
		total += item.PriceCents * int64(item.Quantity)
	}
	return total
}

func getMessageFromMetadata(metadata map[string]interface{}) string {
	if msg, ok := metadata["notification"]; ok {
		if s, ok := msg.(string); ok {
			return s
		}
	}
	return "Payment initiated. Follow the instructions to complete."
}
