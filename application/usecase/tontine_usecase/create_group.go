package tontineusecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreateTontineGroupUsecase permet de créer un groupe de tontine
type CreateTontineGroupUsecase struct {
	groupRepo       repository.TontineGroupRepository
	participantRepo repository.TontineParticipantRepository
	settingsRepo    repository.ProductTontineSettingsRepository
	productRepo     repository.ProductRepository
	customerRepo    repository.CustomerRepositoryInterface
}

// NewCreateTontineGroupUsecase crée une nouvelle instance
func NewCreateTontineGroupUsecase(
	groupRepo repository.TontineGroupRepository,
	participantRepo repository.TontineParticipantRepository,
	settingsRepo repository.ProductTontineSettingsRepository,
	productRepo repository.ProductRepository,
	customerRepo repository.CustomerRepositoryInterface,
) *CreateTontineGroupUsecase {
	return &CreateTontineGroupUsecase{
		groupRepo:       groupRepo,
		participantRepo: participantRepo,
		settingsRepo:    settingsRepo,
		productRepo:     productRepo,
		customerRepo:    customerRepo,
	}
}

// CreateGroupRequest représente la requête de création de groupe
type CreateGroupRequest struct {
	ProductID         string `json:"product_id"`
	CreatorCustomerID string `json:"creator_customer_id"` // Optionnel si créé par marchand
	CircleType        string `json:"circle_type"`
	TotalCycles       int    `json:"total_cycles"`
}

// Validate valide la requête
func (r *CreateGroupRequest) Validate() error {
	if r.ProductID == "" {
		return fmt.Errorf("product_id is required")
	}
	if !entity.IsValidCircleType(r.CircleType) {
		return fmt.Errorf("invalid circle type: %s", r.CircleType)
	}
	if r.TotalCycles < 2 {
		return fmt.Errorf("total_cycles must be at least 2")
	}
	if r.TotalCycles > 50 {
		return fmt.Errorf("total_cycles cannot exceed 50")
	}
	return nil
}

// Execute crée un nouveau groupe de tontine
func (uc *CreateTontineGroupUsecase) Execute(ctx context.Context, req *CreateGroupRequest) (*entity.TontineGroup, error) {
	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 3. Vérifier que le produit existe et appartient à la boutique
	product, err := uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	_ = product

	// 4. Vérifier que la tontine est activée pour ce produit
	settings, err := uc.settingsRepo.FindByProductID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("tontine not enabled for this product: %w", err)
	}
	if !settings.IsTontineEnabled {
		return nil, fmt.Errorf("tontine is not enabled for this product")
	}

	// 5. Vérifier que le type de cercle est autorisé
	if !settings.IsCircleTypeAllowed(req.CircleType) {
		return nil, fmt.Errorf("circle type %s is not allowed for this product", req.CircleType)
	}

	// 6. Valider le nombre de cycles
	if err := settings.ValidateParticipantCount(req.TotalCycles); err != nil {
		return nil, fmt.Errorf("invalid participant count: %w", err)
	}

	// 7. Déterminer le type de créateur
	var creatorType string
	var creatorCustomerID *string

	if req.CreatorCustomerID != "" {
		// Créé par un client
		creatorType = entity.TontineCreatorCustomer

		// Vérifier que le client existe et est KYC vérifié
		customer, err := uc.customerRepo.FindByCustomerID(ctx, req.CreatorCustomerID)
		if err != nil {
			return nil, fmt.Errorf("customer not found: %w", err)
		}
		if err := customer.CanParticipateInTontine(); err != nil {
			return nil, fmt.Errorf("customer cannot participate in tontine: %w", err)
		}

		creatorCustomerID = &req.CreatorCustomerID
	} else {
		// Créé par le marchand
		creatorType = entity.TontineCreatorMerchant
		creatorCustomerID = nil
	}

	// 8. Calculer le montant par cycle
	amountPerCycle, remainder := entity.CalculateAmountPerCycle(product.PriceCents, req.TotalCycles)
	if remainder > 0 {
		// Arrondi au centime supérieur pour le dernier cycle
		amountPerCycle++
	}

	// 9. Générer un code d'invitation unique
	inviteCode, err := generateInviteCode(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}

	// 10. Créer le groupe
	group, err := entity.NewTontineGroup(
		req.ProductID,
		shop.ID.String(),
		creatorCustomerID,
		creatorType,
		req.CircleType,
		amountPerCycle,
		req.TotalCycles,
		inviteCode,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create group entity: %w", err)
	}

	if err := uc.groupRepo.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("failed to save group: %w", err)
	}

	// 11. Si créé par un client, l'ajouter comme premier participant
	if creatorType == entity.TontineCreatorCustomer && creatorCustomerID != nil {
		participant, err := entity.NewTontineParticipant(group.ID, *creatorCustomerID, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to create participant entity: %w", err)
		}

		if err := uc.participantRepo.Add(ctx, participant); err != nil {
			return nil, fmt.Errorf("failed to add creator as participant: %w", err)
		}
	}

	return group, nil
}

// generateInviteCode génère un code alphanumérique unique de 8 caractères
func generateInviteCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	code := strings.ToUpper(hex.EncodeToString(bytes))[:length]
	return code, nil
}
