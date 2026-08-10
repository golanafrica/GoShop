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
// SUBMIT SHIPPING PROOF (ORDER)
// Phase 3.3 : refuse si escrow disputed / released / refunded
// ============================================================

type SubmitShippingProofRequest struct {
	OrderID        string `json:"order_id"`
	ProofURL       string `json:"proof_url"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

type SubmitShippingProofResponse struct {
	ProofID        string `json:"proof_id"`
	OrderID        string `json:"order_id"`
	EscrowStatus   string `json:"escrow_status"`
	ShippingDate   string `json:"shipping_date"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Message        string `json:"message"`
}

func (r *SubmitShippingProofRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if _, err := uuid.Parse(r.OrderID); err != nil {
		return fmt.Errorf("invalid order_id format: %w", err)
	}
	return nil
}

type SubmitShippingProofUsecase struct {
	deliveryProofRepo repository.DeliveryProofRepository
	orderRepo         repository.OrderRepository
	escrowRepo        repository.EscrowAccountRepository // Phase 3.3
	txManager         repository.TxManager
}

func NewSubmitShippingProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	orderRepo repository.OrderRepository,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *SubmitShippingProofUsecase {
	return &SubmitShippingProofUsecase{
		deliveryProofRepo: deliveryProofRepo,
		orderRepo:         orderRepo,
		escrowRepo:        escrowRepo,
		txManager:         txManager,
	}
}

func (uc *SubmitShippingProofUsecase) Execute(ctx context.Context, req *SubmitShippingProofRequest) (*SubmitShippingProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	shop, err := tenant.FromContext(ctx)
	if err != nil {
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

	orderShopUUID, parseErr := uuid.Parse(order.ShopID)
	if parseErr != nil || orderShopUUID != shop.ID {
		return nil, fmt.Errorf("access denied: order does not belong to tenant shop")
	}

	validStatuses := map[string]bool{
		string(entity.OrderStatusConfirmed):      true,
		string(entity.OrderStatusOutForDelivery): true,
	}
	if !validStatuses[order.Status] {
		return nil, fmt.Errorf("cannot submit shipping proof for order status: %s", order.Status)
	}

	// ── Phase 3.3 : bloquer si litige ou escrow terminal ──
	escrow, err := uc.escrowRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for this order: %w", err)
	}
	switch escrow.Status {
	case entity.EscrowAccountDisputed:
		return nil, fmt.Errorf("cannot submit shipping proof: order is under dispute")
	case entity.EscrowAccountFullyReleased, entity.EscrowAccountRefunded:
		return nil, fmt.Errorf("cannot submit shipping proof: escrow is already %s", escrow.Status)
	}

	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		proof = entity.NewDeliveryProofForOrder(req.OrderID)
		if err := uc.deliveryProofRepo.WithTX(tx).Create(ctx, proof); err != nil {
			return nil, fmt.Errorf("failed to create delivery proof: %w", err)
		}
	}

	if proof.HasShippingProof() {
		return nil, fmt.Errorf("shipping proof already submitted for this order")
	}

	if err := proof.SubmitShippingProof(
		req.ProofURL,
		req.TrackingNumber,
		req.Carrier,
		req.Notes,
	); err != nil {
		return nil, fmt.Errorf("failed to submit shipping proof: %w", err)
	}

	if err := uc.deliveryProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	order.Status = string(entity.OrderStatusOutForDelivery)
	if err := uc.orderRepo.WithTX(tx).UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order status to out_for_delivery: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shop.ID.String()).
		Str("proof_id", proof.ID).
		Str("tracking_number", req.TrackingNumber).
		Msg("Shipping proof submitted successfully, order status updated to out_for_delivery")

	shippingDate := ""
	if proof.ShippingDate != nil {
		shippingDate = proof.ShippingDate.Format(time.RFC3339)
	}

	return &SubmitShippingProofResponse{
		ProofID:        proof.ID,
		OrderID:        req.OrderID,
		EscrowStatus:   string(proof.EscrowStatus),
		ShippingDate:   shippingDate,
		TrackingNumber: req.TrackingNumber,
		Carrier:        req.Carrier,
		Message:        "Shipping proof submitted successfully. Order status updated to 'out_for_delivery'. Escrow status updated to 'shipped'.",
	}, nil
}
