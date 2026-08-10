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
// SUBMIT DELIVERY PROOF (TONTINE VOUCHER — participant)
// Phase 3.3b : refuse si escrow groupe disputed / released / refunded
// ============================================================

type SubmitTontineDeliveryProofRequest struct {
	VoucherID string `json:"voucher_id"`
	ProofURL  string `json:"proof_url"`
	Signature string `json:"signature,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Rating    *int   `json:"rating,omitempty"`
}

type SubmitTontineDeliveryProofResponse struct {
	ProofID      string `json:"proof_id"`
	VoucherID    string `json:"voucher_id"`
	EscrowStatus string `json:"escrow_status"`
	DeliveryDate string `json:"delivery_date"`
	Rating       *int   `json:"rating,omitempty"`
	Message      string `json:"message"`
}

func (r *SubmitTontineDeliveryProofRequest) Validate() error {
	if r.VoucherID == "" {
		return fmt.Errorf("voucher_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if _, err := uuid.Parse(r.VoucherID); err != nil {
		return fmt.Errorf("invalid voucher_id format: %w", err)
	}
	if r.Rating != nil && (*r.Rating < 1 || *r.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

type SubmitTontineDeliveryProofUsecase struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	customerRepo       repository.CustomerRepositoryInterface
	escrowRepo         repository.EscrowAccountRepository // Phase 3.3b
	txManager          repository.TxManager
}

func NewSubmitTontineDeliveryProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	customerRepo repository.CustomerRepositoryInterface,
	escrowRepo repository.EscrowAccountRepository,
	txManager repository.TxManager,
) *SubmitTontineDeliveryProofUsecase {
	return &SubmitTontineDeliveryProofUsecase{
		deliveryProofRepo:  deliveryProofRepo,
		tontineVoucherRepo: tontineVoucherRepo,
		customerRepo:       customerRepo,
		escrowRepo:         escrowRepo,
		txManager:          txManager,
	}
}

func (uc *SubmitTontineDeliveryProofUsecase) Execute(ctx context.Context, req *SubmitTontineDeliveryProofRequest) (*SubmitTontineDeliveryProofResponse, error) {
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
		return nil, fmt.Errorf("cannot confirm delivery for voucher status: %s", voucher.Status)
	}

	// ── Phase 3.3b : escrow du groupe tontine ──
	if voucher.GroupID != "" {
		escrow, err := uc.escrowRepo.WithTX(tx).FindByTontineGroupID(ctx, voucher.GroupID)
		if err == nil && escrow != nil {
			switch escrow.Status {
			case entity.EscrowAccountDisputed:
				return nil, fmt.Errorf("cannot submit delivery proof: tontine group escrow is under dispute")
			case entity.EscrowAccountFullyReleased, entity.EscrowAccountRefunded:
				return nil, fmt.Errorf("cannot submit delivery proof: tontine group escrow is already %s", escrow.Status)
			}
		}
	}

	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByTontineVoucherID(ctx, req.VoucherID)
	if err != nil {
		return nil, fmt.Errorf("delivery proof not found: %w", err)
	}

	if !proof.HasShippingProof() {
		return nil, fmt.Errorf("merchant must submit shipping proof first")
	}
	if proof.HasDeliveryProof() {
		return nil, fmt.Errorf("delivery proof already submitted for this voucher")
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
		Str("voucher_id", req.VoucherID).
		Str("proof_id", proof.ID).
		Msg("Tontine delivery proof submitted successfully by participant")

	deliveryDate := ""
	if proof.DeliveryDate != nil {
		deliveryDate = proof.DeliveryDate.Format(time.RFC3339)
	}

	return &SubmitTontineDeliveryProofResponse{
		ProofID:      proof.ID,
		VoucherID:    req.VoucherID,
		EscrowStatus: string(proof.EscrowStatus),
		DeliveryDate: deliveryDate,
		Rating:       req.Rating,
		Message:      "Tontine delivery proof submitted successfully. Escrow status updated to 'delivered'. Dispute window (72h) started.",
	}, nil
}
