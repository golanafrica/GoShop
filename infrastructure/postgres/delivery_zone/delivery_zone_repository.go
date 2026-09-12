package deliveryzone

import (
	"context"
	"database/sql"
	"errors"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type deliveryZoneRepository struct {
	db *sql.DB
}

func NewDeliveryZoneRepository(db *sql.DB) repository.DeliveryZoneRepository {
	return &deliveryZoneRepository{db: db}
}

func (r *deliveryZoneRepository) Create(ctx context.Context, zone *entity.DeliveryZone) error {
	query := `
		INSERT INTO delivery_zones (
			zone_code, zone_name, country, region, zone_type,
			delivery_delay_days, return_delay_days, warranty_response_days,
			cod_confirmation_delay_days, installment_release_delay_days,
			description, is_active, priority, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRowContext(ctx, query,
		zone.ZoneCode, zone.ZoneName, zone.Country, zone.Region, zone.ZoneType,
		zone.DeliveryDelayDays, zone.ReturnDelayDays, zone.WarrantyResponseDays,
		zone.CODConfirmationDelayDays, zone.InstallmentReleaseDelayDays,
		zone.Description, zone.IsActive, zone.Priority, zone.CreatedBy, zone.UpdatedBy,
	).Scan(&zone.ID, &zone.CreatedAt, &zone.UpdatedAt)
}

func (r *deliveryZoneRepository) Update(ctx context.Context, zone *entity.DeliveryZone) error {
	query := `
		UPDATE delivery_zones SET
			zone_name = $1, region = $2, delivery_delay_days = $3, return_delay_days = $4,
			warranty_response_days = $5, cod_confirmation_delay_days = $6,
			installment_release_delay_days = $7, description = $8, is_active = $9,
			priority = $10, updated_by = $11, updated_at = NOW()
		WHERE id = $12
		RETURNING updated_at
	`
	return r.db.QueryRowContext(ctx, query,
		zone.ZoneName, zone.Region, zone.DeliveryDelayDays, zone.ReturnDelayDays,
		zone.WarrantyResponseDays, zone.CODConfirmationDelayDays, zone.InstallmentReleaseDelayDays,
		zone.Description, zone.IsActive, zone.Priority, zone.UpdatedBy, zone.ID,
	).Scan(&zone.UpdatedAt)
}

func (r *deliveryZoneRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM delivery_zones WHERE id = $1", id)
	return err
}

// 🆕 CORRECTION MAJEURE : Utilisation de COALESCE pour éviter les erreurs de scan NULL
// sur les champs optionnels (region, description, created_by, updated_by)
const baseSelectQuery = `
	SELECT 
		id, zone_code, zone_name, country, COALESCE(region, ''), zone_type, 
		delivery_delay_days, return_delay_days, warranty_response_days, 
		cod_confirmation_delay_days, installment_release_delay_days, 
		COALESCE(description, ''), is_active, priority, 
		COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at 
	FROM delivery_zones
`

func (r *deliveryZoneRepository) FindByID(ctx context.Context, id string) (*entity.DeliveryZone, error) {
	return r.queryZone(ctx, baseSelectQuery+" WHERE id = $1", id)
}

func (r *deliveryZoneRepository) FindByCode(ctx context.Context, code string) (*entity.DeliveryZone, error) {
	return r.queryZone(ctx, baseSelectQuery+" WHERE zone_code = $1", code)
}

func (r *deliveryZoneRepository) ListByCountry(ctx context.Context, country string) ([]*entity.DeliveryZone, error) {
	return r.queryZones(ctx, baseSelectQuery+" WHERE country = $1 ORDER BY priority DESC", country)
}

func (r *deliveryZoneRepository) ListByType(ctx context.Context, zoneType entity.ZoneType) ([]*entity.DeliveryZone, error) {
	return r.queryZones(ctx, baseSelectQuery+" WHERE zone_type = $1 ORDER BY priority DESC", zoneType)
}

func (r *deliveryZoneRepository) ListActive(ctx context.Context) ([]*entity.DeliveryZone, error) {
	return r.queryZones(ctx, baseSelectQuery+" WHERE is_active = true ORDER BY country, priority DESC")
}

// Helpers
func (r *deliveryZoneRepository) queryZone(ctx context.Context, query string, args ...interface{}) (*entity.DeliveryZone, error) {
	zone := &entity.DeliveryZone{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&zone.ID, &zone.ZoneCode, &zone.ZoneName, &zone.Country, &zone.Region, &zone.ZoneType,
		&zone.DeliveryDelayDays, &zone.ReturnDelayDays, &zone.WarrantyResponseDays,
		&zone.CODConfirmationDelayDays, &zone.InstallmentReleaseDelayDays,
		&zone.Description, &zone.IsActive, &zone.Priority, &zone.CreatedBy, &zone.UpdatedBy,
		&zone.CreatedAt, &zone.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("zone non trouvée")
	}
	return zone, err
}

func (r *deliveryZoneRepository) queryZones(ctx context.Context, query string, args ...interface{}) ([]*entity.DeliveryZone, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []*entity.DeliveryZone
	for rows.Next() {
		zone := &entity.DeliveryZone{}
		err := rows.Scan(
			&zone.ID, &zone.ZoneCode, &zone.ZoneName, &zone.Country, &zone.Region, &zone.ZoneType,
			&zone.DeliveryDelayDays, &zone.ReturnDelayDays, &zone.WarrantyResponseDays,
			&zone.CODConfirmationDelayDays, &zone.InstallmentReleaseDelayDays,
			&zone.Description, &zone.IsActive, &zone.Priority, &zone.CreatedBy, &zone.UpdatedBy,
			&zone.CreatedAt, &zone.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}
