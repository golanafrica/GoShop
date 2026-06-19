package tenant

import (
	"context"
	"errors"

	"Goshop/domain/entity"
)

type contextKey string

const tenantKey contextKey = "tenant"

// ErrNoTenant est retourné quand aucun tenant n'est dans le contexte
var ErrNoTenant = errors.New("no tenant in context")

// WithTenant ajoute une boutique au contexte
func WithTenant(ctx context.Context, shop *entity.Shop) context.Context {
	return context.WithValue(ctx, tenantKey, shop)
}

// FromContext récupère la boutique depuis le contexte
func FromContext(ctx context.Context) (*entity.Shop, error) {
	shop, ok := ctx.Value(tenantKey).(*entity.Shop)
	if !ok || shop == nil {
		return nil, ErrNoTenant
	}
	return shop, nil
}

// MustFromContext récupère la boutique ou panic
func MustFromContext(ctx context.Context) *entity.Shop {
	shop, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return shop
}

// GetID récupère l'ID du tenant depuis le contexte
func GetID(ctx context.Context) (string, error) {
	shop, err := FromContext(ctx)
	if err != nil {
		return "", err
	}
	return shop.ID.String(), nil
}
