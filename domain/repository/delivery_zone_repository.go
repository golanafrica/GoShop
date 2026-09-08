package repository

import (
	"Goshop/domain/entity"
	"context"
)

type DeliveryZoneRepository interface {
	// Opérations de base
	Create(ctx context.Context, zone *entity.DeliveryZone) error
	Update(ctx context.Context, zone *entity.DeliveryZone) error
	Delete(ctx context.Context, id string) error

	// Requêtes de lecture
	FindByID(ctx context.Context, id string) (*entity.DeliveryZone, error)
	FindByCode(ctx context.Context, code string) (*entity.DeliveryZone, error)
	ListByCountry(ctx context.Context, country string) ([]*entity.DeliveryZone, error)
	ListByType(ctx context.Context, zoneType entity.ZoneType) ([]*entity.DeliveryZone, error)
	ListActive(ctx context.Context) ([]*entity.DeliveryZone, error)
}
