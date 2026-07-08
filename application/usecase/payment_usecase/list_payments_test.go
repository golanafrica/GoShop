package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - LIST PAYMENTS USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Multi-tenant validation
//   - Repository errors
//   - Happy paths avec filtres
//   - Mapping DTOs
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestContextForPayment crée un contexte tenant pour les tests payment
func createTestContextForPayment() context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	return tenant.WithTenant(ctx, shop)
}

// createTestPayment crée un paiement de test
func createTestPayment(shopID uuid.UUID, orderID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, amountCents)
	return payment
}

// ============================================================
// TESTS : ListPaymentsUsecase - Multi-tenant
// ============================================================

func TestListPaymentsUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	// Contexte SANS tenant
	ctx := context.Background()
	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : ListPaymentsUsecase - Repository errors
// ============================================================

func TestListPaymentsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Mock : FindByShop échoue
	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, errors.New("database error"))

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find payments")
}

// ============================================================
// TESTS : ListPaymentsUsecase - Happy paths
// ============================================================

func TestListPaymentsUsecase_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Liste vide
	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Any()).
		Return([]*entity.Payment{}, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Empty(t, response)
}

func TestListPaymentsUsecase_SuccessWithMultiplePayments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer 3 paiements de test
	payment1 := createTestPayment(shop.ID, uuid.New(), 50000)
	payment2 := createTestPayment(shop.ID, uuid.New(), 75000)
	payment3 := createTestPayment(shop.ID, uuid.New(), 100000)

	payments := []*entity.Payment{payment1, payment2, payment3}

	// Mock : Liste avec 3 paiements
	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Any()).
		Return(payments, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 3)

	// Vérifier le mapping DTO
	assert.Equal(t, payment1.ID.String(), response[0].ID)
	assert.Equal(t, payment1.OrderID.String(), response[0].OrderID)
	assert.Equal(t, int64(50000), response[0].AmountCents)
	assert.Equal(t, entity.CurrencyXOF, response[0].Currency)
	assert.Equal(t, entity.PaymentStatusPending, response[0].Status)
}

func TestListPaymentsUsecase_WithStatusFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement avec statut SUCCESS
	successPayment := createTestPayment(shop.ID, uuid.New(), 50000)
	successPayment.MarkProcessing()
	successPayment.MarkSuccess("TXN-123")

	payments := []*entity.Payment{successPayment}

	// Mock : Filtre par statut
	statusFilter := entity.PaymentStatusSuccess
	expectedFilters := repository.PaymentFilters{
		Status: &statusFilter,
		Limit:  10,
		Offset: 0,
	}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Eq(expectedFilters)).
		Return(payments, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Status: &statusFilter,
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 1)
	assert.Equal(t, entity.PaymentStatusSuccess, response[0].Status)
}

func TestListPaymentsUsecase_WithProviderFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement YengaPay
	yengaPayment := createTestPayment(shop.ID, uuid.New(), 50000)

	payments := []*entity.Payment{yengaPayment}

	// Mock : Filtre par provider
	providerFilter := entity.ProviderYengaPay
	expectedFilters := repository.PaymentFilters{
		Provider: &providerFilter,
		Limit:    10,
		Offset:   0,
	}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Eq(expectedFilters)).
		Return(payments, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Provider: &providerFilter,
		Limit:    10,
		Offset:   0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 1)
	assert.Equal(t, entity.ProviderYengaPay, response[0].Provider)
}

func TestListPaymentsUsecase_WithPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Pagination (page 2, 10 par page)
	expectedFilters := repository.PaymentFilters{
		Limit:  10,
		Offset: 10,
	}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Eq(expectedFilters)).
		Return([]*entity.Payment{}, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 10, // Page 2
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

func TestListPaymentsUsecase_WithAllFilters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Tous les filtres
	statusFilter := entity.PaymentStatusSuccess
	providerFilter := entity.ProviderYengaPay
	expectedFilters := repository.PaymentFilters{
		Status:   &statusFilter,
		Provider: &providerFilter,
		Limit:    20,
		Offset:   40,
	}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Eq(expectedFilters)).
		Return([]*entity.Payment{}, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Status:   &statusFilter,
		Provider: &providerFilter,
		Limit:    20,
		Offset:   40,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

// ============================================================
// TESTS : ListPaymentsUsecase - DTO Mapping
// ============================================================

func TestListPaymentsUsecase_DTO_MappingComplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement avec tous les champs
	payment := createTestPayment(shop.ID, uuid.New(), 50000)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")

	phone := "+22670123456"
	payment.CustomerPhone = &phone

	description := "Payment for order"
	payment.Description = &description

	payments := []*entity.Payment{payment}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Any()).
		Return(payments, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 1)

	// Vérifier tous les champs du DTO
	dto := response[0]
	assert.Equal(t, payment.ID.String(), dto.ID)
	assert.Equal(t, payment.OrderID.String(), dto.OrderID)
	assert.Equal(t, entity.ProviderYengaPay, dto.Provider)
	assert.Equal(t, "TXN-123", dto.ProviderRef)
	assert.Equal(t, int64(50000), dto.AmountCents)
	assert.Equal(t, entity.CurrencyXOF, dto.Currency)
	assert.Equal(t, entity.PaymentStatusSuccess, dto.Status)
	assert.Equal(t, "+22670123456", dto.CustomerPhone)
	assert.Equal(t, "Payment for order", dto.Description)
	assert.NotEmpty(t, dto.CreatedAt)
	assert.NotEmpty(t, dto.CompletedAt)
}

func TestListPaymentsUsecase_DTO_WithOptionalFieldsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)

	uc := paymentusecase.NewListPaymentsUsecase(mockPaymentRepo)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement minimal (sans optional fields)
	payment := createTestPayment(shop.ID, uuid.New(), 50000)
	// Ne pas appeler MarkProcessing() ni MarkSuccess()
	// CustomerPhone et Description restent nil

	payments := []*entity.Payment{payment}

	mockPaymentRepo.EXPECT().
		FindByShop(gomock.Any(), shop.ID, gomock.Any()).
		Return(payments, nil)

	req := &paymentusecase.ListPaymentsRequest{
		Limit:  10,
		Offset: 0,
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 1)

	dto := response[0]
	assert.Empty(t, dto.ProviderRef) // nil → non défini
	assert.Empty(t, dto.CustomerPhone)
	assert.Empty(t, dto.Description)
	assert.NotEmpty(t, dto.CreatedAt)
	assert.Empty(t, dto.CompletedAt) // nil → non défini
}
