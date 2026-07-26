package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.24 : TESTS COMPLÉMENTAIRES - INITIATE PAYMENT
// ============================================================

func createTestShopPayment() *entity.Shop {
	return &entity.Shop{
		ID:      uuid.New(),
		OwnerID: "owner-123",
		Slug:    "test-shop",
	}
}

func TestInitiatePaymentUsecase_ProviderInitiateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(paymentRepo, orderRepo, registry, nil, escrowRepo)

	shop := createTestShopPayment()
	ctx := tenant.WithTenant(context.Background(), shop)
	orderID := uuid.New().String()

	req := &paymentdto.InitiatePaymentRequest{
		OrderID:  orderID,
		Provider: entity.ProviderOrangeMoney,
	}

	order := &entity.Order{
		ID: orderID,
		Items: []*entity.OrderItem{
			{PriceCents: 1000, Quantity: 1},
		},
	}

	orderUUID := uuid.MustParse(orderID)
	orderRepo.EXPECT().FindByID(ctx, orderID).Return(order, nil)
	paymentRepo.EXPECT().FindByOrderID(ctx, orderUUID).Return([]*entity.Payment{}, nil)
	paymentRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

	registry.EXPECT().GetAvailable(ctx, entity.ProviderOrangeMoney).Return(provider, nil)

	provider.EXPECT().InitiatePayment(ctx, gomock.Any()).Return(nil, errors.New("provider error"))

	paymentRepo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "initiate payment with provider")
}

func TestInitiatePaymentUsecase_OrderAlreadyHasActivePayment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)

	uc := paymentusecase.NewInitiatePaymentUsecase(paymentRepo, orderRepo, registry, nil, escrowRepo)

	shop := createTestShopPayment()
	ctx := tenant.WithTenant(context.Background(), shop)
	orderID := uuid.New().String()

	req := &paymentdto.InitiatePaymentRequest{
		OrderID:  orderID,
		Provider: entity.ProviderOrangeMoney,
	}

	order := &entity.Order{
		ID: orderID,
		Items: []*entity.OrderItem{
			{PriceCents: 1000, Quantity: 1},
		},
	}

	orderUUID := uuid.MustParse(orderID)
	orderRepo.EXPECT().FindByID(ctx, orderID).Return(order, nil)

	existingPayment := &entity.Payment{
		ID:     uuid.New(),
		Status: entity.PaymentStatusSuccess,
	}
	paymentRepo.EXPECT().FindByOrderID(ctx, orderUUID).Return([]*entity.Payment{existingPayment}, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "order already has an active payment")
}
