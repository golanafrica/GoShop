package orderusecase_test

import (
	"context"
	"errors"
	"testing"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCreateOrderUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
		CustomerID: "cust-1",
		TotalCents: 100000,
		Status:     "PENDING",
		Items:      order.Items,
	}

	// BeginTx OK
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)

	// WithTX attach
	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx).Times(1)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx).Times(1)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx).Times(1)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx).Times(1)

	// Customer exists
	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").
		Return(customer, nil).Times(1)

	// Product found
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").
		Return(product, nil).Times(1)

	// Stock update OK
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil).Times(1)

	// Create order OK
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *entity.Order) (*entity.Order, error) {
			return createdOrder, nil
		}).Times(1)

	// Create order item OK
	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).
		Return(&entity.OrderItem{}, nil).Times(1)

	// Commit OK
	mockTx.EXPECT().Commit().Return(nil).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(

		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	result, err := uc.Execute(context.Background(), order)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "order-123", result.ID)
}

// -----------------------------
//
//	CLIENT NOT FOUND
//
// -----------------------------
func TestCreateOrderUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	order := &entity.Order{CustomerID: "cust-404"}

	// BeginTx OK
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// Repo TX versions
	mockProductRepoTx := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderRepoTx := mockrepo.NewMockOrderRepository(ctrl)
	mockOrderItemRepoTx := mockrepo.NewMockOrderItemRepository(ctrl)

	mockProductRepo.EXPECT().WithTX(mockTx).Return(mockProductRepoTx)
	mockCustomerRepo.EXPECT().WithTX(mockTx).Return(mockCustomerRepoTx)
	mockOrderRepo.EXPECT().WithTX(mockTx).Return(mockOrderRepoTx)
	mockOrderItemRepo.EXPECT().WithTX(mockTx).Return(mockOrderItemRepoTx)

	// Customer not found
	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-404").
		Return(nil, errors.New("not found"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,
		mockProductRepo,
		mockCustomerRepo,
		mockOrderItemRepo,
		mockOrderRepo,
	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	// CORRECTION : Utiliser le message EXACT du code
	assert.Contains(t, err.Error(), "failed to retrieve customer: not found")
}

// -----------------------------
//
//	PRODUCT NOT FOUND
//
// -----------------------------
func TestCreateOrderUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	uc := orderusecase.NewCreateOrderUsecase( // 1. db (remplace $1 par nil)
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// -----------------------------
//
//	STOCK INSUFFICIENT
//
// -----------------------------
func TestCreateOrderUsecase_InsufficientStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
		// 1. db (remplace $1 par nil)
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough stock")
}

// -----------------------------
//
//	ERROR UPDATE PRODUCT
//
// -----------------------------
func TestCreateOrderUsecase_UpdateStockError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
		// 1. db (remplace $1 par nil)
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update stock")
}

// -----------------------------
//
//	ERROR CREATE ORDER
//
// -----------------------------
func TestCreateOrderUsecase_CreateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	// Customer OK
	mockCustomerRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust").Return(&entity.Customer{}, nil)

	// Product OK
	mockProductRepoTx.EXPECT().FindByID(gomock.Any(), "prod-1").Return(product, nil)

	// Update stock OK
	mockProductRepoTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	// âŒ CREATE ORDER FAILS
	mockOrderRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("order creation failed"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase( // 1. db (remplace $1 par nil)
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create order")
}

// -----------------------------
//
//	ERROR CREATE ORDER ITEM
//
// -----------------------------
func TestCreateOrderUsecase_OrderItemError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	// âŒ ORDER ITEM FAILS
	mockOrderItemRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("item error"))

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase( // 1. db (remplace $1 par nil)
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create order item")
}

// -----------------------------
//
//	BEGIN TX ERROR
//
// -----------------------------
func TestCreateOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	// AJOUTEZ CES LIGNES pour définir les mocks (même si vous ne les utilisez pas)
	mockProductRepo := mockrepo.NewMockProductRepository(ctrl)
	mockCustomerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockOrderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := orderusecase.NewCreateOrderUsecase( // 1. db
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), &entity.Order{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

// -----------------------------
//
//	COMMIT ERROR
//
// -----------------------------
func TestCreateOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	// âŒ COMMIT FAILS
	mockTx.EXPECT().Commit().Return(errors.New("commit error"))
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := orderusecase.NewCreateOrderUsecase(
		mockTxManager,     // 2. txManager
		mockProductRepo,   // 3. productRepo
		mockCustomerRepo,  // 4. customerRepo
		mockOrderItemRepo, // 5. orderItemRepo
		mockOrderRepo,     // 6. orderRepo

	)

	_, err := uc.Execute(context.Background(), order)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}
