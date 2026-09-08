package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// INSTALLMENT PLAN (Configuration Marchand)
// ============================================================

// InstallmentPlan représente la configuration du paiement en tranches pour un produit.
// C'est le marchand qui décide : "Je vends ce produit en 3 fois, toutes les 15 jours".
type InstallmentPlan struct {
	ID         string `json:"id" db:"id"`
	ProductID  string `json:"product_id" db:"product_id"`
	ShopID     string `json:"shop_id" db:"shop_id"`
	NbTranches int    `json:"nb_tranches" db:"nb_tranches"` // Ex: 2, 3, 4, 5
	DelaiJours int    `json:"delai_jours" db:"delai_jours"` // Ex: 15, 30 jours entre chaque tranche

	// 🆕 v5.1.0 : Délai dynamique basé sur la zone de livraison
	DeliveryZoneID              *string `json:"delivery_zone_id,omitempty" db:"delivery_zone_id"`
	InstallmentReleaseDelayDays int     `json:"installment_release_delay_days" db:"installment_release_delay_days"`

	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewInstallmentPlan crée un nouveau plan avec validation stricte.
func NewInstallmentPlan(productID, shopID string, nbTranches, delaiJours int) (*InstallmentPlan, error) {
	if nbTranches < 2 || nbTranches > 10 {
		return nil, errors.New("le nombre de tranches doit être compris entre 2 et 10")
	}
	if delaiJours < 1 {
		return nil, errors.New("le délai entre les tranches doit être d'au moins 1 jour")
	}

	return &InstallmentPlan{
		ID:         uuid.New().String(),
		ProductID:  productID,
		ShopID:     shopID,
		NbTranches: nbTranches,
		DelaiJours: delaiJours,
		IsActive:   true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}, nil
}

// Deactivate désactive le plan (le marchand ne veut plus vendre en tranches).
func (p *InstallmentPlan) Deactivate() {
	p.IsActive = false
	p.UpdatedAt = time.Now().UTC()
}

// ============================================================
// ORDER INSTALLMENT (Suivi d'une tranche spécifique)
// ============================================================

type InstallmentStatus string

const (
	InstallmentPending InstallmentStatus = "pending" // En attente de paiement
	InstallmentPaid    InstallmentStatus = "paid"    // Payé (argent bloqué en escrow)
	InstallmentOverdue InstallmentStatus = "overdue" // En retard (déclenche notification)
)

// OrderInstallment représente une échéance spécifique pour une commande.
type OrderInstallment struct {
	ID            string            `json:"id" db:"id"`
	OrderID       string            `json:"order_id" db:"order_id"`
	TrancheNumber int               `json:"tranche_number" db:"tranche_number"` // 1, 2, 3...
	AmountCents   int64             `json:"amount_cents" db:"amount_cents"`     // Montant en FCFA
	DueDate       time.Time         `json:"due_date" db:"due_date"`
	Status        InstallmentStatus `json:"status" db:"status"`
	PaidAt        *time.Time        `json:"paid_at,omitempty" db:"paid_at"`
	PaymentRef    *string           `json:"payment_ref,omitempty" db:"payment_ref"` // Réf Mobile Money
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

// NewOrderInstallment crée une tranche en attente.
func NewOrderInstallment(orderID string, trancheNumber int, amountCents int64, dueDate time.Time) *OrderInstallment {
	return &OrderInstallment{
		ID:            uuid.New().String(),
		OrderID:       orderID,
		TrancheNumber: trancheNumber,
		AmountCents:   amountCents,
		DueDate:       dueDate,
		Status:        InstallmentPending,
		CreatedAt:     time.Now().UTC(),
	}
}

// MarkPaid marque la tranche comme payée. L'argent est maintenant bloqué en séquestre.
func (i *OrderInstallment) MarkPaid(paymentRef string) error {
	if i.Status == InstallmentPaid {
		return errors.New("cette tranche est déjà payée")
	}
	now := time.Now().UTC()
	i.Status = InstallmentPaid
	i.PaidAt = &now
	i.PaymentRef = &paymentRef
	return nil
}

// MarkOverdue marque la tranche comme en retard (appelé par un scheduler).
func (i *OrderInstallment) MarkOverdue() error {
	if i.Status != InstallmentPending {
		return errors.New("seules les tranches en attente peuvent être marquées en retard")
	}
	i.Status = InstallmentOverdue
	return nil
}

// IsPaid vérifie si la tranche est payée.
func (i *OrderInstallment) IsPaid() bool {
	return i.Status == InstallmentPaid
}

// IsOverdue vérifie si la tranche est échue (date dépassée et non payée).
func (i *OrderInstallment) IsOverdue() bool {
	if i.IsPaid() {
		return false
	}
	return time.Now().UTC().After(i.DueDate)
}

// ============================================================
// HELPERS DE CALCUL
// ============================================================

// CalculateInstallmentAmount calcule le montant de chaque tranche.
// Le montant total est divisé équitablement. Le reste (arrondi) est ajouté à la dernière tranche.
// Ex: 10000 FCFA en 3 tranches -> 3333, 3333, 3334.
func CalculateInstallmentAmount(totalAmountCents int64, nbTranches int) []int64 {
	amounts := make([]int64, nbTranches)
	baseAmount := totalAmountCents / int64(nbTranches)
	remainder := totalAmountCents % int64(nbTranches)

	for i := 0; i < nbTranches; i++ {
		amounts[i] = baseAmount
	}
	// On ajoute le reste à la dernière tranche pour que la somme soit exacte
	amounts[nbTranches-1] += remainder

	return amounts
}

// CalculateDueDates calcule les dates d'échéance de chaque tranche à partir de la date de commande.
func CalculateDueDates(startDate time.Time, nbTranches, delaiJours int) []time.Time {
	dates := make([]time.Time, nbTranches)
	for i := 0; i < nbTranches; i++ {
		// La première tranche est due immédiatement (ou à J+0), la suivante à J+delai, etc.
		dates[i] = startDate.AddDate(0, 0, i*delaiJours)
	}
	return dates
}
