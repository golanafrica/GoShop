package entity

import (
	"errors"
	"time"
)

// ============================================================
// Statuts du groupe
// ============================================================

const (
	TontineStatusPendingMembers = "PENDING_MEMBERS"
	TontineStatusActive         = "ACTIVE"
	TontineStatusCompleted      = "COMPLETED"
)

// ============================================================
// Types de cercle
// ============================================================

const (
	TontineCircleCommercial = "COMMERCIAL"
	TontineCircleCorporate  = "CORPORATE"
	TontineCircleFamily     = "FAMILY"
)

// IsValidCircleType vérifie si le type de cercle est valide
func IsValidCircleType(circleType string) bool {
	switch circleType {
	case TontineCircleCommercial, TontineCircleCorporate, TontineCircleFamily:
		return true
	}
	return false
}

// ============================================================
// Types de créateur
// ============================================================

const (
	TontineCreatorMerchant = "merchant"
	TontineCreatorCustomer = "customer"
)

// IsValidCreatorType vérifie si le type de créateur est valide
func IsValidCreatorType(creatorType string) bool {
	switch creatorType {
	case TontineCreatorMerchant, TontineCreatorCustomer:
		return true
	}
	return false
}

// ============================================================
// Statuts participant
// ============================================================

const (
	ParticipantStatusActive    = "active"
	ParticipantStatusSuspended = "suspended"
	ParticipantStatusExcluded  = "excluded"
)

// ============================================================
// Statuts paiement
// ============================================================

const (
	TontinePaymentPending    = "PENDING"
	TontinePaymentProcessing = "PROCESSING"
	TontinePaymentDone       = "DONE"
	TontinePaymentFailed     = "FAILED"
)

// ============================================================
// Statuts voucher
// ============================================================

const (
	VoucherStatusGenerated = "generated"
	VoucherStatusRedeemed  = "redeemed"
	VoucherStatusExpired   = "expired"
	VoucherStatusCancelled = "cancelled"
)

// ============================================================
// ProductTontineSettings — config tontine par produit
// ============================================================

// ProductTontineSettings représente la configuration tontine d'un produit
type ProductTontineSettings struct {
	ProductID             string    `json:"product_id" db:"product_id"`
	ShopID                string    `json:"shop_id" db:"shop_id"`
	IsTontineEnabled      bool      `json:"is_tontine_enabled" db:"is_tontine_enabled"`
	AllowCommercialCircle bool      `json:"allow_commercial_circle" db:"allow_commercial_circle"`
	AllowCorporateCircle  bool      `json:"allow_corporate_circle" db:"allow_corporate_circle"`
	AllowFamilyCircle     bool      `json:"allow_family_circle" db:"allow_family_circle"`
	MinParticipants       int       `json:"min_participants" db:"min_participants"`
	MaxParticipants       int       `json:"max_participants" db:"max_participants"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// IsCircleTypeAllowed vérifie si un type de cercle est autorisé pour ce produit
func (s *ProductTontineSettings) IsCircleTypeAllowed(circleType string) bool {
	switch circleType {
	case TontineCircleCommercial:
		return s.AllowCommercialCircle
	case TontineCircleCorporate:
		return s.AllowCorporateCircle
	case TontineCircleFamily:
		return s.AllowFamilyCircle
	default:
		return false
	}
}

// ValidateParticipantCount vérifie si le nombre de participants est valide
func (s *ProductTontineSettings) ValidateParticipantCount(count int) error {
	if count < s.MinParticipants {
		return errors.New("le nombre de participants doit être au minimum de " + string(rune(s.MinParticipants+'0')))
	}
	if count > s.MaxParticipants {
		return errors.New("le nombre de participants ne peut pas dépasser " + string(rune(s.MaxParticipants+'0')))
	}
	return nil
}

// ============================================================
// TontineGroup — entité principale
// ============================================================

// TontineGroup représente un groupe de tontine
type TontineGroup struct {
	ID                string  `json:"id" db:"id"`
	ProductID         string  `json:"product_id" db:"product_id"`
	ShopID            string  `json:"shop_id" db:"shop_id"`
	CreatorCustomerID *string `json:"creator_customer_id,omitempty" db:"creator_customer_id"`
	CreatorType       string  `json:"creator_type" db:"creator_type"`

	CircleType          string `json:"circle_type" db:"circle_type"`
	AmountPerCycleCents int64  `json:"amount_per_cycle_cents" db:"amount_per_cycle_cents"`

	TotalCycles  int `json:"total_cycles" db:"total_cycles"`
	CurrentCycle int `json:"current_cycle" db:"current_cycle"`

	InviteCode string `json:"invite_code" db:"invite_code"`

	Status      string     `json:"status" db:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewTontineGroup crée un nouveau groupe de tontine
func NewTontineGroup(
	productID, shopID string,
	creatorCustomerID *string,
	creatorType, circleType string,
	amountPerCycleCents int64,
	totalCycles int,
	inviteCode string,
) (*TontineGroup, error) {
	// Validations
	if productID == "" {
		return nil, errors.New("product_id is required")
	}
	if shopID == "" {
		return nil, errors.New("shop_id is required")
	}
	if !IsValidCreatorType(creatorType) {
		return nil, errors.New("invalid creator type")
	}
	if creatorType == TontineCreatorCustomer && creatorCustomerID == nil {
		return nil, errors.New("creator_customer_id is required when creator is customer")
	}
	if !IsValidCircleType(circleType) {
		return nil, errors.New("invalid circle type")
	}
	if amountPerCycleCents <= 0 {
		return nil, errors.New("amount per cycle must be positive")
	}
	if totalCycles < 2 {
		return nil, errors.New("total cycles must be at least 2")
	}
	if inviteCode == "" {
		return nil, errors.New("invite code is required")
	}

	now := time.Now().UTC()
	return &TontineGroup{
		ProductID:           productID,
		ShopID:              shopID,
		CreatorCustomerID:   creatorCustomerID,
		CreatorType:         creatorType,
		CircleType:          circleType,
		AmountPerCycleCents: amountPerCycleCents,
		TotalCycles:         totalCycles,
		CurrentCycle:        1,
		InviteCode:          inviteCode,
		Status:              TontineStatusPendingMembers,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// CanStart vérifie si le groupe peut démarrer
func (g *TontineGroup) CanStart(currentMembersCount int) error {
	if g.Status != TontineStatusPendingMembers {
		return errors.New("le groupe n'est pas en attente de membres")
	}
	if currentMembersCount < g.TotalCycles {
		return errors.New("le groupe n'est pas encore au complet")
	}
	return nil
}

// Start démarre le groupe
func (g *TontineGroup) Start() error {
	if g.Status != TontineStatusPendingMembers {
		return errors.New("le groupe ne peut pas démarrer depuis le statut " + g.Status)
	}
	now := time.Now().UTC()
	g.Status = TontineStatusActive
	g.StartedAt = &now
	g.UpdatedAt = now
	return nil
}

// IsLastCycle vérifie si c'est le dernier cycle
func (g *TontineGroup) IsLastCycle() bool {
	return g.CurrentCycle == g.TotalCycles
}

// IsActive vérifie si le groupe est actif
func (g *TontineGroup) IsActive() bool {
	return g.Status == TontineStatusActive
}

// Complete marque le groupe comme terminé
func (g *TontineGroup) Complete() error {
	if g.Status != TontineStatusActive {
		return errors.New("seul un groupe actif peut être complété")
	}
	now := time.Now().UTC()
	g.Status = TontineStatusCompleted
	g.CompletedAt = &now
	g.UpdatedAt = now
	return nil
}

// IncrementCycle passe au cycle suivant
func (g *TontineGroup) IncrementCycle() error {
	if !g.IsActive() {
		return errors.New("le groupe doit être actif")
	}
	if g.IsLastCycle() {
		return errors.New("déjà au dernier cycle")
	}
	g.CurrentCycle++
	g.UpdatedAt = time.Now().UTC()
	return nil
}

// TotalAmountCents retourne le montant total que chaque participant paiera
func (g *TontineGroup) TotalAmountCents() int64 {
	return g.AmountPerCycleCents * int64(g.TotalCycles)
}

// ============================================================
// TontineParticipant
// ============================================================

// TontineParticipant représente un participant à un groupe de tontine
type TontineParticipant struct {
	ID             string    `json:"id" db:"id"`
	GroupID        string    `json:"group_id" db:"group_id"`
	CustomerID     string    `json:"customer_id" db:"customer_id"`
	PayoutPosition int       `json:"payout_position" db:"payout_position"`
	Status         string    `json:"status" db:"status"`
	JoinedAt       time.Time `json:"joined_at" db:"joined_at"`
}

// NewTontineParticipant crée un nouveau participant
func NewTontineParticipant(groupID, customerID string, payoutPosition int) (*TontineParticipant, error) {
	if groupID == "" {
		return nil, errors.New("group_id is required")
	}
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}
	if payoutPosition < 1 {
		return nil, errors.New("payout_position must be at least 1")
	}

	return &TontineParticipant{
		GroupID:        groupID,
		CustomerID:     customerID,
		PayoutPosition: payoutPosition,
		Status:         ParticipantStatusActive,
		JoinedAt:       time.Now().UTC(),
	}, nil
}

// IsBeneficiaryForCycle vérifie si ce participant reçoit le bien à ce cycle
func (p *TontineParticipant) IsBeneficiaryForCycle(cycleNumber int) bool {
	return p.PayoutPosition == cycleNumber
}

// IsActive vérifie si le participant est actif
func (p *TontineParticipant) IsActive() bool {
	return p.Status == ParticipantStatusActive
}

// ============================================================
// TontinePayment
// ============================================================

// TontinePayment représente un paiement de cotisation
type TontinePayment struct {
	ID                    string     `json:"id" db:"id"`
	GroupID               string     `json:"group_id" db:"group_id"`
	ParticipantID         string     `json:"participant_id" db:"participant_id"`
	CustomerID            string     `json:"customer_id" db:"customer_id"`
	CycleNumber           int        `json:"cycle_number" db:"cycle_number"`
	AmountCents           int64      `json:"amount_cents" db:"amount_cents"`
	CommissionCents       int64      `json:"commission_cents" db:"commission_cents"`
	YengaPayReference     *string    `json:"yengapay_reference,omitempty" db:"yengapay_reference"`
	YengaPayTransactionID *string    `json:"yengapay_transaction_id,omitempty" db:"yengapay_transaction_id"`
	PaymentProvider       string     `json:"payment_provider" db:"payment_provider"`
	Status                string     `json:"status" db:"status"`
	DueDate               time.Time  `json:"due_date" db:"due_date"`
	PaidAt                *time.Time `json:"paid_at,omitempty" db:"paid_at"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
	CommissionStatus      string     `json:"commission_status" db:"commission_status"` // pending, collected, failed

	// 🆕 v3.3.0 : Pour le scheduler (non persisté en DB)
	ShopID string `json:"shop_id,omitempty" db:"-"` // Shop ID du groupe (pour le multi-tenant)
}

// NewTontinePayment crée un nouveau paiement de cotisation
func NewTontinePayment(
	groupID, participantID, customerID string,
	cycleNumber int,
	amountCents, commissionCents int64,
	dueDate time.Time,
) (*TontinePayment, error) {
	if groupID == "" || participantID == "" || customerID == "" {
		return nil, errors.New("group_id, participant_id and customer_id are required")
	}
	if cycleNumber < 1 {
		return nil, errors.New("cycle_number must be at least 1")
	}
	if amountCents <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if commissionCents < 0 {
		return nil, errors.New("commission cannot be negative")
	}

	now := time.Now().UTC()
	return &TontinePayment{
		GroupID:         groupID,
		ParticipantID:   participantID,
		CustomerID:      customerID,
		CycleNumber:     cycleNumber,
		AmountCents:     amountCents,
		CommissionCents: commissionCents,
		PaymentProvider: "yenga_pay",
		Status:          TontinePaymentPending,
		DueDate:         dueDate,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// MarkProcessing marque le paiement comme en cours de traitement
func (p *TontinePayment) MarkProcessing(reference string) error {
	if p.Status != TontinePaymentPending {
		return errors.New("le paiement doit être en pending")
	}
	p.Status = TontinePaymentProcessing
	p.YengaPayReference = &reference
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkDone marque le paiement comme effectué
func (p *TontinePayment) MarkDone(transactionID string) error {
	if p.Status != TontinePaymentPending && p.Status != TontinePaymentProcessing {
		return errors.New("le paiement doit être en pending ou processing")
	}
	now := time.Now().UTC()
	p.Status = TontinePaymentDone
	p.PaidAt = &now
	p.YengaPayTransactionID = &transactionID
	p.UpdatedAt = now
	return nil
}

// MarkFailed marque le paiement comme échoué
func (p *TontinePayment) MarkFailed() error {
	if p.Status != TontinePaymentPending && p.Status != TontinePaymentProcessing {
		return errors.New("le paiement doit être en pending ou processing")
	}
	p.Status = TontinePaymentFailed
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// IsDone vérifie si le paiement est effectué
func (p *TontinePayment) IsDone() bool {
	return p.Status == TontinePaymentDone
}

// NetAmountCents retourne le montant net (après commission)
func (p *TontinePayment) NetAmountCents() int64 {
	return p.AmountCents - p.CommissionCents
}

// ============================================================
// TontineVoucher
// ============================================================

// TontineVoucher représente un voucher de livraison
type TontineVoucher struct {
	ID            string     `json:"id" db:"id"`
	GroupID       string     `json:"group_id" db:"group_id"`
	ParticipantID string     `json:"participant_id" db:"participant_id"`
	CustomerID    string     `json:"customer_id" db:"customer_id"`
	ProductID     string     `json:"product_id" db:"product_id"`
	ShopID        string     `json:"shop_id" db:"shop_id"`
	VoucherCode   string     `json:"voucher_code" db:"voucher_code"`
	CycleNumber   int        `json:"cycle_number" db:"cycle_number"`
	Status        string     `json:"status" db:"status"`
	ExpiresAt     time.Time  `json:"expires_at" db:"expires_at"`
	RedeemedAt    *time.Time `json:"redeemed_at,omitempty" db:"redeemed_at"`
	RedeemedBy    *string    `json:"redeemed_by,omitempty" db:"redeemed_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// NewTontineVoucher crée un nouveau voucher avec 6 mois de validité
func NewTontineVoucher(
	groupID, participantID, customerID, productID, shopID, voucherCode string,
	cycleNumber int,
) (*TontineVoucher, error) {
	if groupID == "" || participantID == "" || customerID == "" {
		return nil, errors.New("group_id, participant_id and customer_id are required")
	}
	if productID == "" || shopID == "" {
		return nil, errors.New("product_id and shop_id are required")
	}
	if voucherCode == "" {
		return nil, errors.New("voucher_code is required")
	}
	if cycleNumber < 1 {
		return nil, errors.New("cycle_number must be at least 1")
	}

	now := time.Now().UTC()
	return &TontineVoucher{
		GroupID:       groupID,
		ParticipantID: participantID,
		CustomerID:    customerID,
		ProductID:     productID,
		ShopID:        shopID,
		VoucherCode:   voucherCode,
		CycleNumber:   cycleNumber,
		Status:        VoucherStatusGenerated,
		ExpiresAt:     now.AddDate(0, 6, 0), // 6 mois de validité
		CreatedAt:     now,
	}, nil
}

// IsValid vérifie si le voucher peut être utilisé
func (v *TontineVoucher) IsValid() bool {
	return v.Status == VoucherStatusGenerated &&
		time.Now().UTC().Before(v.ExpiresAt)
}

// IsExpired vérifie si le voucher a expiré
func (v *TontineVoucher) IsExpired() bool {
	return v.Status == VoucherStatusGenerated &&
		time.Now().UTC().After(v.ExpiresAt)
}

// IsRedeemableInShop vérifie que le voucher est utilisable dans cette boutique
func (v *TontineVoucher) IsRedeemableInShop(shopID string) bool {
	return v.ShopID == shopID
}

// Redeem marque le voucher comme utilisé
func (v *TontineVoucher) Redeem(redeemerID string) error {
	if !v.IsValid() {
		return errors.New("le voucher n'est pas valide")
	}
	if redeemerID == "" {
		return errors.New("redeemer_id is required")
	}
	now := time.Now().UTC()
	v.Status = VoucherStatusRedeemed
	v.RedeemedAt = &now
	v.RedeemedBy = &redeemerID
	return nil
}

// ============================================================
// Fonctions utilitaires
// ============================================================

// CalculateAmountPerCycle calcule le montant par cycle en centimes
// Retourne (amountPerCycle, remainder) pour gérer les divisions non exactes
func CalculateAmountPerCycle(totalPriceCents int64, totalCycles int) (int64, int64) {
	if totalCycles <= 0 {
		return 0, totalPriceCents
	}
	amountPerCycle := totalPriceCents / int64(totalCycles)
	remainder := totalPriceCents % int64(totalCycles)
	return amountPerCycle, remainder
}

// CalculateCommission calcule la commission GoShop en centimes
// commissionRateBp est en basis points (250 = 2.50%, max 1500 = 15%)
func CalculateCommission(amountCents int64, commissionRateBp int) int64 {
	if commissionRateBp < 0 || commissionRateBp > 1500 {
		return 0
	}
	return (amountCents * int64(commissionRateBp)) / 10000
}
