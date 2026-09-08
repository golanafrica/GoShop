package entity

import (
	"errors"
	"time"
)

type ZoneType string

const (
	ZoneTypeUrban         ZoneType = "urban"
	ZoneTypePeriUrban     ZoneType = "peri_urban"
	ZoneTypeRural         ZoneType = "rural"
	ZoneTypeRemote        ZoneType = "remote"
	ZoneTypeInternational ZoneType = "international"
)

var (
	ErrInvalidZoneType = errors.New("type de zone invalide")
	ErrInvalidDelay    = errors.New("délai hors des limites autorisées (1-30 jours)")
)

type DeliveryZone struct {
	ID                          string
	ZoneCode                    string
	ZoneName                    string
	Country                     string
	Region                      string
	ZoneType                    ZoneType
	DeliveryDelayDays           int
	ReturnDelayDays             int
	WarrantyResponseDays        int
	CODConfirmationDelayDays    int
	InstallmentReleaseDelayDays int
	Description                 string
	IsActive                    bool
	Priority                    int
	CreatedBy                   string
	UpdatedBy                   string
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// Validate vérifie les règles métier avant insertion/mise à jour
func (z *DeliveryZone) Validate() error {
	if z.ZoneCode == "" {
		return errors.New("zone_code est requis")
	}
	if z.ZoneName == "" {
		return errors.New("zone_name est requis")
	}
	if z.Country == "" {
		return errors.New("country est requis")
	}

	// Vérification basique des délais (la BDD a aussi des contraintes CHECK)
	if z.InstallmentReleaseDelayDays < 3 || z.InstallmentReleaseDelayDays > 30 {
		return ErrInvalidDelay
	}

	return nil
}
