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

type GetAllOrderUsecase struct {
	repo      repository.OrderRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewGetAllOrderUsecase(
	repo repository.OrderRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *GetAllOrderUsecase {
	return &GetAllOrderUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("get_all_orders_usecase"),
	}
}

func (uc *GetAllOrderUsecase) Execute(ctx context.Context) ([]*entity.Order, error) {
	logger := zerolog.Ctx(ctx)
	start := time.Now()

	// ✅ Pas de logger local — utilise uc.logger directement
	logger.Info().
		Str("operation", "execute").
		Msg("Starting retrieval of all orders")

	// 1. Début de la transaction (lecture seule)
	logger.Debug().
		Str("operation", "execute").
		Msg("Beginning read-only transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback sécurisé
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error().
				Err(rollbackErr).
				Str("operation", "execute").
				Msg("Failed to rollback transaction")
		}
	}()

	// 2. Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Msg("Repository attached to transaction")

	// 3. Récupérer toutes les commandes
	logger.Debug().
		Str("operation", "execute").
		Msg("Fetching all orders from repository")

	orders, err := repo.FindAll(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Msg("Failed to retrieve orders from repository")
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Int("orders_count", len(orders)).
		Msg("Orders retrieved from repository")

	// CORRECTION : Logique de traitement (analyse uniquement si des commandes existent)
	if len(orders) > 0 {
		// 4. Analyser les commandes (optionnel)
		uc.analyzeOrders(ctx, orders)
		logger.Info().
			Str("operation", "execute").
			Int("orders_found", len(orders)).
			Msg("Orders found and analyzed")
	} else {
		logger.Info().
			Str("operation", "execute").
			Msg("No orders found in the system")
	}

	// 5. Commit de la transaction (lecture seule) - CORRECTION : TOUJOURS appelé
	logger.Debug().
		Str("operation", "execute").
		Msg("Committing read-only transaction")

	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Msg("Transaction committed successfully")

	// 6. Log de succès (sans métriques de performance)
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Int("orders_returned", len(orders)).
		Dur("total_duration_ms", duration).
		Msg("All orders retrieved successfully")

	// CORRECTION : Retourner APRÈS le commit
	return orders, nil
}

// analyzeOrders — sans logger en paramètre (utilise uc.logger)
func (uc *GetAllOrderUsecase) analyzeOrders(ctx context.Context, orders []*entity.Order) {
	logger := zerolog.Ctx(ctx)
	if len(orders) == 0 {
		return
	}

	statusCount := make(map[string]int)
	var totalAmount int64
	var oldestOrder, newestOrder time.Time

	for i, order := range orders {
		statusCount[order.Status]++
		totalAmount += order.TotalCents

		if i == 0 || order.CreatedAt.Before(oldestOrder) {
			oldestOrder = order.CreatedAt
		}
		if i == 0 || order.CreatedAt.After(newestOrder) {
			newestOrder = order.CreatedAt
		}
	}

	// Log au niveau Debug (pas de metrics, juste infos utiles)
	logger.Debug().
		Str("operation", "analyze_orders").
		Interface("status_distribution", statusCount).
		Int64("total_amount_cents", totalAmount).
		Float64("average_order_amount", float64(totalAmount)/float64(len(orders))).
		Time("oldest_order", oldestOrder).
		Time("newest_order", newestOrder).
		Msg("Order statistics analysis")
}
