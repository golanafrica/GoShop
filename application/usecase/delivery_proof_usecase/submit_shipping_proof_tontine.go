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
// SUBMIT SHIPPING PROOF (TONTINE VOUCHER)
// Phase 3.3b : refuse si escrow groupe disputed / released / refunded
// ============================================================

type SubmitTontineShippingProofRequest struct {
	VoucherID      string `json:"voucher_id"`
	ProofURL       string `json:"proof_url"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

type SubmitTontineShippingProofResponse struct {
	ProofID        string `json:"proof_id"`
	VoucherID      string `json:"voucher_id"`
	EscrowStatus   string `json:"escrow_status"`
	ShippingDate   string `json:"shipping_date"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Message        string `json:"message"`
}

func (r *SubmitTontineShippingProofRequest) Validate() error {
	if r.VoucherID == "" {
		return fmt.Errorf("voucher_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if _, err := uuid.Parse(r.VoucherID); err != nil {
		return fmt.Errorf("invalid voucher_id format: %w", err)
	}
	return nil
}

type SubmitTontineShippingProofUsecase struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	escrowRepo         repository.EscrowAccountRepository // Phase 3.3b
	txManager          repository.TxManager
}

func NewSubmitTontineShippingProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *SubmitTontineShippingProofUsecase {
	return &SubmitTontineShippingProofUsecase{
		deliveryProofRepo:  deliveryProofRepo,
		tontineVoucherRepo: tontineVoucherRepo,
		escrowRepo:         escrowRepo,
		txManager:          txManager,
	}
}

func (uc *SubmitTontineShippingProofUsecase) Execute(ctx context.Context, req *SubmitTontineShippingProofRequest) (*SubmitTontineShippingProofResponse, error) {
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

	voucher, err := uc.tontineVoucherRepo.WithTX(tx).FindByID(ctx, req.VoucherID)
	if err != nil {
		return nil, fmt.Errorf("tontine voucher not found: %w", err)
	}

	voucherShopUUID, err := uuid.Parse(voucher.ShopID)
	if err != nil || voucherShopUUID != shop.ID {
		return nil, fmt.Errorf("access denied: voucher does not belong to tenant shop")
	}

	if voucher.Status != entity.VoucherStatusGenerated {
		return nil, fmt.Errorf("cannot submit shipping proof for voucher status: %s", voucher.Status)
	}
	if voucher.IsExpired() {
		return nil, fmt.Errorf("voucher has expired")
	}

	// ── Phase 3.3b : escrow du groupe tontine ──
	if voucher.GroupID != "" {
		escrow, err := uc.escrowRepo.WithTX(tx).FindByTontineGroupID(ctx, voucher.GroupID)
		if err == nil && escrow != nil {
			switch escrow.Status {
			case entity.EscrowAccountDisputed:
				return nil, fmt.Errorf("cannot submit shipping proof: tontine group escrow is under dispute")
			case entity.EscrowAccountFullyReleased, entity.EscrowAccountRefunded:
				return nil, fmt.Errorf("cannot submit shipping proof: tontine group escrow is already %s", escrow.Status)
			}
		}
		// Si pas d'escrow groupe (modèle held wallet seul) → on laisse passer
	}

	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByTontineVoucherID(ctx, req.VoucherID)
	if err != nil {
		proof = entity.NewDeliveryProofForTontineVoucher(req.VoucherID)
		if err := uc.deliveryProofRepo.WithTX(tx).Create(ctx, proof); err != nil {
			return nil, fmt.Errorf("failed to create delivery proof: %w", err)
		}
	}

	if proof.HasShippingProof() {
		return nil, fmt.Errorf("shipping proof already submitted for this voucher")
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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("voucher_id", req.VoucherID).
		Str("shop_id", shop.ID.String()).
		Str("proof_id", proof.ID).
		Str("tracking_number", req.TrackingNumber).
		Msg("Tontine voucher shipping proof submitted successfully")

	shippingDate := ""
	if proof.ShippingDate != nil {
		shippingDate = proof.ShippingDate.Format(time.RFC3339)
	}

	return &SubmitTontineShippingProofResponse{
		ProofID:        proof.ID,
		VoucherID:      req.VoucherID,
		EscrowStatus:   string(proof.EscrowStatus),
		ShippingDate:   shippingDate,
		TrackingNumber: req.TrackingNumber,
		Carrier:        req.Carrier,
		Message:        "Tontine voucher shipping proof submitted successfully. Escrow status updated to 'shipped'.",
	}, nil
}
