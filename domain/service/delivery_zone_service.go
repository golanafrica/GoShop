package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils" // Pour accéder à utils.Rdb
)

type DeliveryZoneService interface {
	GetZoneByCode(ctx context.Context, code string) (*entity.DeliveryZone, error)
	GetInstallmentReleaseDelay(ctx context.Context, zoneCode string) (int, error)
	GetDeliveryDelay(ctx context.Context, zoneCode string) (int, error)
	InvalidateCache(ctx context.Context, zoneCode string) error
}

type deliveryZoneService struct {
	repo repository.DeliveryZoneRepository
}

func NewDeliveryZoneService(repo repository.DeliveryZoneRepository) DeliveryZoneService {
	return &deliveryZoneService{
		repo: repo,
	}
}

// GetZoneByCode récupère une zone avec mise en cache Redis (TTL 24h)
func (s *deliveryZoneService) GetZoneByCode(ctx context.Context, code string) (*entity.DeliveryZone, error) {
	cacheKey := fmt.Sprintf("delivery_zone:code:%s", code)

	// 1. Essayer le cache Redis
	if utils.Rdb != nil {
		cached, err := utils.Rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			var zone entity.DeliveryZone
			if err := json.Unmarshal([]byte(cached), &zone); err == nil {
				return &zone, nil
			}
		}
	}

	// 2. Fallback sur la base de données
	zone, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// 3. Mettre en cache pour les prochaines requêtes (24 heures)
	if utils.Rdb != nil && zone != nil {
		data, _ := json.Marshal(zone)
		utils.Rdb.Set(ctx, cacheKey, data, 24*time.Hour)
	}

	return zone, nil
}

// GetInstallmentReleaseDelay retourne le délai de sécurité pour le paiement en tranches
func (s *deliveryZoneService) GetInstallmentReleaseDelay(ctx context.Context, zoneCode string) (int, error) {
	zone, err := s.GetZoneByCode(ctx, zoneCode)
	if err != nil {
		return 7, err // Valeur par défaut de secours
	}
	return zone.InstallmentReleaseDelayDays, nil
}

// GetDeliveryDelay retourne le délai de livraison standard
func (s *deliveryZoneService) GetDeliveryDelay(ctx context.Context, zoneCode string) (int, error) {
	zone, err := s.GetZoneByCode(ctx, zoneCode)
	if err != nil {
		return 7, err // Valeur par défaut de secours
	}
	return zone.DeliveryDelayDays, nil
}

// InvalidateCache supprime la zone du cache (à appeler après Update/Delete par l'admin)
func (s *deliveryZoneService) InvalidateCache(ctx context.Context, zoneCode string) error {
	if utils.Rdb != nil {
		cacheKey := fmt.Sprintf("delivery_zone:code:%s", zoneCode)
		return utils.Rdb.Del(ctx, cacheKey).Err()
	}
	return nil
}
