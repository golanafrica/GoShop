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

// SubmitTontineShippingProofRequest représente la requête pour soumettre une preuve d'expédition tontine
type SubmitTontineShippingProofRequest struct {
	VoucherID      string `json:"voucher_id"`
	ProofURL       string `json:"proof_url"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// SubmitTontineShippingProofResponse représente la réponse après soumission
type SubmitTontineShippingProofResponse struct {
	ProofID        string `json:"proof_id"`
	VoucherID      string `json:"voucher_id"`
	EscrowStatus   string `json:"escrow_status"`
	ShippingDate   string `json:"shipping_date"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Message        string `json:"message"`
}

// Validate valide la requête
func (r *SubmitTontineShippingProofRequest) Validate() error {
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
	return nil
}

// SubmitTontineShippingProofUsecase permet au marchand de soumettre une preuve d'expédition pour un voucher tontine
type SubmitTontineShippingProofUsecase struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	txManager          repository.TxManager
}

// NewSubmitTontineShippingProofUsecase crée une nouvelle instance
func NewSubmitTontineShippingProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	txManager repository.TxManager,
) *SubmitTontineShippingProofUsecase {
	return &SubmitTontineShippingProofUsecase{
		deliveryProofRepo:  deliveryProofRepo,
		tontineVoucherRepo: tontineVoucherRepo,
		txManager:          txManager,
	}
}

// Execute soumet la preuve d'expédition pour un voucher tontine
func (uc *SubmitTontineShippingProofUsecase) Execute(ctx context.Context, req *SubmitTontineShippingProofRequest) (*SubmitTontineShippingProofResponse, error) {
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
		return nil, fmt.Errorf("cannot submit shipping proof for voucher status: %s", voucher.Status)
	}

	// 7. Vérifier que le voucher n'a pas expiré
	if voucher.IsExpired() {
		return nil, fmt.Errorf("voucher has expired")
	}

	// 8. Récupérer ou créer la preuve de livraison
	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByTontineVoucherID(ctx, req.VoucherID)
	if err != nil {
		// Créer une nouvelle preuve si elle n'existe pas
		proof = entity.NewDeliveryProofForTontineVoucher(req.VoucherID)
		if err := uc.deliveryProofRepo.WithTX(tx).Create(ctx, proof); err != nil {
			return nil, fmt.Errorf("failed to create delivery proof: %w", err)
		}
	}

	// 9. Vérifier que la preuve n'a pas déjà été soumise
	if proof.HasShippingProof() {
		return nil, fmt.Errorf("shipping proof already submitted for this voucher")
	}

	// 10. Soumettre la preuve d'expédition
	if err := proof.SubmitShippingProof(
		req.ProofURL,
		req.TrackingNumber,
		req.Carrier,
		req.Notes,
	); err != nil {
		return nil, fmt.Errorf("failed to submit shipping proof: %w", err)
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
		Str("shop_id", shop.ID.String()).
		Str("proof_id", proof.ID).
		Str("tracking_number", req.TrackingNumber).
		Msg("Tontine voucher shipping proof submitted successfully")

	// 14. Construire la réponse
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
