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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"Goshop/domain/entity"
	"Goshop/domain/tenant"
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

// 🆕 PR-8 FIX : Injecte un contexte multi-tenant fictif pour les tests HTTP
func setupTenantContext(r *http.Request) *http.Request {
	shop := &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		IsActive: true,
	}
	return r.WithContext(tenant.WithTenant(r.Context(), shop))
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

	var db *sql.DB // placeholder

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
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

	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	// Mock expectations
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)

	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil)

	mockProductRepoWithTX.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)
	mockProductRepoWithTX.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *entity.Product) (*entity.Product, error) {
			assert.Equal(t, 8, p.Stock)
			return p, nil
		})

	mockOrderRepoWithTX.EXPECT().Create(gomock.Any(), gomock.Any()).Return(createdOrder, nil)
	mockOrderItemRepoWithTX.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.OrderItem{ID: "item-1"}, nil)

	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Act
	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
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
	assert.Equal(t, float64(2000), resp["total_cents"])
}

func TestOrderHandler_CreateOrder_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB
	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	invalidJSON := `{customer_id: "123", items: "not an array"}`

	req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_PAYLOAD", resp["code"])
}

func TestOrderHandler_CreateOrder_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items:      []*orderitemdto.OrderItemRequestDto{},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "VALIDATION_FAILED", resp["code"])
}

func TestOrderHandler_CreateOrder_CustomerNotFound(t *testing.T) {
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

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "non-existent-customer",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "product-123", Quantity: 1},
		},
	}

	product := createTestProduct("product-123", 10)
	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)

	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "non-existent-customer").Return(nil, sql.ErrNoRows)

	mockTx.EXPECT().Rollback().Return(nil)

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_CREATION_FAILED", resp["code"])
}

func TestOrderHandler_CreateOrder_ProductNotFound(t *testing.T) {
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

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "non-existent-product", Quantity: 1},
		},
	}

	customer := createTestCustomer("customer-123")

	mockProductRepo.EXPECT().FindByID(gomock.Any(), "non-existent-product").Return(nil, sql.ErrNoRows)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX).AnyTimes()
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX).AnyTimes()
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX).AnyTimes()
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX).AnyTimes()
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil).AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp["code"])
}

func TestOrderHandler_CreateOrder_InsufficientStock(t *testing.T) {
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

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	reqBody := orderdto.OrderRequestDto{
		CustomerID: "customer-123",
		Items: []*orderitemdto.OrderItemRequestDto{
			{ProductID: "product-123", Quantity: 10},
		},
	}

	customer := createTestCustomer("customer-123")
	product := createTestProduct("product-123", 5)

	mockProductRepo.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoWithTX).AnyTimes()
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoWithTX).AnyTimes()
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoWithTX).AnyTimes()
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX).AnyTimes()
	mockCustomerRepoWithTX.EXPECT().FindByCustomerID(gomock.Any(), "customer-123").Return(customer, nil).AnyTimes()
	mockProductRepoWithTX.EXPECT().FindByID(gomock.Any(), "product-123").Return(product, nil).AnyTimes()
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_OUT_OF_STOCK", resp["code"])
}

func TestOrderHandler_GetOrderById_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	order := createTestOrder("order-123", "customer-123", 2000)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindByID(gomock.Any(), "order-123").Return(order, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	req := httptest.NewRequest("GET", "/orders/order-123", nil)
	req = setupChiContext(req, "order-123")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "order-123", resp["id"])
	assert.Equal(t, float64(2000), resp["total_cents"])
	assert.Equal(t, "PENDING", resp["status"])
}

func TestOrderHandler_GetOrderById_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindByID(gomock.Any(), "non-existent-order").Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil)

	req := httptest.NewRequest("GET", "/orders/non-existent-order", nil)
	req = setupChiContext(req, "non-existent-order")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_NOT_FOUND", resp["code"])
}

func TestOrderHandler_GetOrderById_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	req := httptest.NewRequest("GET", "/orders/", nil)
	req = setupChiContext(req, "")
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetOrderByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", resp["code"])
}

func TestOrderHandler_GetAllOrders_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	orders := []*entity.Order{
		createTestOrder("order-1", "customer-1", 1000),
		createTestOrder("order-2", "customer-2", 2000),
	}

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindAll(gomock.Any()).Return(orders, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	req := httptest.NewRequest("GET", "/orders", nil)
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "order-1", resp[0]["id"])
	assert.Equal(t, "order-2", resp[1]["id"])
}

func TestOrderHandler_GetAllOrders_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxMgr := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockOrderRepo := repository.NewMockOrderRepository(ctrl)
	mockOrderRepoWithTX := repository.NewMockOrderRepository(ctrl)
	mockProductRepo := repository.NewMockProductRepository(ctrl)
	mockCustomerRepo := repository.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := repository.NewMockOrderItemRepository(ctrl)

	var db *sql.DB

	handler := orderhandler.NewOrderHandler(
		db,
		mockTxMgr,
		mockOrderRepo,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		nil,
	)

	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoWithTX)
	mockOrderRepoWithTX.EXPECT().FindAll(gomock.Any()).Return([]*entity.Order{}, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	req := httptest.NewRequest("GET", "/orders", nil)
	req = setupTenantContext(req) // 🆕 PR-8 FIX
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllOrderHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}
