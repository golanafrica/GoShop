package withdrawalusecase

import (
	"context"

	"Goshop/domain/entity"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid" // ✅ AJOUTER CET IMPORT
)

// ============================================================
// 🆕 v4.4.10 : INTERFACES POUR TESTABILITÉ
// ============================================================
//
// 🎯 Note : On réutilise le mock MockPaymentRegistry de payment_usecase
// Les deux interfaces sont identiques (même signature)
//
// ============================================================

// PaymentRegistry définit l'interface pour accéder aux providers de paiement
// Implémenté par *payment.Registry
type PaymentRegistry interface {
	Get(providerCode entity.PaymentProvider) (payment.Provider, error)
	GetAvailable(ctx context.Context, providerCode entity.PaymentProvider) (payment.Provider, error)
}

// ============================================================
// 🆕 v4.4.10 : CASH-OUT PROVIDER POUR TESTABILITÉ
// ============================================================

//go:generate mockgen -destination=../../../mocks/usecase/mock_cashout_provider.go -package=usecase . CashOutProvider

// CashOutProvider définit l'interface pour effectuer un retrait
// Implémenté par *payment.YengaPayProvider
type CashOutProvider interface {
	CashOut(ctx context.Context, req *payment.CashOutRequest) (*payment.CashOutResponse, error)
}

// YengaPayProviderFactory est une factory pour créer un provider CashOut
// Permet de mocker payment.NewYengaPayProvider dans les tests
type YengaPayProviderFactory func(config payment.YengaPayConfig) (CashOutProvider, error)

// ============================================================
// 🆕 v4.4.10 : SHOP PAYMENT SETTINGS REPOSITORY
// ============================================================

//go:generate mockgen -destination=../../../mocks/usecase/mock_shop_payment_settings.go -package=usecase . ShopPaymentSettingsRepository

// ShopPaymentSettingsRepository définit l'interface pour récupérer la config boutique
// Implémenté par l'infrastructure repository
type ShopPaymentSettingsRepository interface {
	GetPaymentSettings(ctx context.Context, shopID uuid.UUID) (*entity.ShopPaymentSettings, error)
}
