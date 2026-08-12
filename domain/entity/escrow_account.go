package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS ESCROW ACCOUNT
// ============================================================

// EscrowAccountStatus représente le statut d'un compte séquestre
type EscrowAccountStatus string

const (
	EscrowAccountFundsHeld      EscrowAccountStatus = "funds_held"      // Fonds bloqués
	EscrowAccountPartialRelease EscrowAccountStatus = "partial_release" // Déblocage partiel (crédit)
	EscrowAccountFullyReleased  EscrowAccountStatus = "released"        // 🆕 v4.8.3 : Déblocage total (aligné contrainte SQL)
	EscrowAccountRefunded       EscrowAccountStatus = "refunded"        // Remboursé au client
	EscrowAccountDisputed       EscrowAccountStatus = "disputed"        // En litige
)

// IsValid vérifie si le statut est valide
func (s EscrowAccountStatus) IsValid() bool {
	switch s {
	case EscrowAccountFundsHeld, EscrowAccountPartialRelease,
		EscrowAccountFullyReleased, EscrowAccountRefunded,
		EscrowAccountDisputed:
		return true
	}
	return false
}

// IsTerminal vérifie si le statut est terminal
func (s EscrowAccountStatus) IsTerminal() bool {
	return s == EscrowAccountFullyReleased || s == EscrowAccountRefunded
}

// EscrowSourceType représente le type de source des fonds
type EscrowSourceType string

const (
	EscrowSourceOrder          EscrowSourceType = "order"           // Achat cash
	EscrowSourceCreditContract EscrowSourceType = "credit_contract" // Crédit tempérament
	EscrowSourceTontineGroup   EscrowSourceType = "tontine_group"   // Tontine
)

// IsValid vérifie si le type de source est valide
func (t EscrowSourceType) IsValid() bool {
	switch t {
	case EscrowSourceOrder, EscrowSourceCreditContract, EscrowSourceTontineGroup:
		return true
	}
	return false
}

// ============================================================
// ESCROW ACCOUNT (Compte séquestre unifié)
// ============================================================

// EscrowAccount représente un compte séquestre unifié
// Gère les fonds bloqués pour TOUS les modes d'achat
type EscrowAccount struct {
	ID string `json:"id" db:"id"`

	// Référence polymorphique (une seule doit être non-nulle)
	OrderID          *string `json:"order_id,omitempty" db:"order_id"`
	CreditContractID *string `json:"credit_contract_id,omitempty" db:"credit_contract_id"`
	TontineGroupID   *string `json:"tontine_group_id,omitempty" db:"tontine_group_id"`

	// Type de source
	SourceType EscrowSourceType `json:"source_type" db:"source_type"`

	// Montants (en centimes)
	TotalAmountCents    int64 `json:"total_amount_cents" db:"total_amount_cents"`
	ReleasedAmountCents int64 `json:"released_amount_cents" db:"released_amount_cents"`
	CommissionCents     int64 `json:"commission_cents" db:"commission_cents"`

	// Statut
	Status EscrowAccountStatus `json:"status" db:"status"`

	// Timestamps
	FundsHeldAt     time.Time  `json:"funds_held_at" db:"funds_held_at"`
	FundsReleasedAt *time.Time `json:"funds_released_at,omitempty" db:"funds_released_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewEscrowAccountForOrder crée un compte séquestre pour une commande (achat cash)
func NewEscrowAccountForOrder(orderID string, totalAmountCents int64, commissionBps int) (*EscrowAccount, error) {
	if orderID == "" {
		return nil, errors.New("order_id is required")
	}
	if totalAmountCents <= 0 {
		return nil, errors.New("total_amount_cents must be positive")
	}

	// 🆕 Utilise CalculateCommission de tontine.go (déjà défini)
	commissionCents := CalculateCommission(totalAmountCents, commissionBps)
	now := time.Now().UTC()

	return &EscrowAccount{
		OrderID:          &orderID,
		SourceType:       EscrowSourceOrder,
		TotalAmountCents: totalAmountCents,
		CommissionCents:  commissionCents,
		Status:           EscrowAccountFundsHeld,
		FundsHeldAt:      now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// NewEscrowAccountForCreditContract crée un compte séquestre pour un contrat de crédit
func NewEscrowAccountForCreditContract(contractID string, totalAmountCents int64, commissionBps int) (*EscrowAccount, error) {
	if contractID == "" {
		return nil, errors.New("credit_contract_id is required")
	}
	if totalAmountCents <= 0 {
		return nil, errors.New("total_amount_cents must be positive")
	}

	commissionCents := CalculateCommission(totalAmountCents, commissionBps)
	now := time.Now().UTC()

	return &EscrowAccount{
		CreditContractID: &contractID,
		SourceType:       EscrowSourceCreditContract,
		TotalAmountCents: totalAmountCents,
		CommissionCents:  commissionCents,
		Status:           EscrowAccountFundsHeld,
		FundsHeldAt:      now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// NewEscrowAccountForTontineGroup crée un compte séquestre pour un groupe tontine
func NewEscrowAccountForTontineGroup(groupID string, totalAmountCents int64, commissionBps int) (*EscrowAccount, error) {
	if groupID == "" {
		return nil, errors.New("tontine_group_id is required")
	}
	if totalAmountCents <= 0 {
		return nil, errors.New("total_amount_cents must be positive")
	}

	commissionCents := CalculateCommission(totalAmountCents, commissionBps)
	now := time.Now().UTC()

	return &EscrowAccount{
		TontineGroupID:   &groupID,
		SourceType:       EscrowSourceTontineGroup,
		TotalAmountCents: totalAmountCents,
		CommissionCents:  commissionCents,
		Status:           EscrowAccountFundsHeld,
		FundsHeldAt:      now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// ============================================================
// MÉTHODES DE TRANSITION
// ============================================================

// ReleaseFunds marque les fonds comme débloqués (totalement)
func (a *EscrowAccount) ReleaseFunds() error {
	if a.Status == EscrowAccountFullyReleased {
		return errors.New("funds already fully released")
	}
	if a.Status == EscrowAccountRefunded {
		return errors.New("funds already refunded")
	}

	now := time.Now().UTC()
	a.Status = EscrowAccountFullyReleased
	a.ReleasedAmountCents = a.TotalAmountCents - a.CommissionCents
	a.FundsReleasedAt = &now
	a.UpdatedAt = now
	return nil
}

// PartialRelease libère partiellement les fonds (pour crédit tempérament)
func (a *EscrowAccount) PartialRelease(amountCents int64) error {
	if a.Status != EscrowAccountFundsHeld && a.Status != EscrowAccountPartialRelease {
		return fmt.Errorf("cannot partial release with status: %s", a.Status)
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}

	// Vérifier qu'on ne dépasse pas le total disponible
	maxReleaseable := a.TotalAmountCents - a.ReleasedAmountCents - a.CommissionCents
	if amountCents > maxReleaseable {
		return fmt.Errorf("amount exceeds available funds: %d > %d", amountCents, maxReleaseable)
	}

	now := time.Now().UTC()
	a.ReleasedAmountCents += amountCents
	a.Status = EscrowAccountPartialRelease
	a.UpdatedAt = now

	// Si tout est libéré, passer à fully_released
	if a.ReleasedAmountCents >= (a.TotalAmountCents - a.CommissionCents) {
		a.Status = EscrowAccountFullyReleased
		a.FundsReleasedAt = &now
	}

	return nil
}

// Refund marque les fonds comme remboursés au client
func (a *EscrowAccount) Refund() error {
	if a.Status.IsTerminal() {
		return fmt.Errorf("cannot refund with terminal status: %s", a.Status)
	}

	now := time.Now().UTC()
	a.Status = EscrowAccountRefunded
	a.ReleasedAmountCents = 0
	a.CommissionCents = 0 // Pas de commission si remboursé
	a.FundsReleasedAt = &now
	a.UpdatedAt = now
	return nil
}

// Dispute marque le compte comme en litige
func (a *EscrowAccount) Dispute() error {
	if a.Status.IsTerminal() {
		return fmt.Errorf("cannot dispute with terminal status: %s", a.Status)
	}

	a.Status = EscrowAccountDisputed
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// ResolveDispute résout un litige (appelé après arbitrage)
func (a *EscrowAccount) ResolveDispute(releaseToMerchant bool) error {
	if a.Status != EscrowAccountDisputed {
		return fmt.Errorf("cannot resolve dispute with status: %s", a.Status)
	}

	if releaseToMerchant {
		return a.ReleaseFunds()
	}
	return a.Refund()
}

// ============================================================
// MÉTHODES DE REQUÊTE
// ============================================================

// IsFundsHeld vérifie si les fonds sont bloqués
func (a *EscrowAccount) IsFundsHeld() bool {
	return a.Status == EscrowAccountFundsHeld
}

// IsFullyReleased vérifie si les fonds sont totalement débloqués
func (a *EscrowAccount) IsFullyReleased() bool {
	return a.Status == EscrowAccountFullyReleased
}

// IsRefunded vérifie si les fonds sont remboursés
func (a *EscrowAccount) IsRefunded() bool {
	return a.Status == EscrowAccountRefunded
}

// IsDisputed vérifie si le compte est en litige
func (a *EscrowAccount) IsDisputed() bool {
	return a.Status == EscrowAccountDisputed
}

// GetReferenceID retourne l'ID de référence (order, contract, ou group)
func (a *EscrowAccount) GetReferenceID() string {
	if a.OrderID != nil {
		return *a.OrderID
	}
	if a.CreditContractID != nil {
		return *a.CreditContractID
	}
	if a.TontineGroupID != nil {
		return *a.TontineGroupID
	}
	return ""
}

// GetMerchantAmount retourne le montant net pour le marchand
func (a *EscrowAccount) GetMerchantAmount() int64 {
	return a.TotalAmountCents - a.CommissionCents
}

// CanReleaseToMerchant vérifie si les fonds peuvent être libérés vers le wallet marchand
func (a *EscrowAccount) CanReleaseToMerchant() bool {
	return a.Status == EscrowAccountFullyReleased || a.Status == EscrowAccountPartialRelease
}

// IsBlockedFromRelease vérifie si les fonds sont bloqués et ne peuvent pas être libérés
func (a *EscrowAccount) IsBlockedFromRelease() bool {
	return a.Status == EscrowAccountFundsHeld || a.Status == EscrowAccountDisputed
}

// GetReleasedPercentage retourne le pourcentage de fonds libérés
func (a *EscrowAccount) GetReleasedPercentage() float64 {
	if a.TotalAmountCents == 0 {
		return 0
	}
	merchantAmount := a.GetMerchantAmount()
	if merchantAmount == 0 {
		return 0
	}
	return float64(a.ReleasedAmountCents) / float64(merchantAmount) * 100
}

// ============================================================
// VALIDATION
// ============================================================

// Validate valide le compte séquestre
func (a *EscrowAccount) Validate() error {
	// Vérifier qu'une seule référence est présente
	refCount := 0
	if a.OrderID != nil {
		refCount++
	}
	if a.CreditContractID != nil {
		refCount++
	}
	if a.TontineGroupID != nil {
		refCount++
	}

	if refCount != 1 {
		return fmt.Errorf("exactly one reference must be set, got %d", refCount)
	}

	// Vérifier la cohérence source_type / référence
	switch a.SourceType {
	case EscrowSourceOrder:
		if a.OrderID == nil {
			return errors.New("order_id must be set for order source type")
		}
	case EscrowSourceCreditContract:
		if a.CreditContractID == nil {
			return errors.New("credit_contract_id must be set for credit_contract source type")
		}
	case EscrowSourceTontineGroup:
		if a.TontineGroupID == nil {
			return errors.New("tontine_group_id must be set for tontine_group source type")
		}
	default:
		return fmt.Errorf("invalid source type: %s", a.SourceType)
	}

	// Vérifier montants
	if a.TotalAmountCents <= 0 {
		return errors.New("total_amount_cents must be positive")
	}
	if a.CommissionCents < 0 {
		return errors.New("commission_cents cannot be negative")
	}
	if a.ReleasedAmountCents < 0 {
		return errors.New("released_amount_cents cannot be negative")
	}
	if a.ReleasedAmountCents > a.TotalAmountCents {
		return errors.New("released_amount_cents cannot exceed total_amount_cents")
	}

	// Vérifier statut
	if !a.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", a.Status)
	}

	return nil
}
