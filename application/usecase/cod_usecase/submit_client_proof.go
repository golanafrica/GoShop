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
// SUBMIT CLIENT PROOF USECASE
// ============================================================

// SubmitClientProofRequest représente la requête pour soumettre une preuve client
type SubmitClientProofRequest struct {
	// Identifiants
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`

	// Preuve de paiement
	ProofURL string `json:"proof_url"`

	// Montant payé au marchand (en centimes)
	AmountCents int64 `json:"amount_cents"`

	// Date du paiement
	PaymentDate time.Time `json:"payment_date"`

	// Optionnels
	ReceiptNumber *string `json:"receipt_number,omitempty"`
	Notes         *string `json:"notes,omitempty"`
}

// SubmitClientProofResponse représente la réponse après soumission
type SubmitClientProofResponse struct {
	// Preuve mise à jour
	ProofID    string                `json:"proof_id"`
	OrderID    string                `json:"order_id"`
	ShopID     string                `json:"shop_id"`
	CustomerID string                `json:"customer_id"`
	Status     entity.CODProofStatus `json:"status"`

	// Preuve client
	ClientProofURL    string    `json:"client_proof_url"`
	ClientAmountCents int64     `json:"client_amount_cents"`
	ClientPaymentDate time.Time `json:"client_payment_date"`
	ClientSubmittedAt time.Time `json:"client_submitted_at"`

	// Preuve marchand (si déjà soumise)
	HasMerchantProof    bool       `json:"has_merchant_proof"`
	MerchantAmountCents *int64     `json:"merchant_amount_cents,omitempty"`
	MerchantReceiptDate *time.Time `json:"merchant_receipt_date,omitempty"`

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
func (r *SubmitClientProofRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if r.ProofURL == "" {
		return fmt.Errorf("proof_url is required")
	}
	if r.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	if r.PaymentDate.IsZero() {
		return fmt.Errorf("payment_date is required")
	}
	if r.PaymentDate.After(time.Now().UTC()) {
		return fmt.Errorf("payment_date cannot be in the future")
	}
	return nil
}

// SubmitClientProofUsecase permet au client de soumettre sa preuve de paiement COD
type SubmitClientProofUsecase struct {
	codProofRepo repository.CODProofRepository
	orderRepo    repository.OrderRepository
	txManager    repository.TxManager
}

// NewSubmitClientProofUsecase crée une nouvelle instance
func NewSubmitClientProofUsecase(
	codProofRepo repository.CODProofRepository,
	orderRepo repository.OrderRepository,
	txManager repository.TxManager,
) *SubmitClientProofUsecase {
	return &SubmitClientProofUsecase{
		codProofRepo: codProofRepo,
		orderRepo:    orderRepo,
		txManager:    txManager,
	}
}

// Execute soumet la preuve client
func (uc *SubmitClientProofUsecase) Execute(ctx context.Context, req *SubmitClientProofRequest) (*SubmitClientProofResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid submit client proof request")
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

	// 8. Vérifier que le client est bien celui de la commande
	if proof.CustomerID != req.CustomerID {
		return nil, fmt.Errorf("access denied: customer does not match order customer")
	}

	// 9. Vérifier que la preuve est dans un état valide pour soumission client
	if proof.Status != entity.CODProofPendingProofs && proof.Status != entity.CODProofMerchantProofSent {
		return nil, fmt.Errorf("cannot submit client proof with status: %s", proof.Status)
	}

	// 10. Vérifier que le délai n'est pas dépassé
	if proof.IsPastDeadline() {
		return nil, fmt.Errorf("deadline exceeded for submitting proof")
	}

	// 11. Soumettre la preuve client
	if err := proof.SubmitClientProof(
		req.ProofURL,
		req.AmountCents,
		req.PaymentDate,
		stringValue(req.ReceiptNumber),
		stringValue(req.Notes),
	); err != nil {
		logger.Error().Err(err).Msg("Failed to submit client proof")
		return nil, fmt.Errorf("failed to submit client proof: %w", err)
	}

	// 12. Mettre à jour la preuve dans la base
	if err := uc.codProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		logger.Error().Err(err).Msg("Failed to update COD proof")
		return nil, fmt.Errorf("failed to update COD proof: %w", err)
	}

	// 13. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 14. Logger le succès
	logger.Info().
		Str("order_id", req.OrderID).
		Str("customer_id", req.CustomerID).
		Str("proof_id", proof.ID).
		Int64("amount_cents", req.AmountCents).
		Str("status", string(proof.Status)).
		Bool("has_merchant_proof", proof.HasMerchantProof()).
		Msg("Client proof submitted successfully")

	// 15. Construire la réponse
	response := &SubmitClientProofResponse{
		ProofID:           proof.ID,
		OrderID:           proof.OrderID,
		ShopID:            proof.ShopID,
		CustomerID:        proof.CustomerID,
		Status:            proof.Status,
		ClientProofURL:    req.ProofURL,
		ClientAmountCents: req.AmountCents,
		ClientPaymentDate: req.PaymentDate,
		ClientSubmittedAt: *proof.ClientSubmittedAt,
		HasMerchantProof:  proof.HasMerchantProof(),
		CommissionCents:   proof.CommissionCents,
		Deadline:          proof.ProofDeadline(),
		DaysRemaining:     proof.DaysUntilDeadline(),
	}

	// Ajouter les infos marchand si disponibles
	if proof.HasMerchantProof() {
		response.MerchantAmountCents = proof.MerchantReceivedAmountCents
		response.MerchantReceiptDate = proof.MerchantReceiptDate
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
// HELPERS
// ============================================================

// stringValue retourne la valeur d'un pointeur string ou "" si nil
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// SubmitClientProofWithAmounts soumet la preuve en vérifiant le montant de la commande
// SubmitClientProofWithAmounts soumet la preuve en utilisant le montant de la commande
// Si declaredAmountCents > 0, utilise ce montant au lieu de order.TotalCents
func (uc *SubmitClientProofUsecase) SubmitClientProofWithAmounts(
	ctx context.Context,
	orderID string,
	customerID string,
	proofURL string,
	paymentDate time.Time,
	receiptNumber, notes *string,
) (*SubmitClientProofResponse, error) {
	//logger := zerolog.Ctx(ctx)

	// 1. Récupérer la commande pour connaître le montant
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 2. Construire la requête avec le montant de la commande
	req := &SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      proofURL,
		AmountCents:   order.TotalCents,
		PaymentDate:   paymentDate,
		ReceiptNumber: receiptNumber,
		Notes:         notes,
	}

	// 3. Logger si le montant diffère (code mort supprimé - toujours faux)
	// Note: Cette condition était toujours fausse car req.AmountCents = order.TotalCents
	// Pour tester un montant différent, utiliser directement Execute() avec une requête manuelle

	return uc.Execute(ctx, req)
}
