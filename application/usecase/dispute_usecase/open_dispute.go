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

// ============================================================
// OPEN DISPUTE USECASE
// ============================================================
// Phase 3.2 : ouverture de litige atomique.
// Create(dispute) + escrow.Dispute() + Update(escrow) dans la même TX.
// Si un des écritures échoue → rollback complet (pas de litige orphelin
// ni d'escrow resté funds_held alors qu'un pending dispute existe).
// ============================================================

type OpenDisputeUsecase struct {
	disputeRepo repository.DisputeRepository
	orderRepo   repository.OrderRepository
	escrowRepo  repository.EscrowAccountRepository
	txManager   repository.TxManager
}

func NewOpenDisputeUsecase(
	disputeRepo repository.DisputeRepository,
	orderRepo repository.OrderRepository,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *OpenDisputeUsecase {
	return &OpenDisputeUsecase{
		disputeRepo: disputeRepo,
		orderRepo:   orderRepo,
		escrowRepo:  escrowRepo,
		txManager:   txManager,
	}
}

type OpenDisputeRequest struct {
	OrderID       string
	ShopID        uuid.UUID
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

	// 1. Vérifier la commande (lecture hors TX — fail-fast)
	order, err := uc.orderRepo.FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Autoriser litige uniquement sur commandes déjà payées / en livraison
	validStatuses := map[string]bool{
		string(entity.OrderStatusConfirmed):      true,
		string(entity.OrderStatusOutForDelivery): true,
		string(entity.OrderStatusDelivered):      true,
	}
	if !validStatuses[order.Status] {
		return nil, fmt.Errorf("cannot open dispute for order status: %s", order.Status)
	}

	// 2. Escrow doit exister et être funds_held (pas déjà released / refunded / disputed)
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for this order: %w", err)
	}
	if escrow.Status != entity.EscrowAccountFundsHeld {
		return nil, fmt.Errorf("cannot open dispute: escrow is already %s", escrow.Status)
	}

	// 3. Pas de litige pending déjà ouvert
	existingDispute, _ := uc.disputeRepo.FindByOrderID(ctx, orderUUID)
	if existingDispute != nil && existingDispute.Status == entity.DisputeStatusPending {
		return nil, fmt.Errorf("a pending dispute already exists for this order")
	}

	// 4. Transaction atomique : dispute + escrow disputed
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	disputeRepoTx := uc.disputeRepo.WithTX(tx)
	escrowRepoTx := uc.escrowRepo.WithTX(tx)

	now := time.Now().UTC()
	newDispute := &entity.Dispute{
		ID:            uuid.New(),
		OrderID:       orderUUID,
		ShopID:        req.ShopID,
		InitiatorID:   req.InitiatorID,
		InitiatorRole: req.InitiatorRole,
		Reason:        req.Reason,
		Status:        entity.DisputeStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := disputeRepoTx.Create(ctx, newDispute); err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	// Machine à états domain : funds_held → disputed (bloque auto-release 3.1)
	if err := escrow.Dispute(); err != nil {
		return nil, fmt.Errorf("failed to mark escrow as disputed: %w", err)
	}
	if err := escrowRepoTx.Update(ctx, escrow); err != nil {
		return nil, fmt.Errorf("failed to update escrow status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit open dispute: %w", err)
	}

	logger.Info().
		Str("dispute_id", newDispute.ID.String()).
		Str("order_id", req.OrderID).
		Str("escrow_status", string(escrow.Status)).
		Str("initiator_role", string(req.InitiatorRole)).
		Msg("Dispute opened successfully, escrow locked (atomic)")

	return newDispute, nil
}
