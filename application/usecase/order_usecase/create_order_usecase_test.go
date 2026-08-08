package orderusecase_test

import (
	"context"
	"errors"
	"testing"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPER : crée un contexte avec tenant (fix B1)
// ============================================================
func contextWithTenant(t *testing.T) (context.Context, string) {
	t.Helper()
	shopID := uuid.New()
	shop := &entity.Shop{ID: shopID}
	ctx := tenant.WithTenant(context.Background(), shop)
	return ctx, shopID.String()
}

// ============================================================
// SUCCESS
// ============================================================
func TestCreateOrderUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, shopID := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust-1",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 2},
		},
	}

	product := &entity.Product{
		ID:         "prod-1",
		Name:       "Laptop",
		PriceCents: 50000,
		Stock:      10,
	}

	customer := &entity.Customer{
		ID:        "cust-1",
		FirstName: "John Doe",
	}

	createdOrder := &entity.Order{
		ID:         "order-123",
		ShopID:     shopID,
		CustomerID: "cust-1",
		TotalCents: 100000,
		Status:     "pending",
		Items:      order.Items,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx).Times(1)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx).Times(1)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx).Times(1)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx).Times(1)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").
		Return(customer, nil).Times(1)

	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").
		Return(product, nil).Times(1)

	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil).Times(1)

	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *entity.Order) (*entity.Order, error) {
			assert.Equal(t, shopID, o.ShopID, "ShopID doit être assigné depuis le tenant")
			return createdOrder, nil
		}).Times(1)

	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).
		Return(&entity.OrderItem{}, nil).Times(1)

	mockTx.EXPECT().Commit().Return(nil).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	result, err := uc.Execute(ctx, order)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "order-123", result.ID)
	assert.Equal(t, shopID, result.ShopID)
}

// ============================================================
// CLIENT NOT FOUND
// ============================================================
func TestCreateOrderUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{CustomerID: "cust-404"}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-404").
		Return(nil, errors.New("not found"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(mockTxManager, mockProductRepo, mockCustomerRepo, mockOrderItemRepo, mockOrderRepo, nil)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve customer: not found")
}

// ============================================================
// PRODUCT NOT FOUND
// ============================================================
func TestCreateOrderUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "p-404", Quantity: 1},
		},
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "p-404").Return(nil, errors.New("not found"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================
// STOCK INSUFFICIENT
// ============================================================
func TestCreateOrderUsecase_InsufficientStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 99},
		},
	}

	product := &entity.Product{
		ID:    "prod-1",
		Stock: 1,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough stock")
}

// ============================================================
// ERROR UPDATE PRODUCT
// ============================================================
func TestCreateOrderUsecase_UpdateStockError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 2},
		},
	}

	product := &entity.Product{
		ID:         "prod-1",
		PriceCents: 20000,
		Stock:      5,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil, errors.New("update error"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update stock")
}

// ============================================================
// ERROR CREATE ORDER
// ============================================================
func TestCreateOrderUsecase_CreateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 1},
		},
	}

	product := &entity.Product{
		ID:         "prod-1",
		PriceCents: 10000,
		Stock:      5,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("order creation failed"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create order")
}

// ============================================================
// ERROR CREATE ORDER ITEM
// ============================================================
func TestCreateOrderUsecase_OrderItemError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 1},
		},
	}

	product := &entity.Product{
		ID:         "prod-1",
		PriceCents: 10000,
		Stock:      5,
	}

	createdOrder := &entity.Order{ID: "order-1"}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(createdOrder, nil)
	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("item error"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create order item")
}

// ============================================================
// BEGIN TX ERROR
// ============================================================
func TestCreateOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, &entity.Order{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

// ============================================================
// COMMIT ERROR
// ============================================================
func TestCreateOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, _ := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{
		CustomerID: "cust",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 1},
		},
	}

	product := &entity.Product{
		ID:         "prod-1",
		PriceCents: 10000,
		Stock:      5,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.Order{ID: "o1"}, nil)
	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.OrderItem{}, nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit error"))
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	_, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// 🆕 v4.8.1 FIX B1 : ShopID assigné depuis tenant
// ============================================================
func TestCreateOrderUsecase_ShopIDFromTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, shopID := contextWithTenant(t)

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	customer := &entity.Customer{
		ID:        "cust-1",
		FirstName: "Test",
		LastName:  "User",
	}
	product := &entity.Product{
		ID:         "prod-1",
		Name:       "Test Product",
		PriceCents: 10000,
		Stock:      10,
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	var capturedOrder *entity.Order
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *entity.Order) (*entity.Order, error) {
			capturedOrder = o
			return &entity.Order{
				ID:            "order-1",
				ShopID:        shopID,
				CustomerID:    customer.ID,
				TotalCents:    10000,
				Status:        "pending",
				PaymentMethod: "mobile_money",
			}, nil
		})

	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.OrderItem{}, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil,
	)

	order := &entity.Order{
		CustomerID:    "cust-1",
		PaymentMethod: "mobile_money",
		Items: []*entity.OrderItem{
			{ProductID: "prod-1", Quantity: 1},
		},
	}

	createdOrder, err := uc.Execute(ctx, order)

	assert.NoError(t, err)
	assert.NotNil(t, createdOrder)
	assert.NotNil(t, capturedOrder, "Order should have been passed to Create")
	assert.Equal(t, shopID, capturedOrder.ShopID, "ShopID should be set BEFORE Create is called")
	assert.Equal(t, shopID, createdOrder.ShopID, "ShopID should be assigned from tenant")
}

// ============================================================
// 🆕 v4.8.1 FIX B1 : Sans tenant → erreur (AVANT Create, APRÈS WithTX)
// ============================================================
func TestCreateOrderUsecase_NoTenant_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Contexte SANS tenant
	ctx := context.Background()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	// L'erreur se produit APRÈS BeginTx + WithTX, mais AVANT FindByCustomerID
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// ✅ 4 mocks WithTX (codProofRepo est nil donc pas de 5e mock)
	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
		nil, // codProofRepo = nil → pas de 5e WithTX
	)

	order := &entity.Order{
		CustomerID:    "cust-1",
		PaymentMethod: "mobile_money",
		Items:         []*entity.OrderItem{},
	}

	createdOrder, err := uc.Execute(ctx, order)

	assert.Error(t, err)
	assert.Nil(t, createdOrder)
	assert.Contains(t, err.Error(), "multi-tenant")
}
