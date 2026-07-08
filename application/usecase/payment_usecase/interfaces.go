package paymentusecase

import (
	"context"

	"Goshop/domain/entity"
	"Goshop/infrastructure/payment"
)

//go:generate mockgen -destination=../../../mocks/usecase/mock_payment_registry.go -package=usecase . PaymentRegistry
//go:generate mockgen -destination=../../../mocks/usecase/mock_payment_provider.go -package=usecase Goshop/infrastructure/payment Provider
//go:generate mockgen -destination=../../../mocks/usecase/mock_provider_completable.go -package=usecase Goshop/infrastructure/payment ProviderCompletable
//go:generate mockgen -destination=../../../mocks/usecase/mock_provider.go -package=usecase Goshop/infrastructure/payment Provider

// ============================================================
// 🆕 v4.4.9 : INTERFACES POUR TESTABILITÉ
// ============================================================
//
// 🎯 Objectif :
//   Permettre le mocking de payment.Registry dans les tests unitaires.
//   payment.Registry est un struct qui ne peut pas être mocké directement.
//
// 📋 Principe :
//   - Interface définie dans le package paymentusecase
//   - Implémentation : *payment.Registry (struct existant)
//   - Injection de dépendance via le constructeur
//
// 🔧 Refactoring :
//   - Avant : registry *payment.Registry (struct)
//   - Après : registry PaymentRegistry (interface)
//
// ============================================================

// PaymentRegistry définit l'interface pour accéder aux providers de paiement
// Implémenté par *payment.Registry
type PaymentRegistry interface {
	// Get récupère un provider par son code
	// Retourne une erreur si le provider n'est pas enregistré
	Get(providerCode entity.PaymentProvider) (payment.Provider, error)

	// GetAvailable récupère un provider disponible pour le contexte donné
	// Vérifie que le provider est actif et configuré
	GetAvailable(ctx context.Context, providerCode entity.PaymentProvider) (payment.Provider, error)
}
