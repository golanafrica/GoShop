package shopusecase

//go:generate mockgen -destination=../../../mocks/usecase/mock_shop_owner_verifier.go -package=usecase . ShopOwnerVerifier
//go:generate mockgen -destination=../../../mocks/usecase/mock_shop_payment_settings.go -package=usecase . ShopPaymentSettingsRepository

import (
	"context"
	"fmt"

	shopdto "Goshop/application/dto/shop_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ShopPaymentSettingsRepository définit les méthodes pour gérer les settings
type ShopPaymentSettingsRepository interface {
	GetPaymentSettings(ctx context.Context, shopID uuid.UUID) (*entity.ShopPaymentSettings, error)
	UpsertPaymentSettings(ctx context.Context, settings *entity.ShopPaymentSettings) error
}

// ShopOwnerVerifier vérifie qu'un utilisateur est propriétaire d'une boutique
type ShopOwnerVerifier interface {
	IsOwner(ctx context.Context, shopID uuid.UUID, userID uuid.UUID) (bool, error)
}

// ConfigurePaymentUsecase configure les moyens de paiement d'une boutique
type ConfigurePaymentUsecase struct {
	shopRepo      repository.ShopRepository
	paymentRepo   ShopPaymentSettingsRepository
	ownerVerifier ShopOwnerVerifier
}

// NewConfigurePaymentUsecase crée une nouvelle instance
func NewConfigurePaymentUsecase(
	shopRepo repository.ShopRepository,
	paymentRepo ShopPaymentSettingsRepository,
	ownerVerifier ShopOwnerVerifier,
) *ConfigurePaymentUsecase {
	return &ConfigurePaymentUsecase{
		shopRepo:      shopRepo,
		paymentRepo:   paymentRepo,
		ownerVerifier: ownerVerifier,
	}
}

// Execute met à jour les settings de paiement
func (uc *ConfigurePaymentUsecase) Execute(ctx context.Context, req *shopdto.UpdatePaymentSettingsRequest, userID uuid.UUID) (*shopdto.PaymentSettingsResponse, error) {
	logger := zerolog.Ctx(ctx)

	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		return nil, fmt.Errorf("invalid shop_id: %w", err)
	}

	// 1. Vérifier que l'utilisateur est propriétaire de la boutique
	isOwner, err := uc.ownerVerifier.IsOwner(ctx, shopID, userID)
	if err != nil {
		return nil, fmt.Errorf("check ownership: %w", err)
	}
	if !isOwner {
		return nil, fmt.Errorf("user is not the owner of this shop")
	}

	// 2. Récupérer les settings actuels
	currentSettings, err := uc.paymentRepo.GetPaymentSettings(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("get current settings: %w", err)
	}

	// 3. Mettre à jour avec les nouvelles valeurs
	if req.OrangeMoneyEnabled != nil {
		currentSettings.OrangeMoney = *req.OrangeMoneyEnabled
	}
	if req.MoovMoneyEnabled != nil {
		currentSettings.MoovMoney = *req.MoovMoneyEnabled
	}
	if req.WaveEnabled != nil {
		currentSettings.Wave = *req.WaveEnabled
	}

	// Mise à jour Yenga Pay
	if req.YengaPay != nil {
		if req.YengaPay.Enabled != nil {
			currentSettings.YengaPay.Enabled = *req.YengaPay.Enabled
		}
		if req.YengaPay.APIKey != nil {
			currentSettings.YengaPay.APIKey = *req.YengaPay.APIKey
		}
		if req.YengaPay.OrganizationID != nil {
			currentSettings.YengaPay.OrganizationID = *req.YengaPay.OrganizationID
		}
		if req.YengaPay.ProjectID != nil {
			currentSettings.YengaPay.ProjectID = *req.YengaPay.ProjectID
		}
		if req.YengaPay.WebhookSecret != nil {
			currentSettings.YengaPay.WebhookSecret = *req.YengaPay.WebhookSecret
		}
		if len(req.YengaPay.Operators) > 0 {
			currentSettings.YengaPay.Operators = req.YengaPay.Operators
		}
		if req.YengaPay.Env != nil {
			currentSettings.YengaPay.Env = *req.YengaPay.Env
		}
	}

	// 4. Sauvegarder
	if err := uc.paymentRepo.UpsertPaymentSettings(ctx, currentSettings); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Bool("yenga_pay_enabled", currentSettings.YengaPay.Enabled).
		Msg("Payment settings updated")

	// 5. Retourner la réponse (sans les clés sensibles)
	return uc.toResponse(currentSettings), nil
}

// GetPaymentSettings récupère les settings d'une boutique
func (uc *ConfigurePaymentUsecase) GetPaymentSettings(ctx context.Context, shopID string, userID uuid.UUID) (*shopdto.PaymentSettingsResponse, error) {
	shopUUID, err := uuid.Parse(shopID)
	if err != nil {
		return nil, fmt.Errorf("invalid shop_id: %w", err)
	}

	// Vérifier que l'utilisateur est propriétaire
	isOwner, err := uc.ownerVerifier.IsOwner(ctx, shopUUID, userID)
	if err != nil {
		return nil, fmt.Errorf("check ownership: %w", err)
	}
	if !isOwner {
		return nil, fmt.Errorf("user is not the owner of this shop")
	}

	settings, err := uc.paymentRepo.GetPaymentSettings(ctx, shopUUID)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	return uc.toResponse(settings), nil
}

// toResponse convertit les settings en réponse DTO
func (uc *ConfigurePaymentUsecase) toResponse(settings *entity.ShopPaymentSettings) *shopdto.PaymentSettingsResponse {
	return &shopdto.PaymentSettingsResponse{
		ShopID:             settings.ShopID.String(),
		OrangeMoneyEnabled: settings.OrangeMoney,
		MoovMoneyEnabled:   settings.MoovMoney,
		WaveEnabled:        settings.Wave,
		YengaPay: shopdto.YengaPayResponseDTO{
			Enabled:       settings.YengaPay.Enabled,
			HasAPIKey:     settings.YengaPay.APIKey != "",
			HasOrgID:      settings.YengaPay.OrganizationID != "",
			HasProjectID:  settings.YengaPay.ProjectID != "",
			HasWebhook:    settings.YengaPay.WebhookSecret != "",
			Operators:     settings.YengaPay.Operators,
			Env:           settings.YengaPay.Env,
			IsUsingGlobal: !settings.YengaPay.Enabled || settings.YengaPay.APIKey == "",
		},
	}
}

// GetTenantShopID récupère le shop ID du contexte multi-tenant
func GetTenantShopID(ctx context.Context) (uuid.UUID, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return shop.ID, nil
}
