package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

type mockResult struct {
	lastInsertId int64
	rowsAffected int64
}

func (m *mockResult) LastInsertId() (int64, error) { return m.lastInsertId, nil }
func (m *mockResult) RowsAffected() (int64, error) { return m.rowsAffected, nil }

// expectDBExecAny configure le mock pour accepter n'importe quel appel à ExecContext
// (10 args pour l'INSERT de recordWebhook, 4 args pour markWebhookProcessed, 5 args pour markWebhookFailed)
func expectDBExecAny(db *mockrepo.MockDBExecutor) {
	// Match 10 args (recordWebhook INSERT)
	db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).AnyTimes()

	// Match 4 args (markWebhookProcessed UPDATE)
	db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).AnyTimes()

	// Match 5 args (markWebhookFailed UPDATE)
	db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).AnyTimes()
}

// ============================================================
// TESTS
// ============================================================

func TestProcessWebhookUsecase_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	db := mockrepo.NewMockDBExecutor(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo, registry, txManager, db, shopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	registry.EXPECT().Get(entity.ProviderOrangeMoney).Return(provider, nil)
	provider.EXPECT().ValidateWebhook(ctx, []byte("{}"), "sig").Return(nil, errors.New("invalid signature"))

	expectDBExecAny(db)

	err := uc.Execute(ctx, entity.ProviderOrangeMoney, []byte("{}"), "sig")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook validation failed")
}

func TestProcessWebhookUsecase_TontineWebhookNilHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	db := mockrepo.NewMockDBExecutor(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo, registry, txManager, db, shopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	event := &payment.WebhookEvent{
		EventType:   "payment.success",
		ExternalID:  "ext-123",
		ProviderRef: "prov-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata: map[string]interface{}{
			"reference": "TONTINE:grp-1:1:part-1",
		},
	}

	registry.EXPECT().Get(entity.ProviderOrangeMoney).Return(provider, nil)
	provider.EXPECT().ValidateWebhook(ctx, []byte("{}"), "sig").Return(event, nil)

	expectDBExecAny(db)

	err := uc.Execute(ctx, entity.ProviderOrangeMoney, []byte("{}"), "sig")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine webhook handler not configured")
}

func TestProcessWebhookUsecase_AlreadyTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	db := mockrepo.NewMockDBExecutor(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo, registry, txManager, db, shopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	shop := createTestShopPayment()
	ctx = tenant.WithTenant(ctx, shop)

	provRef := "prov-123"
	paymentEntity := &entity.Payment{
		ID:          uuid.New(),
		ShopID:      shop.ID,
		ProviderRef: &provRef,
		Status:      entity.PaymentStatusSuccess,
	}

	event := &payment.WebhookEvent{
		EventType:   "payment.success",
		ExternalID:  "ext-123",
		ProviderRef: "prov-123",
		Status:      entity.PaymentStatusSuccess,
	}

	registry.EXPECT().Get(entity.ProviderOrangeMoney).Return(provider, nil)
	provider.EXPECT().ValidateWebhook(ctx, []byte("{}"), "sig").Return(event, nil)

	expectDBExecAny(db)

	paymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderOrangeMoney, "prov-123").
		Return(paymentEntity, nil)

	shopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(shop, nil)

	paymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	err := uc.Execute(ctx, entity.ProviderOrangeMoney, []byte("{}"), "sig")
	assert.NoError(t, err)
}

func TestProcessWebhookUsecase_AlreadyTerminal_EscrowDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	db := mockrepo.NewMockDBExecutor(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		paymentRepo, registry, txManager, db, shopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	shop := createTestShopPayment()
	ctx = tenant.WithTenant(ctx, shop)

	provRef := "prov-123"
	paymentEntity := &entity.Payment{
		ID:          uuid.New(),
		ShopID:      shop.ID,
		ProviderRef: &provRef,
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{"escrow_created": true},
	}

	event := &payment.WebhookEvent{
		EventType:   "payment.success",
		ExternalID:  "ext-456",
		ProviderRef: "prov-123",
		Status:      entity.PaymentStatusSuccess,
	}

	registry.EXPECT().Get(entity.ProviderOrangeMoney).Return(provider, nil)
	provider.EXPECT().ValidateWebhook(ctx, []byte("{}"), "sig").Return(event, nil)

	expectDBExecAny(db)

	paymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderOrangeMoney, "prov-123").
		Return(paymentEntity, nil)

	shopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(shop, nil)

	err := uc.Execute(ctx, entity.ProviderOrangeMoney, []byte("{}"), "sig")
	assert.NoError(t, err)
}
