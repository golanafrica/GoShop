// application/usecase/order_usecase/get_order_by_id_usecase.go
package orderusecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type GetOrderByIdUsecase struct {
	repo      repository.OrderRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewGetOrderByIdUsecase(
	repo repository.OrderRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *GetOrderByIdUsecase {
	return &GetOrderByIdUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("get_order_by_id_usecase"),
	}
}

func (uc *GetOrderByIdUsecase) Execute(ctx context.Context, id string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)
	if id == "" {
		logger.Warn().Msg("Empty order ID provided")
		return nil, fmt.Errorf("empty order ID")
	}

	start := time.Now()

	// ✅ Pas de logger local — utilise uc.logger directement
	logger.Info().
		Str("operation", "execute").
		Str("order_id", id).
		Msg("Starting order retrieval by ID")

	// 1. Début de la transaction (lecture seule)
	logger.Debug().
		Str("operation", "execute").
		Str("order_id", id).
		Msg("Beginning read-only transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("order_id", id).
			Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error().
				Err(rollbackErr).
				Str("operation", "execute").
				Str("order_id", id).
				Msg("Failed to rollback transaction")
		}
	}()

	// 2. Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("order_id", id).
		Msg("Repository attached to transaction")

	// 3. Lecture de la commande
	logger.Debug().
		Str("operation", "execute").
		Str("order_id", id).
		Msg("Fetching order from repository")

	order, err := repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Warn().
				Err(err).
				Dur("duration_before_error", time.Since(start)).
				Str("order_id", id).
				Msg("Order not found")
			return nil, fmt.Errorf("order not found: %w", err)
		}

		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("order_id", id).
			Msg("Failed to retrieve order from repository")
		return nil, fmt.Errorf("failed to retrieve order: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Str("order_id", order.ID).
		Str("order_status", order.Status).
		Str("customer_id", order.CustomerID).
		Int64("total_amount", order.TotalCents).
		Int("items_count", len(order.Items)).
		Msg("Order retrieved from repository")

	// 4. Analyser la commande (optionnel)
	uc.analyzeOrder(ctx, order)

	// 5. Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("order_id", order.ID).
		Msg("Committing read-only transaction")

	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("order_id", order.ID).
			Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Str("order_id", order.ID).
		Msg("Transaction committed successfully")

	// 6. Log de succès (sans métriques de performance)
	duration := time.Since(start)
	logger.Info().
		Str("order_id", order.ID).
		Str("order_status", order.Status).
		Str("customer_id", order.CustomerID).
		Int64("total_amount", order.TotalCents).
		Int("items_count", len(order.Items)).
		Dur("total_duration_ms", duration).
		Msg("Order retrieved successfully")

	return order, nil
}

// analyzeOrder — utilise uc.logger, pas de paramètre logger
func (uc *GetOrderByIdUsecase) analyzeOrder(ctx context.Context, order *entity.Order) {
	logger := zerolog.Ctx(ctx)
	analysis := map[string]interface{}{
		"order_id":    order.ID,
		"status":      order.Status,
		"customer_id": order.CustomerID,
		"total_cents": order.TotalCents,
		"items_count": len(order.Items),
		"created_at":  order.CreatedAt.Format(time.RFC3339),
		"updated_at":  order.UpdatedAt.Format(time.RFC3339),
	}

	if len(order.Items) > 0 {
		var itemsAnalysis []map[string]interface{}
		var totalUnits int
		var itemsTotalCents int64

		for _, item := range order.Items {
			itemsAnalysis = append(itemsAnalysis, map[string]interface{}{
				"product_id": item.ProductID,
				"quantity":   item.Quantity,
				"unit_price": item.PriceCents,
				"subtotal":   item.SubTotal_Cents,
			})
			totalUnits += item.Quantity
			itemsTotalCents += item.SubTotal_Cents
		}

		analysis["total_units"] = totalUnits
		analysis["items_total_cents"] = itemsTotalCents
		analysis["items_analysis"] = itemsAnalysis
	}

	logger.Debug().
		Str("operation", "analyze_order").
		Str("order_id", order.ID).
		Interface("order_analysis", analysis).
		Msg("Order details analysis")
}
