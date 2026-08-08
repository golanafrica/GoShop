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
// SUBMIT SHIPPING PROOF USECASE (ORDER)
// ============================================================

// SubmitShippingProofRequest représente la requête pour soumettre une preuve d'expédition
type SubmitShippingProofRequest struct {
	OrderID        string `json:"order_id"`
	ProofURL       string `json:"proof_url"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// SubmitShippingProofResponse représente la réponse après soumission
type SubmitShippingProofResponse struct {
	ProofID        string `json:"proof_id"`
	OrderID        string `json:"order_id"`
	EscrowStatus   string `json:"escrow_status"`
	ShippingDate   string `json:"shipping_date"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Carrier        string `json:"carrier,omitempty"`
	Message        string `json:"message"`
}

// Validate valide la requête
func (r *SubmitShippingProofRequest) Validate() error {
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
	return nil
}

// SubmitShippingProofUsecase permet au marchand de soumettre une preuve d'expédition pour une commande
type SubmitShippingProofUsecase struct {
	deliveryProofRepo repository.DeliveryProofRepository
	orderRepo         repository.OrderRepository
	txManager         repository.TxManager
}

// NewSubmitShippingProofUsecase crée une nouvelle instance
func NewSubmitShippingProofUsecase(
	deliveryProofRepo repository.DeliveryProofRepository,
	orderRepo repository.OrderRepository,
	txManager repository.TxManager,
) *SubmitShippingProofUsecase {
	return &SubmitShippingProofUsecase{
		deliveryProofRepo: deliveryProofRepo,
		orderRepo:         orderRepo,
		txManager:         txManager,
	}
}

// Execute soumet la preuve d'expédition
func (uc *SubmitShippingProofUsecase) Execute(ctx context.Context, req *SubmitShippingProofRequest) (*SubmitShippingProofResponse, error) {
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
	shopID := shop.ID.String()

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Récupérer la commande
	order, err := uc.orderRepo.WithTX(tx).FindByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 5. Vérifier que la commande appartient au shop
	if order.ShopID != shopID {
		return nil, fmt.Errorf("access denied: order does not belong to tenant shop")
	}

	// 6. Vérifier que la commande est dans un état valide
	validStatuses := map[string]bool{
		string(entity.OrderStatusConfirmed):      true,
		string(entity.OrderStatusOutForDelivery): true,
	}
	if !validStatuses[order.Status] {
		return nil, fmt.Errorf("cannot submit shipping proof for order status: %s", order.Status)
	}

	// 7. Récupérer ou créer la preuve de livraison
	proof, err := uc.deliveryProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		// Créer une nouvelle preuve si elle n'existe pas
		proof = entity.NewDeliveryProofForOrder(req.OrderID)
		if err := uc.deliveryProofRepo.WithTX(tx).Create(ctx, proof); err != nil {
			return nil, fmt.Errorf("failed to create delivery proof: %w", err)
		}
	}

	// 8. Vérifier que la preuve n'a pas déjà été soumise
	if proof.HasShippingProof() {
		return nil, fmt.Errorf("shipping proof already submitted for this order")
	}

	// 9. Soumettre la preuve d'expédition
	if err := proof.SubmitShippingProof(
		req.ProofURL,
		req.TrackingNumber,
		req.Carrier,
		req.Notes,
	); err != nil {
		return nil, fmt.Errorf("failed to submit shipping proof: %w", err)
	}

	// 10. Mettre à jour en base
	if err := uc.deliveryProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	// 11. Commit la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Logger le succès
	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Str("proof_id", proof.ID).
		Str("tracking_number", req.TrackingNumber).
		Msg("Shipping proof submitted successfully")

	// 13. Construire la réponse
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
		Message:        "Shipping proof submitted successfully. Escrow status updated to 'shipped'.",
	}, nil
}
