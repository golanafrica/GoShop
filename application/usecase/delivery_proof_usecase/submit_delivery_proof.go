package deliveryproofusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// SUBMIT DELIVERY PROOF (ORDER — client)
// Phase 3.3 : refuse si escrow disputed / released / refunded
// ============================================================

type SubmitDeliveryProofRequest struct {
	OrderID   string `json:"order_id"`
	ProofURL  string `json:"proof_url"`
	Signature string `json:"signature,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Rating    *int   `json:"rating,omitempty"`
}

type SubmitDeliveryProofResponse struct {
	ProofID      string `json:"proof_id"`
	OrderID      string `json:"order_id"`
	EscrowStatus string `json:"escrow_status"`
	DeliveryDate string `json:"delivery_date"`
	Rating       *int   `json:"rating,omitempty"`
	Message      string `json:"message"`
}

func (r *SubmitDeliveryProofRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if _, err := uuid.Parse(r.OrderID); err != nil {
		return fmt.Errorf("invalid order_id format: %w", err)
	}
	if r.Rating != nil && (*r.Rating < 1 || *r.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

type SubmitDeliveryProofUsecase struct {
	deliveryProofRepo repository.DeliveryProofRepository
	orderRepo         repository.OrderRepository
	customerRepo      repository.CustomerRepositoryInterface
	escrowRepo        repository.EscrowAccountRepository // Phase 3.3
	txManager         repository.TxManager
}

func NewSubmitDeliveryProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	orderRepo repository.OrderRepository,
	customerRepo repository.CustomerRepositoryInterface,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *SubmitDeliveryProofUsecase {
	return &SubmitDeliveryProofUsecase{
		deliveryProofRepo: deliveryProofRepo,
		orderRepo:         orderRepo,
		customerRepo:      customerRepo,
		escrowRepo:        escrowRepo,
		txManager:         txManager,
	}
}

func (uc *SubmitDeliveryProofUsecase) Execute(ctx context.Context, req *SubmitDeliveryProofRequest) (*SubmitDeliveryProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if _, err := tenant.FromContext(ctx); err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	order, err := uc.orderRepo.WithTX(tx).FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	if order.Status != string(entity.OrderStatusOutForDelivery) {
		return nil, fmt.Errorf("cannot confirm delivery for order status: %s", order.Status)
	}

	// ── Phase 3.3 : bloquer si litige ou escrow terminal ──
	escrow, err := uc.escrowRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for this order: %w", err)
	}
	switch escrow.Status {
	case entity.EscrowAccountDisputed:
		return nil, fmt.Errorf("cannot submit delivery proof: order is under dispute")
	case entity.EscrowAccountFullyReleased, entity.EscrowAccountRefunded:
		return nil, fmt.Errorf("cannot submit delivery proof: escrow is already %s", escrow.Status)
	}

	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("delivery proof not found: %w", err)
	}

	if !proof.HasShippingProof() {
		return nil, fmt.Errorf("merchant must submit shipping proof first")
	}
	if proof.HasDeliveryProof() {
		return nil, fmt.Errorf("delivery proof already submitted for this order")
	}

	if err := proof.SubmitDeliveryProof(
		req.ProofURL,
		req.Signature,
		req.Notes,
		req.Rating,
	); err != nil {
		return nil, fmt.Errorf("failed to submit delivery proof: %w", err)
	}

	if err := uc.deliveryProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("order_id", req.OrderID).
		Str("proof_id", proof.ID).
		Msg("Delivery proof submitted successfully by customer")

	deliveryDate := ""
	if proof.DeliveryDate != nil {
		deliveryDate = proof.DeliveryDate.Format(time.RFC3339)
	}

	return &SubmitDeliveryProofResponse{
		ProofID:      proof.ID,
		OrderID:      req.OrderID,
		EscrowStatus: string(proof.EscrowStatus),
		DeliveryDate: deliveryDate,
		Rating:       req.Rating,
		Message:      "Delivery proof submitted successfully. Escrow status updated to 'delivered'. Dispute window (72h) started.",
	}, nil
}
