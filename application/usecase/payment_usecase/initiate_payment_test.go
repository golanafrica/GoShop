package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - INITIATE PAYMENT USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createTestOrderWithItems(orderID string, totalCents int64) *entity.Order {
	return &entity.Order{
		ID:            orderID,
		TotalCents:    totalCents,
		PaymentMethod: "mobile_money",
		Status:        "pending",
		Items: []*entity.OrderItem{
			{
				ID:         uuid.New().String(),
				PriceCents: totalCents,
				Quantity:   1,
			},
		},
	}
}

func createValidInitiateRequest(orderID string) *paymentdto.InitiatePaymentRequest {
	return &paymentdto.InitiatePaymentRequest{
		OrderID:     orderID,
		Provider:    entity.ProviderYengaPay,
		PhoneNumber: "+22670123456",
		CallbackURL: "https://example.com/callback",
	}
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Multi-tenant
// ============================================================

func TestInitiatePaymentUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil, // shopPaymentRepo
		mockEscrowRepo,
	)

	ctx := context.Background()
	req := createValidInitiateRequest(uuid.New().String())

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Validation
// ============================================================

func TestInitiatePaymentUsecase_InvalidOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	req := &paymentdto.InitiatePaymentRequest{
		OrderID:     "invalid-uuid",
		Provider:    entity.ProviderYengaPay,
		PhoneNumber: "+22670123456",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid order_id")
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Repository errors
// ============================================================

func TestInitiatePaymentUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(nil, errors.New("order not found"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order not found")
}

func TestInitiatePaymentUsecase_ExistingActivePayment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	existingPayment, _ := entity.NewPayment(uuid.New(), orderUUID, entity.ProviderYengaPay, 50000)
	existingPayment.MarkProcessing()

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{existingPayment}, nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order already has an active payment")
}

func TestInitiatePaymentUsecase_OrderTotalZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	zeroOrder := &entity.Order{
		ID:     orderID,
		Status: "pending",
		Items:  []*entity.OrderItem{},
	}

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(zeroOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order total must be positive")
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Provider errors
// ============================================================

func TestInitiatePaymentUsecase_ProviderNotAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(nil, errors.New("provider not available"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "provider not available")
}

func TestInitiatePaymentUsecase_ProviderInitiationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("insufficient funds"))

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "initiate payment with provider")
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Happy paths
// ============================================================

func TestInitiatePaymentUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      entity.PaymentStatusProcessing,
			USSDCode:    "*123#",
			Metadata: map[string]interface{}{
				"notification": "Dial *123# to complete payment",
			},
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-123", response.ProviderRef)
	assert.Equal(t, entity.PaymentStatusProcessing, response.Status)
	assert.Equal(t, "*123#", response.USSDCode)
	assert.NotEmpty(t, response.PaymentID)
	assert.NotZero(t, response.ExpiresAt)
}

func TestInitiatePaymentUsecase_Success_WithCustomerEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-456",
			Status:      entity.PaymentStatusProcessing,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	req.CustomerEmail = "customer@example.com"
	req.Description = "Payment for order"

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-456", response.ProviderRef)
}

func TestInitiatePaymentUsecase_Success_WithMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-789",
			Status:      entity.PaymentStatusProcessing,
			Metadata: map[string]interface{}{
				"otp":          "123456",
				"notification": "Enter OTP to complete",
			},
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	req.Metadata = map[string]interface{}{
		"custom_field": "custom_value",
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-789", response.ProviderRef)
	assert.NotNil(t, response.Metadata)
}

// ============================================================
// TESTS : InitiatePaymentUsecase - Zones non couvertes
// ============================================================

func TestInitiatePaymentUsecase_CheckExistingPaymentsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return(nil, errors.New("database error"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "check existing payments")
}

func TestInitiatePaymentUsecase_SavePaymentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "save payment")
}

func TestInitiatePaymentUsecase_UpdatePaymentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      entity.PaymentStatusProcessing,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update payment")
}

func TestInitiatePaymentUsecase_EmptyProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "",
			Status:      entity.PaymentStatusPending,
			USSDCode:    "*123#",
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Empty(t, response.ProviderRef)
	assert.Equal(t, entity.PaymentStatusPending, response.Status)
}

// ============================================================
// TEST : InitiatePaymentUsecase - Constructor with ShopSettings (using nil)
// ============================================================

func TestInitiatePaymentUsecase_WithNilShopPaymentSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil, // shopPaymentRepo = nil
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-SHOP",
			Status:      entity.PaymentStatusProcessing,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-SHOP", response.ProviderRef)
}

func TestInitiatePaymentUsecase_DatabaseFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		nil,
		mockEscrowRepo,
	)

	ctx := createTestContextForPayment()
	orderID := uuid.New().String()
	orderUUID, _ := uuid.Parse(orderID)

	testOrder := createTestOrderWithItems(orderID, 50000)

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), orderID).
		Return(testOrder, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{}, nil)

	mockPaymentRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		InitiatePayment(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      entity.PaymentStatusProcessing,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(errors.New("connection lost"))

	req := createValidInitiateRequest(orderID)
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update payment")
}
