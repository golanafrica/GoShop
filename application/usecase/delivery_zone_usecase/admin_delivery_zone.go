package deliveryzoneusecase

import (
	"context"
	"errors"

	deliveryzonedto "Goshop/application/dto/delivery_zone_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
)

type AdminDeliveryZoneUsecase struct {
	repo    repository.DeliveryZoneRepository
	service service.DeliveryZoneService
}

func NewAdminDeliveryZoneUsecase(repo repository.DeliveryZoneRepository, svc service.DeliveryZoneService) *AdminDeliveryZoneUsecase {
	return &AdminDeliveryZoneUsecase{repo: repo, service: svc}
}

func (uc *AdminDeliveryZoneUsecase) CreateZone(ctx context.Context, req deliveryzonedto.CreateZoneRequest, adminID string) (*entity.DeliveryZone, error) {
	existing, _ := uc.repo.FindByCode(ctx, req.ZoneCode)
	if existing != nil {
		return nil, errors.New("zone_code existe déjà")
	}

	zone := &entity.DeliveryZone{
		ZoneCode:                    req.ZoneCode,
		ZoneName:                    req.ZoneName,
		Country:                     req.Country,
		Region:                      req.Region,
		ZoneType:                    entity.ZoneType(req.ZoneType),
		DeliveryDelayDays:           req.DeliveryDelayDays,
		ReturnDelayDays:             req.ReturnDelayDays,
		WarrantyResponseDays:        req.WarrantyResponseDays,
		CODConfirmationDelayDays:    req.CODConfirmationDelayDays,
		InstallmentReleaseDelayDays: req.InstallmentReleaseDelayDays,
		Description:                 req.Description,
		IsActive:                    true,
		Priority:                    req.Priority,
		CreatedBy:                   adminID,
		UpdatedBy:                   adminID,
	}

	if err := zone.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.Create(ctx, zone); err != nil {
		return nil, err
	}

	return zone, nil
}

func (uc *AdminDeliveryZoneUsecase) UpdateZone(ctx context.Context, id string, req deliveryzonedto.UpdateZoneRequest, adminID string) (*entity.DeliveryZone, error) {
	zone, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.ZoneName != "" {
		zone.ZoneName = req.ZoneName
	}
	if req.Region != "" {
		zone.Region = req.Region
	}
	if req.DeliveryDelayDays != nil {
		zone.DeliveryDelayDays = *req.DeliveryDelayDays
	}
	if req.ReturnDelayDays != nil {
		zone.ReturnDelayDays = *req.ReturnDelayDays
	}
	if req.WarrantyResponseDays != nil {
		zone.WarrantyResponseDays = *req.WarrantyResponseDays
	}
	if req.CODConfirmationDelayDays != nil {
		zone.CODConfirmationDelayDays = *req.CODConfirmationDelayDays
	}
	if req.InstallmentReleaseDelayDays != nil {
		zone.InstallmentReleaseDelayDays = *req.InstallmentReleaseDelayDays
	}
	if req.Description != "" {
		zone.Description = req.Description
	}
	if req.IsActive != nil {
		zone.IsActive = *req.IsActive
	}
	if req.Priority != nil {
		zone.Priority = *req.Priority
	}

	zone.UpdatedBy = adminID

	if err := uc.repo.Update(ctx, zone); err != nil {
		return nil, err
	}

	_ = uc.service.InvalidateCache(ctx, zone.ZoneCode)

	return zone, nil
}

func (uc *AdminDeliveryZoneUsecase) DeleteZone(ctx context.Context, id string) error {
	zone, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = uc.service.InvalidateCache(ctx, zone.ZoneCode)

	return nil
}

func (uc *AdminDeliveryZoneUsecase) ListZones(ctx context.Context) ([]*entity.DeliveryZone, error) {
	return uc.repo.ListActive(ctx)
}
