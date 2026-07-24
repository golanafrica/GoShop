package shopusecase

//go:generate mockgen -destination=../../../mocks/usecase/mock_shop_owner_verifier.go -package=usecase . ShopOwnerVerifier
//go:generate mockgen -destination=../../../mocks/usecase/mock_shop_payment_settings.go -package=usecase . ShopPaymentSettingsRepository

import (
	"context"
	"fmt"

	shopdto "Goshop/application/dto/shop_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"

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

	// 3. Mettre à jour avec les nouvelles valeurs (Modèle Marketplace Centralisé)
	if req.CashOnDeliveryEnabled != nil {
		currentSettings.CashOnDeliveryEnabled = *req.CashOnDeliveryEnabled
	}

	// CashCommissionRate est un int dans l'entité, on déréférence le pointeur de la requête
	if req.CashCommissionRate != nil {
		currentSettings.CashCommissionRate = *req.CashCommissionRate
	}

	// Mise à jour de la sous-structure YengaPay (sans toucher aux clés API sensibles)
	if req.YengaPayEnabled != nil {
		currentSettings.YengaPay.Enabled = *req.YengaPayEnabled
	}
	if req.YengaPayOperators != nil {
		currentSettings.YengaPay.Operators = req.YengaPayOperators
	}

	// 🛡️ NOTE IMPORTANTE :
	// Les clés API YengaPay (APIKey, OrganizationID, ProjectID, WebhookSecret)
	// ne sont PLUS mises à jour ici. Elles restent intactes en base de données.
	// L'authentification se fait exclusivement via les variables d'environnement globales de GoShop.

	// 4. Sauvegarder
	if err := uc.paymentRepo.UpsertPaymentSettings(ctx, currentSettings); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Bool("yenga_pay_enabled", currentSettings.YengaPay.Enabled).
		Msg("Payment settings updated (Marketplace Centralized Model)")

	// 5. Retourner la réponse
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
	// Valeurs par défaut sécurisées pour éviter les slices nil
	operators := settings.YengaPay.Operators
	if operators == nil {
		operators = []string{}
	}

	return &shopdto.PaymentSettingsResponse{
		ShopID:                settings.ShopID.String(),
		CashOnDeliveryEnabled: settings.CashOnDeliveryEnabled,
		CashCommissionRate:    settings.CashCommissionRate,
		OnlineCommissionRate:  250, // Valeur par défaut (2.5%) car le champ n'existe pas encore dans l'entité
		YengaPayEnabled:       settings.YengaPay.Enabled,
		YengaPayOperators:     operators,
	}
}
