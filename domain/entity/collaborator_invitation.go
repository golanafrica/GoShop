package entity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 🆕 v4.3.0 : COLLABORATOR INVITATION
// ============================================================

// CollaboratorInvitation représente une invitation en attente
type CollaboratorInvitation struct {
	ID             uuid.UUID        `json:"id" db:"id"`
	InvitationType InvitationType   `json:"invitation_type" db:"invitation_type"`
	ShopID         *uuid.UUID       `json:"shop_id,omitempty" db:"shop_id"`
	Email          string           `json:"email" db:"email"`
	Role           string           `json:"role" db:"role"`
	Permissions    json.RawMessage  `json:"permissions" db:"permissions"`
	Token          string           `json:"token" db:"token"`
	InvitedBy      string           `json:"invited_by" db:"invited_by"`
	Status         InvitationStatus `json:"status" db:"status"`
	InvitedAt      time.Time        `json:"invited_at" db:"invited_at"`
	ExpiresAt      time.Time        `json:"expires_at" db:"expires_at"`
	AcceptedAt     *time.Time       `json:"accepted_at,omitempty" db:"accepted_at"`
	CancelledAt    *time.Time       `json:"cancelled_at,omitempty" db:"cancelled_at"`
	Message        *string          `json:"message,omitempty" db:"message"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at" db:"updated_at"`
}

// NewPlatformInvitation crée une nouvelle invitation plateforme
func NewPlatformInvitation(
	email string,
	role PlatformRole,
	permissions PlatformPermissions,
	invitedBy string,
	message *string,
) (*CollaboratorInvitation, error) {
	if email == "" {
		return nil, errors.New("email is required")
	}
	if !IsValidPlatformRole(role) {
		return nil, ErrInvalidPlatformRole
	}
	if invitedBy == "" {
		return nil, errors.New("invited_by is required")
	}

	token, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	permJSON, err := json.Marshal(permissions)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: InvitationTypePlatform,
		Email:          email,
		Role:           string(role),
		Permissions:    permJSON,
		Token:          token,
		InvitedBy:      invitedBy,
		Status:         InvitationStatusPending,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		Message:        message,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// NewShopInvitation crée une nouvelle invitation boutique
func NewShopInvitation(
	shopID uuid.UUID,
	email string,
	role ShopRole,
	permissions ShopPermissions,
	invitedBy string,
	message *string,
) (*CollaboratorInvitation, error) {
	if shopID == uuid.Nil {
		return nil, errors.New("shop_id is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}
	if !IsValidShopRole(role) {
		return nil, ErrInvalidShopRole
	}
	if invitedBy == "" {
		return nil, errors.New("invited_by is required")
	}

	token, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	permJSON, err := json.Marshal(permissions)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: InvitationTypeShop,
		ShopID:         &shopID,
		Email:          email,
		Role:           string(role),
		Permissions:    permJSON,
		Token:          token,
		InvitedBy:      invitedBy,
		Status:         InvitationStatusPending,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		Message:        message,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// MarkAccepted marque l'invitation comme acceptée
func (ci *CollaboratorInvitation) MarkAccepted() error {
	if ci.Status != InvitationStatusPending {
		return ErrInvitationAlreadyUsed
	}
	if ci.IsExpired() {
		return ErrInvitationExpired
	}

	now := time.Now()
	ci.Status = InvitationStatusAccepted
	ci.AcceptedAt = &now
	ci.UpdatedAt = now

	return nil
}

// Cancel annule l'invitation
func (ci *CollaboratorInvitation) Cancel() {
	now := time.Now()
	ci.Status = InvitationStatusCancelled
	ci.CancelledAt = &now
	ci.UpdatedAt = now
}

// IsExpired vérifie si l'invitation a expiré
func (ci *CollaboratorInvitation) IsExpired() bool {
	return time.Now().After(ci.ExpiresAt)
}

// IsPending vérifie si l'invitation est en attente
func (ci *CollaboratorInvitation) IsPending() bool {
	return ci.Status == InvitationStatusPending && !ci.IsExpired()
}

// GetPlatformPermissions retourne les permissions plateforme
func (ci *CollaboratorInvitation) GetPlatformPermissions() (PlatformPermissions, error) {
	var perms PlatformPermissions
	if err := json.Unmarshal(ci.Permissions, &perms); err != nil {
		return PlatformPermissions{}, err
	}
	return perms, nil
}

// GetShopPermissions retourne les permissions boutique
func (ci *CollaboratorInvitation) GetShopPermissions() (ShopPermissions, error) {
	var perms ShopPermissions
	if err := json.Unmarshal(ci.Permissions, &perms); err != nil {
		return ShopPermissions{}, err
	}
	return perms, nil
}

// generateSecureToken génère un token sécurisé de 64 caractères hex
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
