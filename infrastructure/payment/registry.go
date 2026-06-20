package payment

import (
	"context"
	"fmt"
	"sync"

	"Goshop/domain/entity"
)

// Registry est le registre des providers de paiement
type Registry struct {
	mu        sync.RWMutex
	providers map[entity.PaymentProvider]Provider
}

// NewRegistry crée un nouveau registre
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[entity.PaymentProvider]Provider),
	}
}

// Register enregistre un provider
func (r *Registry) Register(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	code := provider.Code()
	if _, exists := r.providers[code]; exists {
		return fmt.Errorf("provider %s already registered", code)
	}

	r.providers[code] = provider
	return nil
}

// Get récupère un provider par son code
func (r *Registry) Get(code entity.PaymentProvider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[code]
	if !exists {
		return nil, fmt.Errorf("provider %s not registered", code)
	}

	return provider, nil
}

// GetAvailable récupère un provider disponible
func (r *Registry) GetAvailable(ctx context.Context, code entity.PaymentProvider) (Provider, error) {
	provider, err := r.Get(code)
	if err != nil {
		return nil, err
	}

	if !provider.IsAvailable(ctx) {
		return nil, fmt.Errorf("provider %s is not available", code)
	}

	return provider, nil
}

// List retourne tous les providers enregistrés
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// ListAvailable retourne les providers disponibles
func (r *Registry) ListAvailable(ctx context.Context) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0)
	for _, p := range r.providers {
		if p.IsAvailable(ctx) {
			providers = append(providers, p)
		}
	}
	return providers
}

// Has vérifie si un provider est enregistré
func (r *Registry) Has(code entity.PaymentProvider) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.providers[code]
	return exists
}
