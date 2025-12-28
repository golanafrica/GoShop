// interfaces/handler/orders/order_handler.go
// interfaces/handler/orders/order_handler.go
package orders

import (
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/application/mapper"
	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type OrderHandler struct {
	createOrderUsecase  *orderusecase.CreateOrderUsecase
	getOrderByIdUsecase *orderusecase.GetOrderByIdUsecase
	getAllOrderUsecase  *orderusecase.GetAllOrderUsecase
	productRepo         repository.ProductRepository
	//logger              *setupLogging.Logger
}

func NewOrderHandler(
	db *sql.DB,
	txManager repository.TxManager,
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	customerRepo repository.CustomerRepositoryInterface,
	orderItemRepo repository.OrderItemRepository,
	//logger *setupLogging.Logger,
) *OrderHandler {
	return &OrderHandler{
		createOrderUsecase:  orderusecase.NewCreateOrderUsecase(txManager, productRepo, customerRepo, orderItemRepo, orderRepo),
		getOrderByIdUsecase: orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager),
		getAllOrderUsecase:  orderusecase.NewGetAllOrderUsecase(orderRepo, txManager),
		productRepo:         productRepo,
		//logger:              logger.WithComponent("order_handler"),
	}
}

// ------------------------------------------------------------
//
//	CREATE ORDER
//
// ------------------------------------------------------------
func (h *OrderHandler) CreateOrderHandler(w http.ResponseWriter, r *http.Request) error {

	ctx := r.Context()

	start := time.Now()

	//logger := h.logger.WithOperation("create_order")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Starting order creation")

	// Décodage de la requête
	var req orderdto.OrderRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("content_type", r.Header.Get("Content-Type")).
			Int64("content_length", r.ContentLength).
			Msg("Failed to decode JSON payload")
		return utils.ErrInvalidPayload
	}

	logger.Debug().
		Str("customer_id", req.CustomerID).
		Int("items_count", len(req.Items)).
		Msg("Order request decoded successfully")

	// Validation
	if err := req.Validate(); err != nil {
		logger.Warn().
			Err(err).
			Interface("request", map[string]interface{}{
				"customer_id": req.CustomerID,
				"items_count": len(req.Items),
			}).
			Msg("Order validation failed")
		return utils.ErrValidationFailed
	}

	logger.Debug().Msg("Order request validated successfully")

	// Enrichissement des items de la commande
	logger.Info().Msg("Enriching order items with product information")
	var items []*entity.OrderItem
	var totalCents int64
	var totalItems int

	for idx, itemReq := range req.Items {
		itemLogger := logger.With().
			Int("item_index", idx).
			Str("product_id", itemReq.ProductID).
			Int("requested_quantity", int(itemReq.Quantity)).
			Logger()

		itemLogger.Debug().Msg("Processing order item")

		// Récupération du produit
		product, err := h.productRepo.FindByID(ctx, itemReq.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				itemLogger.Warn().
					Err(err).
					Msg("Product not found for order item")
				return utils.ErrProductNotFound
			}

			itemLogger.Error().
				Err(err).
				Stack().
				Msg("Failed to retrieve product for order item")
			return utils.ErrInternalServer
		}

		// Vérification du stock
		if product.Stock < int(itemReq.Quantity) {
			itemLogger.Warn().
				Int("available_stock", product.Stock).
				Int("requested_quantity", int(itemReq.Quantity)).
				Str("product_name", product.Name).
				Msg("Insufficient stock for product")
			return utils.ErrProductOutOfStock
		}

		// Calcul du sous-total
		subTotal := product.PriceCents * int64(itemReq.Quantity)
		items = append(items, &entity.OrderItem{
			ProductID:      itemReq.ProductID,
			Quantity:       int(itemReq.Quantity),
			PriceCents:     product.PriceCents,
			SubTotal_Cents: subTotal,
		})
		totalCents += subTotal
		totalItems += int(itemReq.Quantity)

		itemLogger.Debug().
			Str("product_name", product.Name).
			Int64("unit_price", product.PriceCents).
			Int64("subtotal", subTotal).
			Msg("Order item processed")
	}

	logger.Info().
		Int("total_items_processed", len(items)).
		Int("total_units", totalItems).
		Int64("total_amount_cents", totalCents).
		Msg("All order items enriched successfully")

	// Création de l'entité commande
	orderEntity := &entity.Order{
		CustomerID: req.CustomerID,
		TotalCents: totalCents,
		Status:     "pending",
		Items:      items,
	}

	logger.Debug().
		Str("customer_id", orderEntity.CustomerID).
		Str("status", orderEntity.Status).
		Msg("Order entity created")

	// Appel du usecase
	logger.Info().Msg("Executing create order usecase")
	createdOrder, err := h.createOrderUsecase.Execute(ctx, orderEntity)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Interface("order_details", map[string]interface{}{
				"customer_id": orderEntity.CustomerID,
				"total_cents": orderEntity.TotalCents,
				"items_count": len(orderEntity.Items),
				"status":      orderEntity.Status,
			}).
			Msg("Failed to create order")
		return utils.ErrOrderCreateFail
	}

	logger.Info().
		Str("order_id", createdOrder.ID).
		Str("customer_id", createdOrder.CustomerID).
		Int64("total_amount", createdOrder.TotalCents).
		Msg("Order created successfully in usecase")

	// Conversion en DTO de réponse
	response := mapper.ToOrderResponse(createdOrder)

	// Log de succès final
	logger.Info().
		Str("order_id", response.ID).
		Dur("total_duration", time.Since(start)).
		Int("http_status", http.StatusCreated).
		Msg("Order creation completed successfully")

	// Réponse HTTP
	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// ------------------------------------------------------------
//
//	GET ORDER BY ID
//
// ------------------------------------------------------------
func (h *OrderHandler) GetOrderByIdHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	id := chi.URLParam(r, "id")
	//logger := h.logger.WithOperation("get_order_by_id")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("order_id", id).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Retrieving order by ID")

	// Validation de l'ID
	if id == "" {
		logger.Warn().Msg("Empty order ID provided in request")
		return utils.ErrNotFound
	}

	logger.Debug().Msg("Validating order ID format")

	// Appel du usecase
	logger.Debug().Msg("Executing get order by ID usecase")
	order, err := h.getOrderByIdUsecase.Execute(ctx, id)
	if err != nil {
		logger.Warn().
			Err(err).
			Dur("duration_before_error", time.Since(start)).
			Msg("Order not found")
		return utils.ErrOrderNotFound
	}

	logger.Debug().
		Str("order_status", order.Status).
		Str("customer_id", order.CustomerID).
		Int64("total_amount", order.TotalCents).
		Int("items_count", len(order.Items)).
		Msg("Order retrieved from usecase")

	// Conversion en DTO de réponse
	response := mapper.ToOrderResponse(order)

	// Log de succès
	logger.Info().
		Str("order_id", response.ID).
		Str("order_status", response.Status).
		Dur("total_duration", time.Since(start)).
		Int("http_status", http.StatusOK).
		Msg("Order retrieved successfully")

	// Réponse HTTP
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ------------------------------------------------------------
//
//	GET ALL ORDERS
//
// ------------------------------------------------------------
func (h *OrderHandler) GetAllOrderHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()

	//logger := h.logger.WithOperation("get_all_orders")
	logger := zerolog.Ctx(ctx)
	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("query_params", r.URL.RawQuery).
		Msg("Retrieving all orders")

	// Récupération des paramètres de pagination
	limit, offset := extractPaginationParams(r, logger)

	logger.Debug().
		Int("limit", limit).
		Int("offset", offset).
		Msg("Pagination parameters")

	// Appel du usecase
	logger.Debug().Msg("Executing get all orders usecase")
	orders, err := h.getAllOrderUsecase.Execute(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to retrieve orders")
		return utils.ErrInternalServer
	}

	logger.Debug().
		Int("orders_count", len(orders)).
		Msg("Orders retrieved from usecase")

	if len(orders) == 0 {
		logger.Info().Msg("No orders found")
		utils.WriteJSON(w, http.StatusOK, []interface{}{})
		return nil
	}

	// Conversion en DTO de réponse
	logger.Debug().Msg("Converting orders to DTO response")
	response := make([]*orderdto.OrderResponseDto, len(orders))
	for i, ord := range orders {
		response[i] = mapper.ToOrderResponse(ord)
	}

	// Log de succès avec statistiques
	logger.Info().
		Int("orders_returned", len(response)).
		Dur("total_duration", time.Since(start)).
		Int("http_status", http.StatusOK).
		Msg("All orders retrieved successfully")

	// Réponse HTTP
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// Helper: extract pagination params — now expects *setupLogging.Logger
func extractPaginationParams(r *http.Request, logger *zerolog.Logger) (limit, offset int) {
	limit = 50 // default
	offset = 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
			logger.Debug().Int("parsed_limit", limit).Msg("Limit parameter parsed")
		} else {
			logger.Warn().
				Str("limit_value", limitStr).
				Err(err).
				Msg("Invalid limit parameter, using default")
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
			logger.Debug().Int("parsed_offset", offset).Msg("Offset parameter parsed")
		} else {
			logger.Warn().
				Str("offset_value", offsetStr).
				Err(err).
				Msg("Invalid offset parameter, using default")
		}
	}

	// Optional filters logging
	if status := r.URL.Query().Get("status"); status != "" {
		logger.Debug().Str("filter_status", status).Msg("Status filter applied")
	}
	if customerID := r.URL.Query().Get("customer_id"); customerID != "" {
		logger.Debug().Str("filter_customer_id", customerID).Msg("Customer filter applied")
	}

	return limit, offset
}

// [Optionnel] Métriques et middleware restent inchangés si tu les utilises ailleurs,
// mais ils ne sont pas nécessaires ici car ton App gère déjà le logging global.
