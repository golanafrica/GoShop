// application/usecase/order_usecase/create_order_usecase.go
// application/usecase/order_usecase/create_order_usecase.go
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

	"github.com/rs/zerolog"
)

type CreateOrderUsecase struct {
	txManager     repository.TxManager
	productRepo   repository.ProductRepository
	customerRepo  repository.CustomerRepositoryInterface
	orderItemRepo repository.OrderItemRepository
	orderRepo     repository.OrderRepository
	//logger        *setupLogging.Logger
}

func NewCreateOrderUsecase(
	txManager repository.TxManager,
	productRepo repository.ProductRepository,
	customerRepo repository.CustomerRepositoryInterface,
	orderItemRepo repository.OrderItemRepository,
	orderRepo repository.OrderRepository,
	//logger *setupLogging.Logger,
) *CreateOrderUsecase {
	return &CreateOrderUsecase{
		txManager:     txManager,
		productRepo:   productRepo,
		customerRepo:  customerRepo,
		orderItemRepo: orderItemRepo,
		orderRepo:     orderRepo,
		//	logger:        logger.WithComponent("create_order_usecase"),
	}
}

func (ouc *CreateOrderUsecase) Execute(ctx context.Context, order *entity.Order) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)
	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Int("items_count", len(order.Items)).
		Msg("Starting order creation process")

	// 1. Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Beginning transaction")
	tx, err := ouc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", order.CustomerID).
			Msg("Failed to start transaction")
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("original_error", err.Error()).
					Str("operation", "execute").
					Str("customer_id", order.CustomerID).
					Msg("Failed to rollback transaction")
			} else {
				logger.Debug().
					Str("operation", "execute").
					Str("customer_id", order.CustomerID).
					Msg("Transaction rolled back due to error")
			}
		}
	}()

	// 2. Attacher les repositories à la transaction
	productRepo := ouc.productRepo.WithTX(tx)
	customerRepo := ouc.customerRepo.WithTX(tx)
	orderItemRepo := ouc.orderItemRepo.WithTX(tx)
	orderRepo := ouc.orderRepo.WithTX(tx)

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Repositories attached to transaction")

	// 3. Vérifier le client
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Verifying customer")
	customer, err := customerRepo.FindByCustomerID(ctx, order.CustomerID)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Warn().
				Str("customer_id", order.CustomerID).
				Msg("Customer not found")
			return nil, errors.New("customer not found")
		}
		logger.Error().
			Err(err).
			Str("customer_id", order.CustomerID).
			Msg("Failed to retrieve customer")
		return nil, fmt.Errorf("failed to retrieve customer: %w", err)
	}

	if customer == nil {
		logger.Warn().Str("customer_id", order.CustomerID).Msg("Customer not found")
		return nil, errors.New("customer not found")
	}

	logger.Debug().
		Str("customer_id", order.CustomerID).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Customer verified")

	// 4. Traiter chaque item
	var totalCents int64
	logger.Info().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Processing order items")

	for i, item := range order.Items {
		itemLogger := logger.With().
			Str("operation", "execute").
			Str("customer_id", order.CustomerID).
			Int("item_index", i).
			Str("product_id", item.ProductID).
			Int("quantity", item.Quantity).
			Logger()

		itemLogger.Debug().Msg("Processing order item")

		product, err := productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				itemLogger.Warn().Msg("Product not found")
				return nil, errors.New("product not found")
			}
			itemLogger.Error().Err(err).Msg("Failed to retrieve product")
			return nil, fmt.Errorf("failed to retrieve product: %w", err)
		}

		if product.Stock < int(item.Quantity) {
			itemLogger.Warn().
				Int("available_stock", product.Stock).
				Int("requested_quantity", item.Quantity).
				Str("product_name", product.Name).
				Msg("Insufficient stock for product")
			return nil, errors.New("not enough stock for product")
		}

		item.PriceCents = product.PriceCents
		item.SubTotal_Cents = product.PriceCents * int64(item.Quantity)
		totalCents += item.SubTotal_Cents

		product.Stock -= item.Quantity
		if _, err := productRepo.Update(ctx, product); err != nil {
			itemLogger.Error().
				Err(err).
				Str("product_name", product.Name).
				Msg("Failed to update product stock")
			return nil, fmt.Errorf("failed to update stock for product: %w", err)
		}

		itemLogger.Debug().
			Str("product_name", product.Name).
			Int64("unit_price", product.PriceCents).
			Int64("subtotal", item.SubTotal_Cents).
			Int("new_stock", product.Stock).
			Msg("Order item processed successfully")
	}

	logger.Info().
		Str("customer_id", order.CustomerID).
		Int("items_processed", len(order.Items)).
		Int64("total_amount", totalCents).
		Msg("All order items processed")

	// 5. Créer la commande
	order.TotalCents = totalCents
	order.Status = "PENDING"
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Creating order in repository")
	createdOrder, err := orderRepo.Create(ctx, order)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("customer_id", order.CustomerID).
			Int64("total_cents", order.TotalCents).
			Int("items_count", len(order.Items)).
			Str("status", order.Status).
			Msg("Failed to create order")
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	logger.Debug().
		Str("order_id", createdOrder.ID).
		Str("customer_id", createdOrder.CustomerID).
		Msg("Order created in repository")

	// 6. Créer les items de commande
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Creating order items")
	for i, item := range order.Items {
		item.OrderID = createdOrder.ID
		if _, err := orderItemRepo.Create(ctx, item); err != nil {
			logger.Error().
				Err(err).
				Stack().
				Str("order_id", createdOrder.ID).
				Int("item_index", i).
				Str("product_id", item.ProductID).
				Msg("Failed to create order item")
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}

	logger.Debug().
		Str("order_id", createdOrder.ID).
		Int("order_items_created", len(order.Items)).
		Msg("All order items created successfully")

	// 7. Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Committing transaction")
	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", order.CustomerID).
			Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", order.CustomerID).
		Msg("Transaction committed successfully")

	// 8. Log de succès final
	duration := time.Since(start)
	logger.Info().
		Str("order_id", createdOrder.ID).
		Str("customer_id", createdOrder.CustomerID).
		Int64("total_amount", createdOrder.TotalCents).
		Int("total_items", len(order.Items)).
		Dur("total_duration_ms", duration).
		Msg("Order creation completed successfully")

		// ✅ Métriques métier — uniquement après commit réussi
	metrics.OrdersCreatedTotal.Inc()
	metrics.OrdersRevenueCentsTotal.Inc()

	return createdOrder, nil
}
