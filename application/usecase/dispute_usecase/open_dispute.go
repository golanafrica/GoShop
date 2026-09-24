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
// Ouverture de litige atomique.
//
// Chemins :
//  1. escrow funds_held  → Create(dispute) + escrow.Dispute() (bloque auto-release)
//  2. escrow released    → Create(dispute) SEULEMENT (fonds déjà chez le marchand ;
//                         resolve customer_wins fera refund + clawback)
//  Autres statuts (refunded, disputed, partial…) → refus
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

	validStatuses := map[string]bool{
		string(entity.OrderStatusConfirmed):      true,
		string(entity.OrderStatusOutForDelivery): true,
		string(entity.OrderStatusDelivered):      true,
	}
	if !validStatuses[order.Status] {
		return nil, fmt.Errorf("cannot open dispute for order status: %s", order.Status)
	}

	// 2. Escrow : funds_held (classique) OU released (post-libération → clawback possible)
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for this order: %w", err)
	}

	switch escrow.Status {
	case entity.EscrowAccountFundsHeld, entity.EscrowAccountFullyReleased:
		// OK
	default:
		return nil, fmt.Errorf("cannot open dispute: escrow is already %s", escrow.Status)
	}

	postRelease := escrow.Status == entity.EscrowAccountFullyReleased

	// 3. Pas de litige pending déjà ouvert
	existingDispute, _ := uc.disputeRepo.FindByOrderID(ctx, orderUUID)
	if existingDispute != nil &&
		(existingDispute.Status == entity.DisputeStatusPending ||
			existingDispute.Status == entity.DisputeStatusUnderReview) {
		return nil, fmt.Errorf("a pending dispute already exists for this order")
	}

	// 4. Transaction atomique
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

	if !postRelease {
		// funds_held → disputed (bloque le scheduler auto-release)
		if err := escrow.Dispute(); err != nil {
			return nil, fmt.Errorf("failed to mark escrow as disputed: %w", err)
		}
		if err := escrowRepoTx.Update(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to update escrow status: %w", err)
		}
	}
	// postRelease : on laisse status=released ; resolve customer_wins fera clawback

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit open dispute: %w", err)
	}

	logger.Info().
		Str("dispute_id", newDispute.ID.String()).
		Str("order_id", req.OrderID).
		Str("escrow_status", string(escrow.Status)).
		Bool("post_release", postRelease).
		Str("initiator_role", string(req.InitiatorRole)).
		Msg("Dispute opened successfully (atomic)")

	return newDispute, nil
}
