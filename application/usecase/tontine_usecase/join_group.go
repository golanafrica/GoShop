package tontineusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// JoinTontineGroupUsecase permet à un client de rejoindre un groupe de tontine
type JoinTontineGroupUsecase struct {
	groupRepo       repository.TontineGroupRepository
	participantRepo repository.TontineParticipantRepository
	customerRepo    repository.CustomerRepositoryInterface
}

// NewJoinTontineGroupUsecase crée une nouvelle instance
func NewJoinTontineGroupUsecase(
	groupRepo repository.TontineGroupRepository,
	participantRepo repository.TontineParticipantRepository,
	customerRepo repository.CustomerRepositoryInterface,
) *JoinTontineGroupUsecase {
	return &JoinTontineGroupUsecase{
		groupRepo:       groupRepo,
		participantRepo: participantRepo,
		customerRepo:    customerRepo,
	}
}

// JoinGroupRequest représente la requête pour rejoindre un groupe
type JoinGroupRequest struct {
	InviteCode string `json:"invite_code"`
	CustomerID string `json:"customer_id"`
}

// Validate valide la requête
func (r *JoinGroupRequest) Validate() error {
	if r.InviteCode == "" {
		return fmt.Errorf("invite_code is required")
	}
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	return nil
}

// JoinGroupResponse représente la réponse après avoir rejoint un groupe
type JoinGroupResponse struct {
	Group       *entity.TontineGroup       `json:"group"`
	Participant *entity.TontineParticipant `json:"participant"`
	GroupReady  bool                       `json:"group_ready"` // true si le groupe est complet
}

// Execute permet à un client de rejoindre un groupe de tontine
func (uc *JoinTontineGroupUsecase) Execute(ctx context.Context, req *JoinGroupRequest) (*JoinGroupResponse, error) {
	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 3. Trouver le groupe par code d'invitation
	group, err := uc.groupRepo.FindByInviteCode(ctx, req.InviteCode)
	if err != nil {
		return nil, fmt.Errorf("code d'invitation invalide")
	}

	// 4. Vérifier que le groupe appartient à la boutique du contexte
	if group.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("ce groupe n'existe pas dans cette boutique")
	}

	// 5. Vérifier le statut du groupe
	if group.Status != entity.TontineStatusPendingMembers {
		return nil, fmt.Errorf("ce groupe n'accepte plus de nouveaux membres (statut: %s)", group.Status)
	}

	// 6. Vérifier que le client existe et est KYC vérifié
	customer, err := uc.customerRepo.FindByCustomerID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("client introuvable: %w", err)
	}
	if err := customer.CanParticipateInTontine(); err != nil {
		return nil, fmt.Errorf("impossible de rejoindre: %w", err)
	}

	// 7. Vérifier qu'il n'est pas déjà membre
	existing, _ := uc.participantRepo.FindByGroupAndCustomer(ctx, group.ID, req.CustomerID)
	if existing != nil {
		return nil, fmt.Errorf("vous êtes déjà membre de ce groupe")
	}

	// 8. Vérifier qu'il reste de la place
	currentCount, err := uc.participantRepo.CountByGroup(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("erreur vérification membres: %w", err)
	}
	if currentCount >= group.TotalCycles {
		return nil, fmt.Errorf("ce groupe est complet (%d/%d membres)", currentCount, group.TotalCycles)
	}

	// 9. Calculer la position du nouveau participant (ordre d'arrivée)
	newPosition := currentCount + 1

	// 10. Créer le participant
	participant, err := entity.NewTontineParticipant(group.ID, req.CustomerID, newPosition)
	if err != nil {
		return nil, fmt.Errorf("erreur création participant: %w", err)
	}

	if err := uc.participantRepo.Add(ctx, participant); err != nil {
		return nil, fmt.Errorf("erreur ajout participant: %w", err)
	}

	// 11. Si le groupe est maintenant complet, le démarrer
	newCount := currentCount + 1
	groupReady := false
	if newCount == group.TotalCycles {
		if err := uc.groupRepo.Start(ctx, group.ID); err != nil {
			return nil, fmt.Errorf("erreur démarrage groupe: %w", err)
		}
		group.Status = entity.TontineStatusActive
		groupReady = true
	}

	return &JoinGroupResponse{
		Group:       group,
		Participant: participant,
		GroupReady:  groupReady,
	}, nil
}
