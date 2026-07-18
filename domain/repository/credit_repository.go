package repository

//go:generate mockgen -destination=../../mocks/repository/mock_credit_plan_repository.go -package=repository . CreditPlanRepository
//go:generate mockgen -destination=../../mocks/repository/mock_credit_application_repository.go -package=repository . CreditApplicationRepository
//go:generate mockgen -destination=../../mocks/repository/mock_credit_contract_repository.go -package=repository . CreditContractRepository
//go:generate mockgen -destination=../../mocks/repository/mock_credit_installment_repository.go -package=repository . CreditInstallmentRepository
//go:generate mockgen -destination=../../mocks/repository/mock_credit_score_repository.go -package=repository . CreditScoreRepository

import (
	"context"

	"Goshop/domain/entity"
)

// ============================================================
// 🆕 v3.5.0 : TYPES POUR LE DASHBOARD MARCHAND
// ============================================================

type MerchantCreditStats struct {
	ActiveContractsCount  int
	TotalFinancedCents    int64
	TotalOutstandingCents int64
}

type MerchantRecoveryStats struct {
	PaidCount           int
	LateCount           int
	OverdueAmountCents  int64
	OverdueCount        int
	RecoveryRatePercent float64
}

// ============================================================
// AJOUTS AUX INTERFACES EXISTANTES
// ============================================================

// Dans l'interface CreditContractRepository, ajoute :
// GetMerchantCreditStats(ctx context.Context, shopID string) (*MerchantCreditStats, error)

// Dans l'interface CreditInstallmentRepository, ajoute :
// GetMerchantRecoveryStats(ctx context.Context, shopID string) (*MerchantRecoveryStats, error)

// ============================================================
// CREDIT PLAN REPOSITORY
// ============================================================

// CreditPlanRepository définit les opérations sur les plans de crédit
type CreditPlanRepository interface {
	// Create crée un nouveau plan de crédit
	Create(ctx context.Context, plan *entity.CreditPlan) error

	// FindByProductID trouve un plan par produit
	FindByProductID(ctx context.Context, productID string) (*entity.CreditPlan, error)

	// FindByShopID retourne tous les plans d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditPlan, error)

	// FindEnabledByShopID retourne les plans activés d'une boutique
	FindEnabledByShopID(ctx context.Context, shopID string) ([]*entity.CreditPlan, error)

	// Update met à jour un plan
	Update(ctx context.Context, plan *entity.CreditPlan) error

	// Upsert crée ou met à jour un plan
	Upsert(ctx context.Context, plan *entity.CreditPlan) error

	// Delete supprime un plan
	Delete(ctx context.Context, productID string) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CreditPlanRepository
}

// ============================================================
// CREDIT APPLICATION REPOSITORY
// ============================================================

// CreditApplicationRepository définit les opérations sur les demandes de crédit
type CreditApplicationRepository interface {
	// Create crée une nouvelle demande
	Create(ctx context.Context, app *entity.CreditApplication) error

	// FindByID trouve une demande par ID
	FindByID(ctx context.Context, id string) (*entity.CreditApplication, error)

	// FindByCustomerID retourne les demandes d'un client
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditApplication, error)

	// FindPendingByShopID retourne les demandes en attente d'une boutique
	FindPendingByShopID(ctx context.Context, shopID string) ([]*entity.CreditApplication, error)

	// FindByCustomerAndProduct trouve une demande par client et produit
	FindByCustomerAndProduct(ctx context.Context, customerID, productID string) (*entity.CreditApplication, error)

	// CountByCustomerAndStatus compte les demandes d'un client par statut
	CountByCustomerAndStatus(ctx context.Context, customerID string, status entity.CreditApplicationStatus) (int, error)

	// Update met à jour une demande
	Update(ctx context.Context, app *entity.CreditApplication) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CreditApplicationRepository
}

// ============================================================
// CREDIT CONTRACT REPOSITORY
// ============================================================

// CreditContractRepository définit les opérations sur les contrats de crédit
type CreditContractRepository interface {
	// Create crée un nouveau contrat
	Create(ctx context.Context, contract *entity.CreditContract) error

	// FindByID trouve un contrat par ID
	FindByID(ctx context.Context, id string) (*entity.CreditContract, error)

	// FindByApplicationID trouve un contrat par demande
	FindByApplicationID(ctx context.Context, applicationID string) (*entity.CreditContract, error)

	// FindByCustomerID retourne les contrats d'un client
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditContract, error)

	// FindActiveByCustomerID retourne les contrats actifs d'un client
	FindActiveByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditContract, error)

	// FindByShopID retourne les contrats d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error)

	// FindActiveByShopID retourne les contrats actifs d'une boutique
	FindActiveByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error)

	// CountActiveByCustomerID compte les contrats actifs d'un client
	CountActiveByCustomerID(ctx context.Context, customerID string) (int, error)

	// Update met à jour un contrat
	Update(ctx context.Context, contract *entity.CreditContract) error

	GetMerchantCreditStats(ctx context.Context, shopID string) (*MerchantCreditStats, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CreditContractRepository
}

// ============================================================
// CREDIT INSTALLMENT REPOSITORY
// ============================================================

// CreditInstallmentRepository définit les opérations sur les échéances
type CreditInstallmentRepository interface {
	// Create crée une nouvelle échéance
	Create(ctx context.Context, installment *entity.CreditInstallment) error

	// CreateBatch crée plusieurs échéances en lot
	CreateBatch(ctx context.Context, installments []*entity.CreditInstallment) error

	// FindByID trouve une échéance par ID
	FindByID(ctx context.Context, id string) (*entity.CreditInstallment, error)

	// FindByContractID retourne les échéances d'un contrat
	FindByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error)

	// FindByContractIDAndNumber trouve une échéance par contrat et numéro
	FindByContractIDAndNumber(ctx context.Context, contractID string, number int) (*entity.CreditInstallment, error)

	// FindPendingByContractID retourne les échéances en attente d'un contrat
	FindPendingByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error)

	// FindOverdue retourne les échéances échues (due_date < now)
	FindOverdue(ctx context.Context) ([]*entity.CreditInstallment, error)

	// FindOverdueByContractID retourne les échéances échues d'un contrat
	FindOverdueByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error)

	// CountPendingByContractID compte les échéances en attente d'un contrat
	CountPendingByContractID(ctx context.Context, contractID string) (int, error)

	// SumPendingAmountByContractID somme des montants en attente
	SumPendingAmountByContractID(ctx context.Context, contractID string) (int64, error)

	// Update met à jour une échéance
	Update(ctx context.Context, installment *entity.CreditInstallment) error

	GetMerchantRecoveryStats(ctx context.Context, shopID string) (*MerchantRecoveryStats, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CreditInstallmentRepository

	// 🆕 v3.4.0 : Méthodes pour le scheduler de commissions
	// FindPaidWithoutCommission récupère les échéances payées sans commission collectée
	// Remplit aussi le champ ShopID via JOIN avec credit_contracts
	FindPaidWithoutCommission(ctx context.Context, limit int) ([]*entity.CreditInstallment, error)

	// UpdateCreditCommissionStatus met à jour le statut de commission d'une échéance
	UpdateCreditCommissionStatus(ctx context.Context, installmentID string, status string, commissionCents int64, batchID *string) error

	// FindDueInstallments récupère les échéances dues (pending ou late) dont la date d'échéance est <= aujourd'hui
	FindDueInstallments(ctx context.Context, limit int) ([]*entity.CreditInstallment, error)
}

// ============================================================
// CREDIT SCORE REPOSITORY
// ============================================================

// CreditScoreRepository définit les opérations sur les scores de crédit
type CreditScoreRepository interface {
	// Create crée un nouveau score
	Create(ctx context.Context, score *entity.CreditScore) error

	// FindByCustomerAndShop trouve un score par client et boutique
	FindByCustomerAndShop(ctx context.Context, customerID, shopID string) (*entity.CreditScore, error)

	// FindByCustomerID retourne tous les scores d'un client (multi-boutique)
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditScore, error)

	// FindByShopID retourne tous les scores d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditScore, error)

	// FindLowScores retourne les scores faibles (< seuil)
	FindLowScores(ctx context.Context, shopID string, minScore int) ([]*entity.CreditScore, error)

	// Update met à jour un score
	Update(ctx context.Context, score *entity.CreditScore) error

	// Upsert crée ou met à jour un score
	Upsert(ctx context.Context, score *entity.CreditScore) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CreditScoreRepository
}
