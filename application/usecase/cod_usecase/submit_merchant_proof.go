package codusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// SUBMIT MERCHANT PROOF USECASE
// ============================================================

// SubmitMerchantProofRequest représente la requête pour soumettre une preuve marchand
type SubmitMerchantProofRequest struct {
	// Identifiants
	OrderID string `json:"order_id"`

	// Preuve de réception (photo du reçu, signature client, etc.)
	ProofURL string `json:"proof_url"`

	// Montant reçu du client (en centimes)
	AmountCents int64 `json:"amount_cents"`

	// Date de réception du paiement
	ReceiptDate time.Time `json:"receipt_date"`

	// Optionnels
	Notes *string `json:"notes,omitempty"`
}

// SubmitMerchantProofResponse représente la réponse après soumission
type SubmitMerchantProofResponse struct {
	// Preuve mise à jour
	ProofID    string                `json:"proof_id"`
	OrderID    string                `json:"order_id"`
	ShopID     string                `json:"shop_id"`
	CustomerID string                `json:"customer_id"`
	Status     entity.CODProofStatus `json:"status"`

	// Preuve marchand
	MerchantProofURL    string    `json:"merchant_proof_url"`
	MerchantAmountCents int64     `json:"merchant_amount_cents"`
	MerchantReceiptDate time.Time `json:"merchant_receipt_date"`
	MerchantSubmittedAt time.Time `json:"merchant_submitted_at"`

	// Preuve client (si déjà soumise)
	HasClientProof    bool       `json:"has_client_proof"`
	ClientAmountCents *int64     `json:"client_amount_cents,omitempty"`
	ClientPaymentDate *time.Time `json:"client_payment_date,omitempty"`

	// Cohérence (si les deux preuves sont présentes)
	AmountsMatch *bool `json:"amounts_match,omitempty"`
	DatesMatch   *bool `json:"dates_match,omitempty"`
	IsCoherent   bool  `json:"is_coherent"`

	// Commission
	CommissionCents int64 `json:"commission_cents"`

	// Délai
	Deadline      time.Time `json:"deadline"`
	DaysRemaining int       `json:"days_remaining"`
}

// Validate valide la requête
func (r *SubmitMerchantProofRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if r.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	if r.ReceiptDate.IsZero() {
		return fmt.Errorf("receipt_date is required")
	}
	if r.ReceiptDate.After(time.Now().UTC()) {
		return fmt.Errorf("receipt_date cannot be in the future")
	}
	return nil
}

// SubmitMerchantProofUsecase permet au marchand de soumettre sa preuve de réception COD
type SubmitMerchantProofUsecase struct {
	codProofRepo repository.CODProofRepository
	orderRepo    repository.OrderRepository
	txManager    repository.TxManager
}

// NewSubmitMerchantProofUsecase crée une nouvelle instance
func NewSubmitMerchantProofUsecase(
	codProofRepo repository.CODProofRepository,
	orderRepo repository.OrderRepository,
	txManager repository.TxManager,
) *SubmitMerchantProofUsecase {
	return &SubmitMerchantProofUsecase{
		codProofRepo: codProofRepo,
		orderRepo:    orderRepo,
		txManager:    txManager,
	}
}

// Execute soumet la preuve marchand
func (uc *SubmitMerchantProofUsecase) Execute(ctx context.Context, req *SubmitMerchantProofRequest) (*SubmitMerchantProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid submit merchant proof request")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Récupérer la commande
	order, err := uc.orderRepo.WithTX(tx).FindByID(ctx, req.OrderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find order")
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 5. Vérifier que la commande est bien en cash à la livraison
	if !order.IsCashOnDelivery() {
		return nil, fmt.Errorf("order is not a cash-on-delivery order")
	}

	// 6. Récupérer la preuve COD
	proof, err := uc.codProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find COD proof")
		return nil, fmt.Errorf("COD proof not found for this order: %w", err)
	}

	// 7. Vérifier que la preuve appartient au bon shop (multi-tenant)
	if proof.ShopID != shopID {
		return nil, fmt.Errorf("access denied: proof does not belong to tenant shop")
	}

	// 8. Vérifier que la preuve est dans un état valide pour soumission marchand
	if proof.Status != entity.CODProofPendingProofs && proof.Status != entity.CODProofClientProofSent {
		return nil, fmt.Errorf("cannot submit merchant proof with status: %s", proof.Status)
	}

	// 9. Vérifier que le délai n'est pas dépassé
	if proof.IsPastDeadline() {
		return nil, fmt.Errorf("deadline exceeded for submitting proof")
	}

	// 10. Soumettre la preuve marchand
	if err := proof.SubmitMerchantProof(
		req.ProofURL,
		req.AmountCents,
		req.ReceiptDate,
		stringValue(req.Notes),
	); err != nil {
		logger.Error().Err(err).Msg("Failed to submit merchant proof")
		return nil, fmt.Errorf("failed to submit merchant proof: %w", err)
	}

	// 11. Mettre à jour la preuve dans la base
	if err := uc.codProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		logger.Error().Err(err).Msg("Failed to update COD proof")
		return nil, fmt.Errorf("failed to update COD proof: %w", err)
	}

	// 12. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 13. Logger le succès
	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Str("proof_id", proof.ID).
		Int64("amount_cents", req.AmountCents).
		Str("status", string(proof.Status)).
		Bool("has_client_proof", proof.HasClientProof()).
		Bool("is_coherent", proof.IsCoherent()).
		Msg("Merchant proof submitted successfully")

	// 14. Construire la réponse
	response := &SubmitMerchantProofResponse{
		ProofID:             proof.ID,
		OrderID:             proof.OrderID,
		ShopID:              proof.ShopID,
		CustomerID:          proof.CustomerID,
		Status:              proof.Status,
		MerchantProofURL:    req.ProofURL,
		MerchantAmountCents: req.AmountCents,
		MerchantReceiptDate: req.ReceiptDate,
		MerchantSubmittedAt: *proof.MerchantSubmittedAt,
		HasClientProof:      proof.HasClientProof(),
		CommissionCents:     proof.CommissionCents,
		Deadline:            proof.ProofDeadline(),
		DaysRemaining:       proof.DaysUntilDeadline(),
	}

	// Ajouter les infos client si disponibles
	if proof.HasClientProof() {
		response.ClientAmountCents = proof.ClientPaymentAmountCents
		response.ClientPaymentDate = proof.ClientPaymentDate
	}

	// Ajouter la cohérence si les deux preuves sont présentes
	if proof.IsComplete() {
		response.AmountsMatch = proof.AmountsMatch
		response.DatesMatch = proof.DatesMatch
		response.IsCoherent = proof.IsCoherent()
	}

	return response, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// SubmitMerchantProofWithAmounts soumet la preuve en vérifiant le montant de la commande
// SubmitMerchantProofWithAmounts soumet la preuve en utilisant le montant de la commande
func (uc *SubmitMerchantProofUsecase) SubmitMerchantProofWithAmounts(
	ctx context.Context,
	orderID string,
	proofURL string,
	receiptDate time.Time,
	notes *string,
) (*SubmitMerchantProofResponse, error) {
	//logger := zerolog.Ctx(ctx)

	// 1. Récupérer la commande pour connaître le montant
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 2. Construire la requête avec le montant de la commande
	req := &SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    proofURL,
		AmountCents: order.TotalCents,
		ReceiptDate: receiptDate,
		Notes:       notes,
	}

	// 3. Logger si le montant diffère (code mort supprimé - toujours faux)
	// Note: Cette condition était toujours fausse car req.AmountCents = order.TotalCents
	// Pour tester un montant différent, utiliser directement Execute() avec une requête manuelle

	return uc.Execute(ctx, req)
}
