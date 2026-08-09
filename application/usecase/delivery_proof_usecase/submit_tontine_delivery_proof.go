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

// SubmitTontineDeliveryProofRequest représente la requête pour confirmer la réception d'un voucher tontine
type SubmitTontineDeliveryProofRequest struct {
	VoucherID string `json:"voucher_id"`
	ProofURL  string `json:"proof_url"`
	Signature string `json:"signature,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Rating    *int   `json:"rating,omitempty"`
}

// SubmitTontineDeliveryProofResponse représente la réponse après confirmation
type SubmitTontineDeliveryProofResponse struct {
	ProofID      string `json:"proof_id"`
	VoucherID    string `json:"voucher_id"`
	EscrowStatus string `json:"escrow_status"`
	DeliveryDate string `json:"delivery_date"`
	Rating       *int   `json:"rating,omitempty"`
	Message      string `json:"message"`
}

// Validate valide la requête
func (r *SubmitTontineDeliveryProofRequest) Validate() error {
	if r.VoucherID == "" {
		return fmt.Errorf("voucher_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	_, err := uuid.Parse(r.VoucherID)
	if err != nil {
		return fmt.Errorf("invalid voucher_id format: %w", err)
	}
	if r.Rating != nil && (*r.Rating < 1 || *r.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

// SubmitTontineDeliveryProofUsecase permet au participant de confirmer la réception d'un voucher tontine
type SubmitTontineDeliveryProofUsecase struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	customerRepo       repository.CustomerRepositoryInterface
	txManager          repository.TxManager
}

// NewSubmitTontineDeliveryProofUsecase crée une nouvelle instance
func NewSubmitTontineDeliveryProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	customerRepo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
) *SubmitTontineDeliveryProofUsecase {
	return &SubmitTontineDeliveryProofUsecase{
		deliveryProofRepo:  deliveryProofRepo,
		tontineVoucherRepo: tontineVoucherRepo,
		customerRepo:       customerRepo,
		txManager:          txManager,
	}
}

// Execute confirme la réception du voucher
func (uc *SubmitTontineDeliveryProofUsecase) Execute(ctx context.Context, req *SubmitTontineDeliveryProofRequest) (*SubmitTontineDeliveryProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Récupérer le voucher
	voucher, err := uc.tontineVoucherRepo.WithTX(tx).FindByID(ctx, req.VoucherID)
	if err != nil {
		return nil, fmt.Errorf("tontine voucher not found: %w", err)
	}

	// 5. Vérifier que le voucher appartient au shop (FIX: comparaison UUID robuste)
	voucherShopUUID, err := uuid.Parse(voucher.ShopID)
	if err != nil || voucherShopUUID != shop.ID {
		return nil, fmt.Errorf("access denied: voucher does not belong to tenant shop")
	}

	// 6. Vérifier que le voucher est dans un état valide
	if voucher.Status != entity.VoucherStatusGenerated {
		return nil, fmt.Errorf("cannot confirm delivery for voucher status: %s", voucher.Status)
	}

	// 7. Récupérer la preuve de livraison
	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByTontineVoucherID(ctx, req.VoucherID)
	if err != nil {
		return nil, fmt.Errorf("delivery proof not found: %w", err)
	}

	// 8. Vérifier que la preuve marchande existe
	if !proof.HasShippingProof() {
		return nil, fmt.Errorf("merchant must submit shipping proof first")
	}

	// 9. Vérifier que la preuve client n'a pas déjà été soumise
	if proof.HasDeliveryProof() {
		return nil, fmt.Errorf("delivery proof already submitted for this voucher")
	}

	// 10. Soumettre la preuve de réception
	if err := proof.SubmitDeliveryProof(
		req.ProofURL,
		req.Signature,
		req.Notes,
		req.Rating,
	); err != nil {
		return nil, fmt.Errorf("failed to submit delivery proof: %w", err)
	}

	// 11. Mettre à jour en base
	if err := uc.deliveryProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	// 12. Commit la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 13. Logger le succès
	logger.Info().
		Str("voucher_id", req.VoucherID).
		Str("proof_id", proof.ID).
		Msg("Tontine delivery proof submitted successfully by participant")

	// 14. Construire la réponse
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
