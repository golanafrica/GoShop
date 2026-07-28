package disputeusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type OpenDisputeUsecase struct {
	disputeRepo repository.DisputeRepository
	orderRepo   repository.OrderRepository
	escrowRepo  repository.EscrowAccountRepository
}

func NewOpenDisputeUsecase(
	disputeRepo repository.DisputeRepository,
	orderRepo repository.OrderRepository,
	escrowRepo repository.EscrowAccountRepository,
) *OpenDisputeUsecase {
	return &OpenDisputeUsecase{
		disputeRepo: disputeRepo,
		orderRepo:   orderRepo,
		escrowRepo:  escrowRepo,
	}
}

type OpenDisputeRequest struct {
	OrderID       string
	ShopID        uuid.UUID // 🆕 AJOUTÉ : Provenant directement du contexte tenant
	InitiatorID   uuid.UUID
	InitiatorRole entity.InitiatorRole
	Reason        string
}

func (uc *OpenDisputeUsecase) Execute(ctx context.Context, req *OpenDisputeRequest) (*entity.Dispute, error) {
	logger := zerolog.Ctx(ctx)

	orderUUID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order_id: %w", err)
	}

	// 1. Vérifier la commande
	order, err := uc.orderRepo.FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// ✅ Autoriser l'ouverture de litige pour : confirmed, out_for_delivery, delivered
	validStatuses := map[string]bool{
		string(entity.OrderStatusConfirmed):      true,
		string(entity.OrderStatusOutForDelivery): true,
		string(entity.OrderStatusDelivered):      true,
	}

	if !validStatuses[order.Status] {
		return nil, fmt.Errorf("cannot open dispute for order status: %s", order.Status)
	}

	// 2. Vérifier que l'Escrow est toujours en attente
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for this order: %w", err)
	}
	if escrow.Status != entity.EscrowAccountFundsHeld {
		return nil, fmt.Errorf("cannot open dispute: escrow is already %s", escrow.Status)
	}

	// 3. Vérifier qu'aucun litige n'est déjà ouvert pour cette commande
	existingDispute, _ := uc.disputeRepo.FindByOrderID(ctx, orderUUID)
	if existingDispute != nil && existingDispute.Status == entity.DisputeStatusPending {
		return nil, fmt.Errorf("a pending dispute already exists for this order")
	}

	// 4. Créer le litige
	newDispute := &entity.Dispute{
		ID:            uuid.New(),
		OrderID:       orderUUID,
		ShopID:        req.ShopID, // ✅ Utilisation du ShopID fiable provenant du contexte
		InitiatorID:   req.InitiatorID,
		InitiatorRole: req.InitiatorRole,
		Reason:        req.Reason,
		Status:        entity.DisputeStatusPending,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := uc.disputeRepo.Create(ctx, newDispute); err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	logger.Info().
		Str("dispute_id", newDispute.ID.String()).
		Str("order_id", req.OrderID).
		Msg("Dispute opened successfully")

	return newDispute, nil
}
