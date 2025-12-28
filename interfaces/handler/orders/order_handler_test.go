package orders_test

import (
	orderitemdto "Goshop/application/dto/orderItem_dto"
	orderdto "Goshop/application/dto/order_dto"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"Goshop/domain/entity"
	orderhandler "Goshop/interfaces/handler/orders"
	"Goshop/interfaces/middl"
	"Goshop/mocks/repository"
)

// ========================================
// Helper functions
// ========================================

func createTestCustomer(id string) *entity.Customer {
	return &entity.Customer{
		ID:        id,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestProduct(id string, stock int) *entity.Product {
	return &entity.Product{
		ID:          id,
		Name:        "Product " + id,
		Description: "Description " + id,
		PriceCents:  1000,
		Stock:       stock,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func createTestOrder(id, customerID string, total int64) *entity.Order {
	return &entity.Order{
		ID:         id,
		CustomerID: customerID,
		TotalCents: total,
		Status:     "PENDING",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Items: []*entity.OrderItem{
			{
				ID:             "item-1",
				OrderID:        id,
				ProductID:      "product-1",
				Quantity:       2,
				PriceCents:     500,
				SubTotal_Cents: 1000,
			},
		},
	}
}

func setupChiContext(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ========================================
// Tests
// ========================================

func TestOrderHandler_CreateOrder_Success(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocks
	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockProductRepoWithTX := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockCustomerRepoWithTX := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)
	mockOrderItemRepoWithTX := repository.NewMockOrderItemRepository(ctrl)

	// Pas besoin de mock DB pour le handler
	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Request body
	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items: []*orderitemdto.OrderItemRequestDto{
			{
				ProductID: "product-123",
				Quantity:  2,
			},
		},
	}

	// Mock data
	customer := createTestCustomer("customer-123")
	product := createTestProduct("product-123", 10)
	createdOrder := createTestOrder("order-456", "customer-123", 2000)

	// CORRECTION : Mock pour l'appel direct du handler à productRepo
	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// WithTX calls
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)

	// Customer check
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil)

	// Product check and stock update (avec transaction)
	mockProductRepoWithTX.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)
	mockProductRepoWithTX.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *entity.Product) (*entity.Product, error) {
			// Stock should be reduced by 2
			assert.Equal(t, 8, p.Stock)
			return p, nil
		})

	// Order creation
	mockOrderRepoWithTX.EXPECT().Create(gomock.Any(), gomock.Any()).Return(createdOrder, nil)

	// Order item creation
	mockOrderItemRepoWithTX.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.OrderItem{ID: "item-1"}, nil)

	// Transaction commit/rollback
	mockTx.EXPECT().Commit().Return(nil)
	// ✅ CORRECTION : Ajout de Rollback().AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "order-456", resp["id"])
	assert.Equal(t, float64(2000), resp["total_cents"]) // JSON numbers are float64
}

func TestOrderHandler_CreateOrder_InvalidPayload(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder
	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Invalid JSON
	invalidJSON := `{customer_id: "123", items: "not an array"}`

	// Act
	req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_PAYLOAD", resp["code"])
}

func TestOrderHandler_CreateOrder_ValidationFailed(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Empty items - should fail validation
	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items:      []*orderitemdto.OrderItemRequestDto{}, // Empty!
	}

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "VALIDATION_FAILED", resp["code"])
}

func TestOrderHandler_CreateOrder_CustomerNotFound(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockProductRepoWithTX := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockCustomerRepoWithTX := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)
	mockOrderItemRepoWithTX := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "non-existent-customer",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "product-123", Quantity: 1},
		},
	}

	// CORRECTION : Mock pour l'appel direct du handler à productRepo
	product := createTestProduct("product-123", 10)
	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)

	// ✅ CORRECTION : Utilisation de sql.ErrNoRows pour "not found"
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "non-existent-customer").Return(nil, sql.ErrNoRows)

	mockTx.EXPECT().Rollback().Return(nil)

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_CREATION_FAILED", resp["code"])
}

func TestOrderHandler_CreateOrder_ProductNotFound(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockProductRepoWithTX := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockCustomerRepoWithTX := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)
	mockOrderItemRepoWithTX := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "non-existent-product", Quantity: 1},
		},
	}

	customer := createTestCustomer("customer-123")

	// CORRECTION : Mock pour l'appel direct du handler à productRepo (retourne nil pour produit non trouvé)
	mockProductRepo.EXPECT().FindByID(gomock.Any(), "non-existent-product").Return(nil, sql.ErrNoRows)

	// CORRECTION : Les mocks de transaction NE SERONT PAS appelés car le handler retourne une erreur avant
	// Ne pas définir d'attentes pour ces mocks OU utiliser AnyTimes()
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX).AnyTimes()
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX).AnyTimes()
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX).AnyTimes()
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX).AnyTimes()
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil).AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert - CORRECTION : Attendre 404 "PRODUCT_NOT_FOUND"
	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp["code"])
}

func TestOrderHandler_CreateOrder_InsufficientStock(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockProductRepoWithTX := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockCustomerRepoWithTX := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)
	mockOrderItemRepoWithTX := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "product-123", Quantity: 10}, // Requesting 10
		},
	}

	customer := createTestCustomer("customer-123")
	product := createTestProduct("product-123", 5) // Only 5 in stock

	// CORRECTION : Mock pour l'appel direct du handler à productRepo
	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	// CORRECTION : Les mocks de transaction NE SERONT PAS appelés car le handler retourne une erreur avant
	// Ne pas définir d'attentes pour ces mocks OU utiliser AnyTimes()
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX).AnyTimes()
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX).AnyTimes()
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX).AnyTimes()
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX).AnyTimes()
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil).AnyTimes()
	mockProductRepoWithTX.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil).AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert - CORRECTION : Attendre 400 "PRODUCT_OUT_OF_STOCK"
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_OUT_OF_STOCK", resp["code"])
}

func TestOrderHandler_GetOrderById_Success(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	order := createTestOrder("order-123", "customer-123", 2000)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindByID(gomock.Any(), "order-123").Return(order, nil)
	mockTx.EXPECT().Commit().Return(nil)
	// ✅ CORRECTION : Ajout de Rollback().AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	req := httptest.NewRequest("GET", "/orders/order-123", nil)
	req = setupChiContext(req, "order-123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "order-123", resp["id"])
	assert.Equal(t, float64(2000), resp["total_cents"])
	assert.Equal(t, "PENDING", resp["status"])
}

func TestOrderHandler_GetOrderById_NotFound(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	// ✅ CORRECTION : Utilisation de sql.ErrNoRows pour "not found"
	mockOrderRepoWithTX.EXPECT().FindByID(gomock.Any(), "non-existent-order").Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil)

	// Act
	req := httptest.NewRequest("GET", "/orders/non-existent-order", nil)
	req = setupChiContext(req, "non-existent-order")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_NOT_FOUND", resp["code"])
}

func TestOrderHandler_GetOrderById_EmptyID(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Empty ID in URL param
	req := httptest.NewRequest("GET", "/orders/", nil)
	req = setupChiContext(req, "") // Empty ID
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", resp["code"])
}

func TestOrderHandler_GetAllOrders_Success(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	orders := []*entity.Order{
		createTestOrder("order-1", "customer-1", 1000),
		createTestOrder("order-2", "customer-2", 2000),
	}

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindAll(gomock.Any()).Return(orders, nil)
	mockTx.EXPECT().Commit().Return(nil)
	// ✅ CORRECTION : Ajout de Rollback().AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	req := httptest.NewRequest("GET", "/orders", nil)
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "order-1", resp[0]["id"])
	assert.Equal(t, "order-2", resp[1]["id"])
}

func TestOrderHandler_GetAllOrders_Empty(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
	)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindAll(gomock.Any()).Return([]*entity.Order{}, nil)
	mockTx.EXPECT().Commit().Return(nil)
	// ✅ CORRECTION : Ajout de Rollback().AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	req := httptest.NewRequest("GET", "/orders", nil)
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllOrderHandler)
	httpHandler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}
