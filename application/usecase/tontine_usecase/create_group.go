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
	txManager       repository.TxManager // 🆕 FIX B3
}

// NewCreateTontineGroupUsecase crée une nouvelle instance
func NewCreateTontineGroupUsecase(
	groupRepo repository.TontineGroupRepository,
	participantRepo repository.TontineParticipantRepository,
	settingsRepo repository.ProductTontineSettingsRepository,
	productRepo repository.ProductRepository,
	customerRepo repository.CustomerRepositoryInterface,
	txManager repository.TxManager, // 🆕 FIX B3
) *CreateTontineGroupUsecase {
	return &CreateTontineGroupUsecase{
		groupRepo:       groupRepo,
		participantRepo: participantRepo,
		settingsRepo:    settingsRepo,
		productRepo:     productRepo,
		customerRepo:    customerRepo,
		txManager:       txManager,
	}
}

// CreateGroupRequest représente la requête de création de groupe
type CreateGroupRequest struct {
	ProductID         string `json:"product_id"`
	CreatorCustomerID string `json:"creator_customer_id"`
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

	// 3. Vérifier que le produit existe
	_, err = uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// 4. Vérifier que la tontine est activée pour ce produit et récupérer les settings
	settings, err := uc.settingsRepo.FindByProductID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("tontine settings not found: %w", err)
	}

	// 🆕 FIX B4 : Vérifier explicitement que les settings (et donc le produit) appartiennent à cette boutique
	if settings.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("product does not belong to this shop")
	}

	if !settings.IsTontineEnabled {
		return nil, fmt.Errorf("tontine is not enabled for this product")
	}

	// 5. Vérifier que le type de cercle est autorisé
	if !settings.IsCircleTypeAllowed(req.CircleType) {
		return nil, fmt.Errorf("circle type %s is not allowed", req.CircleType)
	}

	// 6. Valider le nombre de cycles
	if err := settings.ValidateParticipantCount(req.TotalCycles); err != nil {
		return nil, fmt.Errorf("invalid participant count: %w", err)
	}

	// 7. Déterminer le type de créateur
	var creatorType string
	var creatorCustomerID *string

	if req.CreatorCustomerID != "" {
		creatorType = entity.TontineCreatorCustomer
		customer, err := uc.customerRepo.FindByCustomerID(ctx, req.CreatorCustomerID)
		if err != nil {
			return nil, fmt.Errorf("customer not found: %w", err)
		}
		if err := customer.CanParticipateInTontine(); err != nil {
			return nil, fmt.Errorf("customer cannot participate: %w", err)
		}
		creatorCustomerID = &req.CreatorCustomerID
	} else {
		creatorType = entity.TontineCreatorMerchant
		creatorCustomerID = nil
	}

	// 8. Calculer le montant par cycle
	// Note: Nous utilisons settings.MinParticipants ou une logique métier si le prix n'est pas directement sur le produit,
	// mais ici on suppose que le prix est géré ailleurs ou que productRepo le retourne.
	// Si productRepo ne retourne pas le prix, il faudra l'ajuster. Pour l'instant, on garde la logique existante.
	// (Si product.PriceCents n'existe pas, il faudra le récupérer via une autre méthode, mais compilons d'abord).

	// Pour éviter l'erreur si product n'a pas PriceCents, on utilise une valeur par défaut ou on suppose que le repo le fournit.
	// Si tu as une erreur sur product.PriceCents, dis-le moi. Sinon, on continue.
	// *Correction* : Comme on n'a plus l'objet product, on ne peut pas faire product.PriceCents.
	// Il faut récupérer le produit avec son prix. Si l'entité Product n'a pas ShopID mais a PriceCents, on peut le garder.
	// Réintroduisons product mais sans vérifier product.ShopID.

	product, err := uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	amountPerCycle := (product.PriceCents + int64(req.TotalCycles) - 1) / int64(req.TotalCycles)

	// 9. Générer un code d'invitation unique
	inviteCode, err := generateInviteCode(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}

	// 10. Créer l'entité groupe
	group, err := entity.NewTontineGroup(req.ProductID, shop.ID.String(), creatorCustomerID, creatorType, req.CircleType, amountPerCycle, req.TotalCycles, inviteCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create group entity: %w", err)
	}

	// 🆕 FIX B3 : Utiliser une transaction pour garantir l'atomicité Groupe + Participant
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	groupRepoTx := uc.groupRepo.WithTX(tx)
	if err := groupRepoTx.Create(ctx, group); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to save group: %w", err)
	}

	// 11. Si créé par un client, l'ajouter comme premier participant
	if creatorType == entity.TontineCreatorCustomer && creatorCustomerID != nil {
		participant, err := entity.NewTontineParticipant(group.ID, *creatorCustomerID, 1)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to create participant entity: %w", err)
		}
		participantRepoTx := uc.participantRepo.WithTX(tx)
		if err := participantRepoTx.Add(ctx, participant); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to add creator as participant: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return group, nil
}

func generateInviteCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes))[:length], nil
}
