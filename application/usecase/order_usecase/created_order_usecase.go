// application/usecase/order_usecase/created_order_usecase.go

package orderusecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"Goshop/application/metrics"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CreateOrderUsecase struct {
	txManager     repository.TxManager
	productRepo   repository.ProductRepository
	customerRepo  repository.CustomerRepositoryInterface
	orderItemRepo repository.OrderItemRepository
	orderRepo     repository.OrderRepository
	codProofRepo  repository.CODProofRepository // 🆕 v3.0.1
}

func NewCreateOrderUsecase(
	txManager repository.TxManager,
	productRepo repository.ProductRepository,
	customerRepo repository.CustomerRepositoryInterface,
	orderItemRepo repository.OrderItemRepository,
	orderRepo repository.OrderRepository,
	codProofRepo repository.CODProofRepository, // 🆕 v3.0.1 (peut être nil pour rétrocompatibilité)
) *CreateOrderUsecase {
	return &CreateOrderUsecase{
		txManager:     txManager,
		productRepo:   productRepo,
		customerRepo:  customerRepo,
		orderItemRepo: orderItemRepo,
		orderRepo:     orderRepo,
		codProofRepo:  codProofRepo,
	}
}

func (ouc *CreateOrderUsecase) Execute(ctx context.Context, order *entity.Order) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)
	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Int("items_count", len(order.Items)).
		Str("payment_method", order.PaymentMethod).
		Msg("Starting order creation process")

	// 1. Début de la transaction
	tx, err := ouc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("original_error", err.Error()).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// 2. Attacher les repositories à la transaction
	productRepo := ouc.productRepo.WithTX(tx)
	customerRepo := ouc.customerRepo.WithTX(tx)
	orderItemRepo := ouc.orderItemRepo.WithTX(tx)
	orderRepo := ouc.orderRepo.WithTX(tx)

	// 🆕 v3.0.1 : Attacher le repository COD à la transaction (si disponible)
	var codProofRepo repository.CODProofRepository
	if ouc.codProofRepo != nil {
		codProofRepo = ouc.codProofRepo.WithTX(tx)
	}

	// 🆕 v4.8.1 FIX B1 : Lire le tenant AVANT la création pour assigner ShopID
	shop, tenantErr := tenant.FromContext(ctx)
	if tenantErr != nil {
		return nil, fmt.Errorf("multi-tenant: no tenant in context: %w", tenantErr)
	}
	order.ShopID = shop.ID.String()

	logger.Debug().
		Str("shop_id", order.ShopID).
		Msg("ShopID assigned from tenant context")

	// 3. Vérifier le client
	customer, err := customerRepo.FindByCustomerID(ctx, order.CustomerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("customer not found")
		}
		return nil, fmt.Errorf("failed to retrieve customer: %w", err)
	}

	if customer == nil {
		return nil, errors.New("customer not found")
	}

	logger.Debug().
		Str("customer_id", order.CustomerID).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Customer verified")

	// 4. Traiter chaque item (décrémentation stock - Option B)
	var totalCents int64
	for i, item := range order.Items {
		itemLogger := logger.With().
			Int("item_index", i).
			Str("product_id", item.ProductID).
			Int("quantity", item.Quantity).
			Logger()

		product, err := productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("product not found")
			}
			return nil, fmt.Errorf("failed to retrieve product: %w", err)
		}

		if product.Stock < int(item.Quantity) {
			itemLogger.Warn().
				Int("available_stock", product.Stock).
				Int("requested_quantity", item.Quantity).
				Msg("Insufficient stock for product")
			return nil, errors.New("not enough stock for product")
		}

		item.PriceCents = product.PriceCents
		item.SubTotal_Cents = product.PriceCents * int64(item.Quantity)
		totalCents += item.SubTotal_Cents

		product.Stock -= item.Quantity
		if _, err := productRepo.Update(ctx, product); err != nil {
			return nil, fmt.Errorf("failed to update stock for product: %w", err)
		}

		itemLogger.Debug().
			Str("product_name", product.Name).
			Int64("subtotal", item.SubTotal_Cents).
			Int("new_stock", product.Stock).
			Msg("Order item processed successfully")
	}

	// 5. Définir le statut initial et reserved_until selon payment_method
	order.TotalCents = totalCents
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now

	// Valeur par défaut pour payment_method
	if order.PaymentMethod == "" {
		order.PaymentMethod = string(entity.PaymentMethodMobileMoney)
	}

	// Statut initial selon la méthode de paiement
	switch order.PaymentMethod {
	case string(entity.PaymentMethodCashOnDelivery):
		order.Status = string(entity.OrderStatusPendingConfirmation)
		// Réserver le stock pour 24h
		reservedUntil := now.Add(24 * time.Hour)
		order.ReservedUntil = &reservedUntil
		logger.Info().
			Str("payment_method", order.PaymentMethod).
			Str("status", order.Status).
			Time("reserved_until", reservedUntil).
			Msg("Cash order created - waiting for merchant confirmation")

	default:
		order.Status = string(entity.OrderStatusPending)
		logger.Info().
			Str("payment_method", order.PaymentMethod).
			Str("status", order.Status).
			Msg("Mobile money order created - waiting for payment")
	}

	// 6. Créer la commande (avec ShopID maintenant défini)
	createdOrder, err := orderRepo.Create(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// 7. Créer les items de commande
	for i, item := range order.Items {
		item.OrderID = createdOrder.ID
		if _, err := orderItemRepo.Create(ctx, item); err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
		logger.Debug().
			Int("item_index", i).
			Str("product_id", item.ProductID).
			Msg("Order item created")
	}

	// 🆕 v3.0.1 : Créer automatiquement une preuve COD si paiement à la livraison
	if createdOrder.PaymentMethod == string(entity.PaymentMethodCashOnDelivery) && codProofRepo != nil {
		// Le shopID est déjà dans order.ShopID (fix B1)
		shopID := createdOrder.ShopID

		if shopID != "" {
			// Calculer la commission (2.5% du total)
			commissionCents := createdOrder.TotalCents * 250 / 10000

			codProof := &entity.CODProof{
				ID:               uuid.New().String(),
				OrderID:          createdOrder.ID,
				ShopID:           shopID,
				CustomerID:       createdOrder.CustomerID,
				CommissionCents:  commissionCents,
				CommissionStatus: entity.CODCommissionPending,
				Status:           entity.CODProofPendingProofs,
				CreatedAt:        now,
				UpdatedAt:        now,
			}

			if err := codProofRepo.Create(ctx, codProof); err != nil {
				// Non bloquant : on log mais on continue
				logger.Error().
					Err(err).
					Str("order_id", createdOrder.ID).
					Msg("Failed to create COD proof (non-blocking)")
			} else {
				logger.Info().
					Str("order_id", createdOrder.ID).
					Str("cod_proof_id", codProof.ID).
					Int64("commission_cents", commissionCents).
					Msg("COD proof created automatically")
			}
		}
	}

	// 8. Commit de la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 9. Log de succès final
	duration := time.Since(start)
	logger.Info().
		Str("order_id", createdOrder.ID).
		Str("customer_id", createdOrder.CustomerID).
		Str("shop_id", createdOrder.ShopID). // 🆕 v4.8.1 : Log ShopID
		Str("payment_method", createdOrder.PaymentMethod).
		Str("status", createdOrder.Status).
		Int64("total_amount", createdOrder.TotalCents).
		Int("total_items", len(order.Items)).
		Dur("total_duration_ms", duration).
		Msg("Order creation completed successfully")

	// Métriques métier
	metrics.OrdersCreatedTotal.Inc()
	metrics.OrdersRevenueCentsTotal.Inc()

	return createdOrder, nil
}
