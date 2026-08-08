package deliveryproofusecase

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
// SUBMIT DELIVERY PROOF USECASE (ORDER)
// ============================================================

// SubmitDeliveryProofRequest représente la requête pour confirmer la réception d'une commande
type SubmitDeliveryProofRequest struct {
	OrderID   string `json:"order_id"`
	ProofURL  string `json:"proof_url"`
	Signature string `json:"signature,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Rating    *int   `json:"rating,omitempty"`
}

// SubmitDeliveryProofResponse représente la réponse après confirmation
type SubmitDeliveryProofResponse struct {
	ProofID      string `json:"proof_id"`
	OrderID      string `json:"order_id"`
	EscrowStatus string `json:"escrow_status"`
	DeliveryDate string `json:"delivery_date"`
	Rating       *int   `json:"rating,omitempty"`
	Message      string `json:"message"`
}

// Validate valide la requête
func (r *SubmitDeliveryProofRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	_, err := uuid.Parse(r.OrderID)
	if err != nil {
		return fmt.Errorf("invalid order_id format: %w", err)
	}
	if r.Rating != nil && (*r.Rating < 1 || *r.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

// SubmitDeliveryProofUsecase permet au client de confirmer la réception d'une commande
type SubmitDeliveryProofUsecase struct {
	deliveryProofRepo repository.DeliveryProofRepository
	orderRepo         repository.OrderRepository
	customerRepo      repository.CustomerRepositoryInterface
	txManager         repository.TxManager
}

// NewSubmitDeliveryProofUsecase crée une nouvelle instance
func NewSubmitDeliveryProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	orderRepo repository.OrderRepository,
	customerRepo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
) *SubmitDeliveryProofUsecase {
	return &SubmitDeliveryProofUsecase{
		deliveryProofRepo: deliveryProofRepo,
		orderRepo:         orderRepo,
		customerRepo:      customerRepo,
		txManager:         txManager,
	}
}

// Execute confirme la réception
func (uc *SubmitDeliveryProofUsecase) Execute(ctx context.Context, req *SubmitDeliveryProofRequest) (*SubmitDeliveryProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 3. Récupérer la commande
	order, err := uc.orderRepo.WithTX(tx).FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 4. Vérifier que la commande est dans un état valide
	if order.Status != string(entity.OrderStatusOutForDelivery) {
		return nil, fmt.Errorf("cannot confirm delivery for order status: %s", order.Status)
	}

	// 5. Récupérer la preuve de livraison
	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("delivery proof not found: %w", err)
	}

	// 6. Vérifier que la preuve marchande existe
	if !proof.HasShippingProof() {
		return nil, fmt.Errorf("merchant must submit shipping proof first")
	}

	// 7. Vérifier que la preuve client n'a pas déjà été soumise
	if proof.HasDeliveryProof() {
		return nil, fmt.Errorf("delivery proof already submitted for this order")
	}

	// 8. Soumettre la preuve de réception
	if err := proof.SubmitDeliveryProof(
		req.ProofURL,
		req.Signature,
		req.Notes,
		req.Rating,
	); err != nil {
		return nil, fmt.Errorf("failed to submit delivery proof: %w", err)
	}

	// 9. Mettre à jour en base
	if err := uc.deliveryProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	// 10. Commit la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Logger le succès
	logger.Info().
		Str("order_id", req.OrderID).
		Str("proof_id", proof.ID).
		Msg("Delivery proof submitted successfully by customer")

	// 12. Construire la réponse
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
