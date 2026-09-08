package deliveryzonedto

import "time"

// CreateZoneRequest représente la requête pour créer une zone
type CreateZoneRequest struct {
	ZoneCode                    string `json:"zone_code" validate:"required"`
	ZoneName                    string `json:"zone_name" validate:"required"`
	Country                     string `json:"country" validate:"required"`
	Region                      string `json:"region"`
	ZoneType                    string `json:"zone_type" validate:"required,oneof=urban peri_urban rural remote international"`
	DeliveryDelayDays           int    `json:"delivery_delay_days" validate:"gte=1,lte=30"`
	ReturnDelayDays             int    `json:"return_delay_days" validate:"gte=3,lte=60"`
	WarrantyResponseDays        int    `json:"warranty_response_days" validate:"gte=1,lte=30"`
	CODConfirmationDelayDays    int    `json:"cod_confirmation_delay_days" validate:"gte=1,lte=30"`
	InstallmentReleaseDelayDays int    `json:"installment_release_delay_days" validate:"gte=3,lte=30"`
	Description                 string `json:"description"`
	Priority                    int    `json:"priority"`
}

// UpdateZoneRequest représente la requête pour mettre à jour une zone
type UpdateZoneRequest struct {
	ZoneName                    string `json:"zone_name"`
	Region                      string `json:"region"`
	DeliveryDelayDays           *int   `json:"delivery_delay_days"`
	ReturnDelayDays             *int   `json:"return_delay_days"`
	WarrantyResponseDays        *int   `json:"warranty_response_days"`
	CODConfirmationDelayDays    *int   `json:"cod_confirmation_delay_days"`
	InstallmentReleaseDelayDays *int   `json:"installment_release_delay_days"`
	Description                 string `json:"description"`
	IsActive                    *bool  `json:"is_active"`
	Priority                    *int   `json:"priority"`
}

// ZoneResponse représente la réponse de l'API pour une zone
type ZoneResponse struct {
	ID                          string    `json:"id"`
	ZoneCode                    string    `json:"zone_code"`
	ZoneName                    string    `json:"zone_name"`
	Country                     string    `json:"country"`
	Region                      string    `json:"region"`
	ZoneType                    string    `json:"zone_type"`
	DeliveryDelayDays           int       `json:"delivery_delay_days"`
	ReturnDelayDays             int       `json:"return_delay_days"`
	WarrantyResponseDays        int       `json:"warranty_response_days"`
	CODConfirmationDelayDays    int       `json:"cod_confirmation_delay_days"`
	InstallmentReleaseDelayDays int       `json:"installment_release_delay_days"`
	Description                 string    `json:"description"`
	IsActive                    bool      `json:"is_active"`
	Priority                    int       `json:"priority"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}
