package entity

import (
	"errors"
	"time"
)

// ============================================================
// TIERS DE FIABILITÉ
// ============================================================

type ReliabilityTier string

const (
	TierBronze ReliabilityTier = "BRONZE" // Score 400-599 : Nouveau client
	TierSilver ReliabilityTier = "SILVER" // Score 600-799 : KYC vérifié
	TierGold   ReliabilityTier = "GOLD"   // Score 800-1000 : Client fidèle
)

// ============================================================
// LIMITES PAR TIER
// ============================================================

const (
	MaxInstallmentsBronze = 5 // Max 5 tranches
	MaxInstallmentsSilver = 5 // Max 5 tranches
	MaxInstallmentsGold   = 0 // 0 = illimité
)

// ============================================================
// RÈGLES DE SCORING
// ============================================================

const (
	// Score initial
	BaseScoreWithoutKYC = 400 // Nouveau client sans KYC
	BaseScoreWithKYC    = 600 // Nouveau client avec KYC vérifié

	// Bonus (ce qui fait monter le score)
	BonusKYCVerified        = 200 // +200 pour KYC vérifié (passage Bronze → Silver)
	BonusSuccessfulCODOrder = 20  // +20 par commande COD livrée sans litige
	BonusTontineCycleOnTime = 30  // +30 par cycle de tontine complété à l'heure
	BonusInstallmentOnTime  = 25  // +25 par tranche payée à l'échéance
	BonusTenOrdersNoDispute = 100 // +100 bonus pour 10 commandes sans litige

	// Malus (ce qui fait baisser le score)
	PenaltyLateInstallment    = 100 // -100 pour tranche en retard de +15 jours
	PenaltyCODDisputeLost     = 150 // -150 pour litige COD confirmé (client perd)
	PenaltyTontineMissedCycle = 80  // -80 pour cycle de tontine non payé
	PenaltyAccountFrozen      = 200 // -200 pour compte gelé (solde négatif)

	// Limites
	MinScore = 0
	MaxScore = 1000
)

// ============================================================
// ENTITÉ
// ============================================================

// CustomerReliabilityScore représente le score de fiabilité d'un client
type CustomerReliabilityScore struct {
	ID               string          `json:"id" db:"id"`
	CustomerID       string          `json:"customer_id" db:"customer_id"`
	Score            int             `json:"score" db:"score"`
	Tier             ReliabilityTier `json:"tier" db:"tier"`
	LastCalculatedAt time.Time       `json:"last_calculated_at" db:"last_calculated_at"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewCustomerReliabilityScore crée un score pour un nouveau client SANS KYC
func NewCustomerReliabilityScore(customerID string) *CustomerReliabilityScore {
	now := time.Now().UTC()
	return &CustomerReliabilityScore{
		CustomerID:       customerID,
		Score:            BaseScoreWithoutKYC, // 400
		Tier:             TierBronze,
		LastCalculatedAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// NewCustomerReliabilityScoreWithKYC crée un score pour un client AVEC KYC vérifié
func NewCustomerReliabilityScoreWithKYC(customerID string) *CustomerReliabilityScore {
	now := time.Now().UTC()
	return &CustomerReliabilityScore{
		CustomerID:       customerID,
		Score:            BaseScoreWithKYC, // 600
		Tier:             TierSilver,
		LastCalculatedAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// ============================================================
// MÉTHODES DE CALCUL
// ============================================================

// CalculateTier détermine le tier en fonction du score
func (s *CustomerReliabilityScore) CalculateTier() {
	if s.Score >= 800 {
		s.Tier = TierGold
	} else if s.Score >= 600 {
		s.Tier = TierSilver
	} else {
		s.Tier = TierBronze
	}
}

// ApplyBonus ajoute des points (plafonné à 1000)
func (s *CustomerReliabilityScore) ApplyBonus(points int) {
	s.Score += points
	if s.Score > MaxScore {
		s.Score = MaxScore
	}
	s.CalculateTier()
	s.LastCalculatedAt = time.Now().UTC()
	s.UpdatedAt = time.Now().UTC()
}

// ApplyPenalty retire des points (plancher à 0)
func (s *CustomerReliabilityScore) ApplyPenalty(points int) {
	s.Score -= points
	if s.Score < MinScore {
		s.Score = MinScore
	}
	s.CalculateTier()
	s.LastCalculatedAt = time.Now().UTC()
	s.UpdatedAt = time.Now().UTC()
}

// ============================================================
// MÉTHODES MÉTIER
// ============================================================

// CanUseInstallments vérifie si le client peut utiliser les tranches
// Tous les tiers peuvent utiliser les tranches (Bronze/Silver : max 5, Gold : illimité)
func (s *CustomerReliabilityScore) CanUseInstallments() bool {
	return true
}

// GetMaxInstallments retourne le nombre maximum de tranches autorisées
func (s *CustomerReliabilityScore) GetMaxInstallments() int {
	switch s.Tier {
	case TierGold:
		return MaxInstallmentsGold // 0 = illimité
	case TierSilver:
		return MaxInstallmentsSilver // 5
	case TierBronze:
		return MaxInstallmentsBronze // 5
	default:
		return MaxInstallmentsBronze
	}
}

// HasInstallmentLimit vérifie si le client a une limite de tranches
func (s *CustomerReliabilityScore) HasInstallmentLimit() bool {
	return s.Tier != TierGold
}

// CanJoinCommercialTontine vérifie si le client peut rejoindre une tontine commerciale
func (s *CustomerReliabilityScore) CanJoinCommercialTontine() bool {
	return s.Tier == TierSilver || s.Tier == TierGold
}

// CanJoinFamilyTontine vérifie si le client peut rejoindre une tontine familiale
func (s *CustomerReliabilityScore) CanJoinFamilyTontine() bool {
	return true
}

// ============================================================
// VALIDATION
// ============================================================

// Validate valide l'entité avant persistance
func (s *CustomerReliabilityScore) Validate() error {
	if s.CustomerID == "" {
		return errors.New("customer_id is required")
	}
	if s.Score < MinScore || s.Score > MaxScore {
		return errors.New("score must be between 0 and 1000")
	}
	switch s.Tier {
	case TierBronze, TierSilver, TierGold:
		return nil
	default:
		return errors.New("invalid tier")
	}
}
