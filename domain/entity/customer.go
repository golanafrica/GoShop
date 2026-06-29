package entity

import (
	"errors"
	"time"
)

// Customer représente un client dans une boutique
type Customer struct {
	ID        string    `json:"id" db:"id"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// 🆕 Champs KYC (Know Your Customer) pour tontine
	KYCLevel       KYCLevel   `json:"kyc_level" db:"kyc_level"`
	KYCValidatedAt *time.Time `json:"kyc_validated_at,omitempty" db:"kyc_validated_at"`
	KYCValidatedBy *string    `json:"kyc_validated_by,omitempty" db:"kyc_validated_by"`
}

// ============================================================
// Méthodes KYC pour participation à la tontine
// ============================================================

// CanParticipateInTontine vérifie si le client peut participer à une tontine
// Retourne une erreur si le KYC n'est pas vérifié
func (c *Customer) CanParticipateInTontine() error {
	if c.KYCLevel != KYCLevelVerified {
		return errors.New("votre identité doit être vérifiée avant de participer à une tontine. Veuillez uploader votre CNI ou passeport.")
	}
	return nil
}

// MarkKYCPending marque le client comme en attente de validation KYC
// À appeler après l'upload des documents KYC
func (c *Customer) MarkKYCPending() {
	c.KYCLevel = KYCLevelPending
	c.KYCValidatedAt = nil
	c.KYCValidatedBy = nil
	c.UpdatedAt = time.Now().UTC()
}

// MarkKYCVerified valide le KYC du client
// validatedBy est l'ID du marchand (user owner du shop) qui a validé
func (c *Customer) MarkKYCVerified(validatedBy string) {
	if validatedBy == "" {
		return
	}
	now := time.Now().UTC()
	c.KYCLevel = KYCLevelVerified
	c.KYCValidatedAt = &now
	c.KYCValidatedBy = &validatedBy
	c.UpdatedAt = now
}

// MarkKYCRejected rejette le KYC du client
// Le client devra re-uploader des documents
func (c *Customer) MarkKYCRejected() {
	c.KYCLevel = KYCLevelRejected
	c.KYCValidatedAt = nil
	c.KYCValidatedBy = nil
	c.UpdatedAt = time.Now().UTC()
}

// IsKYCVerified vérifie si le KYC est validé
func (c *Customer) IsKYCVerified() bool {
	return c.KYCLevel == KYCLevelVerified
}

// IsKYCPending vérifie si le KYC est en attente
func (c *Customer) IsKYCPending() bool {
	return c.KYCLevel == KYCLevelPending
}

// IsKYCRejected vérifie si le KYC a été rejeté
func (c *Customer) IsKYCRejected() bool {
	return c.KYCLevel == KYCLevelRejected
}

// NeedsKYC vérifie si le client doit passer par le KYC pour participer à une tontine
// (ni vérifié, ni en attente)
func (c *Customer) NeedsKYC() bool {
	return c.KYCLevel == KYCLevelNone || c.KYCLevel == KYCLevelRejected
}
