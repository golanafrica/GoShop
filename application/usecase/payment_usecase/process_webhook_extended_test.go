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
// 🆕 v4.4.24 : TESTS COMPLÉMENTAIRES - PROCESS WEBHOOK
// ============================================================

// NOTE : mockResult est déjà déclaré dans process_webhook_test.go,
// nous l'utilisons donc directement ici sans le redéclarer.

func TestProcessWebhookUsecase_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	registry := mockusecase.NewMockPaymentRegistry(ctrl)
	db := mockrepo.NewMockDBExecutor(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	provider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(paymentRepo, registry, db, shopRepo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	registry.EXPECT().Get(entity.ProviderOrangeMoney).Return(provider, nil)
	provider.EXPECT().ValidateWebhook(ctx, []byte("{}"), "sig").Return(nil, errors.New("invalid signature"))

	db.EXPECT().ExecContext(
		ctx,
		gomock.Any(),        // query
		gomock.Any(),        // provider
		gomock.Any(),        // event_type
		gomock.Any(),        // external_id
		gomock.Any(),        // payload
		gomock.Any(),        // signature
		false,               // signature_validated
		"invalid signature", // processing_error
		false,               // processed
	).Return(&mockResult{rowsAffected: 1}, nil)

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

	uc := paymentusecase.NewProcessWebhookUsecase(paymentRepo, registry, db, shopRepo, nil, nil, nil, nil, nil)
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

	db.EXPECT().ExecContext(
		ctx,
		gomock.Any(), // query
		gomock.Any(), // provider
		gomock.Any(), // event_type
		gomock.Any(), // external_id
		gomock.Any(), // payload
		gomock.Any(), // signature
		true,         // signature_validated
		"",           // processing_error
		true,         // processed
	).Return(&mockResult{rowsAffected: 1}, nil)

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

	uc := paymentusecase.NewProcessWebhookUsecase(paymentRepo, registry, db, shopRepo, nil, nil, nil, nil, nil)
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

	db.EXPECT().ExecContext(
		gomock.Any(), // ctx
		gomock.Any(), // query
		gomock.Any(), // provider
		gomock.Any(), // event_type
		gomock.Any(), // external_id
		gomock.Any(), // payload
		gomock.Any(), // signature
		true,         // signature_validated
		"",           // processing_error
		true,         // processed
	).Return(&mockResult{rowsAffected: 1}, nil)

	paymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderOrangeMoney, "prov-123").
		Return(paymentEntity, nil)

	shopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(shop, nil)

	// ✅ AJOUT CRITIQUE : Le usecase appelle Update pour mettre à jour le flag escrow_created
	// DOIT être déclaré AVANT l'appel à uc.Execute
	paymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	err := uc.Execute(ctx, entity.ProviderOrangeMoney, []byte("{}"), "sig")
	assert.NoError(t, err) // Retourne nil silencieusement car déjà terminal et escrow géré
}
