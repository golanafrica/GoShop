package creditusecase

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
// APPROVE CREDIT USECASE
// ============================================================

// ApproveCreditRequest représente la requête pour approuver un crédit
type ApproveCreditRequest struct {
	ApplicationID string `json:"application_id"`
	ReviewedBy    string `json:"reviewed_by"` // user_id du marchand
}

// ApproveCreditResponse représente la réponse après approbation
type ApproveCreditResponse struct {
	// Demande approuvée
	ApplicationID string                         `json:"application_id"`
	CustomerID    string                         `json:"customer_id"`
	ProductID     string                         `json:"product_id"`
	ShopID        string                         `json:"shop_id"`
	Status        entity.CreditApplicationStatus `json:"status"`

	// Contrat créé
	ContractID        string    `json:"contract_id"`
	ContractStatus    string    `json:"contract_status"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	InstallmentsCount int       `json:"installments_count"`

	// Montants
	DownPaymentCents    int64 `json:"down_payment_cents"`
	FinancedAmountCents int64 `json:"financed_amount_cents"`
	InterestAmountCents int64 `json:"interest_amount_cents"`
	TotalAmountCents    int64 `json:"total_amount_cents"`
	MonthlyPaymentCents int64 `json:"monthly_payment_cents"`

	// Message
	Message string `json:"message"`
}

// Validate valide la requête
func (r *ApproveCreditRequest) Validate() error {
	if r.ApplicationID == "" {
		return fmt.Errorf("application_id is required")
	}
	if r.ReviewedBy == "" {
		return fmt.Errorf("reviewed_by is required")
	}
	return nil
}

// ApproveCreditUsecase permet à un marchand d'approuver une demande de crédit
type ApproveCreditUsecase struct {
	applicationRepo repository.CreditApplicationRepository
	contractRepo    repository.CreditContractRepository
	installmentRepo repository.CreditInstallmentRepository
	scoreRepo       repository.CreditScoreRepository
	txManager       repository.TxManager
}

// NewApproveCreditUsecase crée une nouvelle instance
func NewApproveCreditUsecase(
	applicationRepo repository.CreditApplicationRepository,
	contractRepo repository.CreditContractRepository,
	installmentRepo repository.CreditInstallmentRepository,
	scoreRepo repository.CreditScoreRepository,
	txManager repository.TxManager,
) *ApproveCreditUsecase {
	return &ApproveCreditUsecase{
		applicationRepo: applicationRepo,
		contractRepo:    contractRepo,
		installmentRepo: installmentRepo,
		scoreRepo:       scoreRepo,
		txManager:       txManager,
	}
}

// Execute approuve la demande et crée le contrat + échéances
func (uc *ApproveCreditUsecase) Execute(ctx context.Context, req *ApproveCreditRequest) (*ApproveCreditResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid approve credit request")
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

	// 4. Récupérer la demande
	application, err := uc.applicationRepo.WithTX(tx).FindByID(ctx, req.ApplicationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find application")
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// 5. Vérifier que la demande appartient au shop (multi-tenant)
	if application.ShopID != shopID {
		return nil, fmt.Errorf("access denied: application does not belong to tenant shop")
	}

	// 6. Vérifier que la demande est en attente
	if application.Status != entity.CreditApplicationPending {
		return nil, fmt.Errorf("application is not in pending status (current: %s)", application.Status)
	}

	// 7. Approuver la demande
	if err := application.Approve(req.ReviewedBy); err != nil {
		logger.Error().Err(err).Msg("Failed to approve application")
		return nil, fmt.Errorf("failed to approve application: %w", err)
	}

	// 8. Mettre à jour la demande
	if err := uc.applicationRepo.WithTX(tx).Update(ctx, application); err != nil {
		logger.Error().Err(err).Msg("Failed to update application")
		return nil, fmt.Errorf("failed to update application: %w", err)
	}

	// 9. Créer le contrat de crédit
	contract, err := entity.NewCreditContract(application)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create credit contract entity")
		return nil, fmt.Errorf("failed to create credit contract: %w", err)
	}

	// Forcer l'ID
	contract.ID = uuid.New().String()

	// 10. Sauvegarder le contrat
	if err := uc.contractRepo.WithTX(tx).Create(ctx, contract); err != nil {
		logger.Error().Err(err).Msg("Failed to save credit contract")
		return nil, fmt.Errorf("failed to save credit contract: %w", err)
	}

	// 11. Générer les échéances mensuelles
	installments := generateInstallments(contract)

	// 12. Sauvegarder les échéances en batch
	if err := uc.installmentRepo.WithTX(tx).CreateBatch(ctx, installments); err != nil {
		logger.Error().Err(err).Msg("Failed to save installments")
		return nil, fmt.Errorf("failed to save installments: %w", err)
	}

	// 13. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 14. Logger le succès
	logger.Info().
		Str("application_id", req.ApplicationID).
		Str("contract_id", contract.ID).
		Str("customer_id", application.CustomerID).
		Str("shop_id", shopID).
		Int("installments_count", len(installments)).
		Int64("monthly_payment_cents", contract.MonthlyPaymentCents).
		Msg("Credit application approved and contract created")

	// 15. Construire la réponse
	return &ApproveCreditResponse{
		ApplicationID:       application.ID,
		CustomerID:          application.CustomerID,
		ProductID:           application.ProductID,
		ShopID:              application.ShopID,
		Status:              application.Status,
		ContractID:          contract.ID,
		ContractStatus:      string(contract.Status),
		StartDate:           contract.StartDate,
		EndDate:             contract.EndDate,
		InstallmentsCount:   len(installments),
		DownPaymentCents:    contract.DownPaymentCents,
		FinancedAmountCents: contract.FinancedAmountCents,
		InterestAmountCents: contract.InterestAmountCents,
		TotalAmountCents:    contract.TotalAmountCents,
		MonthlyPaymentCents: contract.MonthlyPaymentCents,
		Message:             "Credit application approved. Contract created with installments. Waiting for down payment.",
	}, nil
}

// ============================================================
// REJECT CREDIT USECASE
// ============================================================

// RejectCreditRequest représente la requête pour rejeter un crédit
type RejectCreditRequest struct {
	ApplicationID   string `json:"application_id"`
	ReviewedBy      string `json:"reviewed_by"`
	RejectionReason string `json:"rejection_reason"`
}

// RejectCreditResponse représente la réponse après rejet
type RejectCreditResponse struct {
	ApplicationID   string                         `json:"application_id"`
	CustomerID      string                         `json:"customer_id"`
	ProductID       string                         `json:"product_id"`
	ShopID          string                         `json:"shop_id"`
	Status          entity.CreditApplicationStatus `json:"status"`
	RejectionReason string                         `json:"rejection_reason"`
	ReviewedAt      time.Time                      `json:"reviewed_at"`
	Message         string                         `json:"message"`
}

// Validate valide la requête
func (r *RejectCreditRequest) Validate() error {
	if r.ApplicationID == "" {
		return fmt.Errorf("application_id is required")
	}
	if r.ReviewedBy == "" {
		return fmt.Errorf("reviewed_by is required")
	}
	if r.RejectionReason == "" {
		return fmt.Errorf("rejection_reason is required")
	}
	return nil
}

// RejectCreditUsecase permet à un marchand de rejeter une demande de crédit
type RejectCreditUsecase struct {
	applicationRepo repository.CreditApplicationRepository
	scoreRepo       repository.CreditScoreRepository
	txManager       repository.TxManager
}

// NewRejectCreditUsecase crée une nouvelle instance
func NewRejectCreditUsecase(
	applicationRepo repository.CreditApplicationRepository,
	scoreRepo repository.CreditScoreRepository,
	txManager repository.TxManager,
) *RejectCreditUsecase {
	return &RejectCreditUsecase{
		applicationRepo: applicationRepo,
		scoreRepo:       scoreRepo,
		txManager:       txManager,
	}
}

// Execute rejette la demande
func (uc *RejectCreditUsecase) Execute(ctx context.Context, req *RejectCreditRequest) (*RejectCreditResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid reject credit request")
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

	// 4. Récupérer la demande
	application, err := uc.applicationRepo.WithTX(tx).FindByID(ctx, req.ApplicationID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find application")
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// 5. Vérifier que la demande appartient au shop
	if application.ShopID != shopID {
		return nil, fmt.Errorf("access denied: application does not belong to tenant shop")
	}

	// 6. Vérifier que la demande est en attente
	if application.Status != entity.CreditApplicationPending {
		return nil, fmt.Errorf("application is not in pending status (current: %s)", application.Status)
	}

	// 7. Rejeter la demande
	if err := application.Reject(req.ReviewedBy, req.RejectionReason); err != nil {
		logger.Error().Err(err).Msg("Failed to reject application")
		return nil, fmt.Errorf("failed to reject application: %w", err)
	}

	// 8. Mettre à jour la demande
	if err := uc.applicationRepo.WithTX(tx).Update(ctx, application); err != nil {
		logger.Error().Err(err).Msg("Failed to update application")
		return nil, fmt.Errorf("failed to update application: %w", err)
	}

	// 9. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 10. Logger le succès
	logger.Info().
		Str("application_id", req.ApplicationID).
		Str("customer_id", application.CustomerID).
		Str("shop_id", shopID).
		Str("reason", req.RejectionReason).
		Msg("Credit application rejected")

	// 11. Construire la réponse
	return &RejectCreditResponse{
		ApplicationID:   application.ID,
		CustomerID:      application.CustomerID,
		ProductID:       application.ProductID,
		ShopID:          application.ShopID,
		Status:          application.Status,
		RejectionReason: *application.RejectionReason,
		ReviewedAt:      *application.ReviewedAt,
		Message:         "Credit application rejected",
	}, nil
}

// ============================================================
// HELPERS
// ============================================================

// generateInstallments génère les échéances mensuelles pour un contrat
func generateInstallments(contract *entity.CreditContract) []*entity.CreditInstallment {
	installments := make([]*entity.CreditInstallment, 0, contract.DurationMonths)

	// Date de début = date de création du contrat
	startDate := contract.StartDate

	for i := 1; i <= contract.DurationMonths; i++ {
		// Calculer la date d'échéance (mois par mois)
		dueDate := startDate.AddDate(0, i, 0)

		// Créer l'échéance
		installment := entity.NewCreditInstallment(
			contract.ID,
			i,
			dueDate,
			contract.MonthlyPaymentCents,
		)

		// Forcer l'ID
		installment.ID = uuid.New().String()

		installments = append(installments, installment)
	}

	return installments
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// GetContract retourne les détails d'un contrat
func (uc *ApproveCreditUsecase) GetContract(ctx context.Context, contractID string) (*entity.CreditContract, error) {
	return uc.contractRepo.FindByID(ctx, contractID)
}

// GetContractInstallments retourne les échéances d'un contrat
func (uc *ApproveCreditUsecase) GetContractInstallments(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error) {
	return uc.installmentRepo.FindByContractID(ctx, contractID)
}

// GetPendingApplications retourne les demandes en attente pour une boutique
func (uc *ApproveCreditUsecase) GetPendingApplications(ctx context.Context, shopID string) ([]*entity.CreditApplication, error) {
	return uc.applicationRepo.FindPendingByShopID(ctx, shopID)
}

// GetActiveContractsByShop retourne les contrats actifs d'une boutique
func (uc *ApproveCreditUsecase) GetActiveContractsByShop(ctx context.Context, shopID string) ([]*entity.CreditContract, error) {
	return uc.contractRepo.FindActiveByShopID(ctx, shopID)
}

// GetActiveContractsByCustomer retourne les contrats actifs d'un client
func (uc *ApproveCreditUsecase) GetActiveContractsByCustomer(ctx context.Context, customerID string) ([]*entity.CreditContract, error) {
	return uc.contractRepo.FindActiveByCustomerID(ctx, customerID)
}

// GetOverdueInstallments retourne les échéances en retard
func (uc *ApproveCreditUsecase) GetOverdueInstallments(ctx context.Context) ([]*entity.CreditInstallment, error) {
	return uc.installmentRepo.FindOverdue(ctx)
}

// GetOverdueInstallmentsByContract retourne les échéances en retard d'un contrat
func (uc *ApproveCreditUsecase) GetOverdueInstallmentsByContract(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error) {
	return uc.installmentRepo.FindOverdueByContractID(ctx, contractID)
}

// GetContractStats retourne les statistiques d'un contrat
func (uc *ApproveCreditUsecase) GetContractStats(ctx context.Context, contractID string) (map[string]interface{}, error) {
	// Récupérer le contrat
	contract, err := uc.contractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("contract not found: %w", err)
	}

	// Récupérer les échéances
	installments, err := uc.installmentRepo.FindByContractID(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to get installments: %w", err)
	}

	// Calculer les statistiques
	paidCount := 0
	pendingCount := 0
	lateCount := 0
	totalPaidCents := int64(0)
	totalPendingCents := int64(0)

	for _, inst := range installments {
		switch inst.Status {
		case entity.InstallmentPaid:
			paidCount++
			totalPaidCents += inst.AmountCents
		case entity.InstallmentPending:
			pendingCount++
			totalPendingCents += inst.AmountCents
		case entity.InstallmentLate:
			lateCount++
			totalPendingCents += inst.AmountCents
		}
	}

	return map[string]interface{}{
		"contract_id":          contract.ID,
		"status":               contract.Status,
		"total_installments":   len(installments),
		"paid_installments":    paidCount,
		"pending_installments": pendingCount,
		"late_installments":    lateCount,
		"total_paid_cents":     totalPaidCents,
		"total_pending_cents":  totalPendingCents,
		"completion_percent":   float64(paidCount) / float64(len(installments)) * 100,
	}, nil
}
