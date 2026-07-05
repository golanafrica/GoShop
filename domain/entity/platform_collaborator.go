package entity

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 🆕 v4.3.0 : PLATFORM COLLABORATOR (Niveau 1)
// ============================================================

// PlatformCollaborator représente un collaborateur plateforme
type PlatformCollaborator struct {
	ID          uuid.UUID           `json:"id" db:"id"`
	UserID      string              `json:"user_id" db:"user_id"`
	Role        PlatformRole        `json:"role" db:"role"`
	Permissions PlatformPermissions `json:"permissions" db:"permissions"`

	// Invitation
	InvitedBy  string     `json:"invited_by" db:"invited_by"`
	InvitedAt  time.Time  `json:"invited_at" db:"invited_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`

	// Statut
	IsActive bool `json:"is_active" db:"is_active"`

	// Soft delete avec audit
	DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	DeletedBy      *string    `json:"deleted_by,omitempty" db:"deleted_by"`
	DeletionReason *string    `json:"deletion_reason,omitempty" db:"deletion_reason"`

	// Activité
	LastLoginAt    *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty" db:"last_activity_at"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewPlatformCollaborator crée un nouveau collaborateur plateforme
func NewPlatformCollaborator(
	userID string,
	role PlatformRole,
	permissions PlatformPermissions,
	invitedBy string,
) (*PlatformCollaborator, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if !IsValidPlatformRole(role) {
		return nil, ErrInvalidPlatformRole
	}
	if invitedBy == "" {
		return nil, errors.New("invited_by is required")
	}

	now := time.Now()
	return &PlatformCollaborator{
		ID:          uuid.New(),
		UserID:      userID,
		Role:        role,
		Permissions: permissions,
		InvitedBy:   invitedBy,
		InvitedAt:   now,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MarkAccepted marque l'invitation comme acceptée
func (pc *PlatformCollaborator) MarkAccepted() {
	now := time.Now()
	pc.AcceptedAt = &now
	pc.UpdatedAt = now
}

// Deactivate désactive le collaborateur (soft delete)
func (pc *PlatformCollaborator) Deactivate(deletedBy, reason string) error {
	if reason == "" {
		return ErrDeletionReasonRequired
	}

	now := time.Now()
	pc.IsActive = false
	pc.DeletedAt = &now
	pc.DeletedBy = &deletedBy
	pc.DeletionReason = &reason
	pc.UpdatedAt = now

	return nil
}

// Reactivate réactive le collaborateur
func (pc *PlatformCollaborator) Reactivate() {
	now := time.Now()
	pc.IsActive = true
	pc.DeletedAt = nil
	pc.DeletedBy = nil
	pc.DeletionReason = nil
	pc.UpdatedAt = now
}

// UpdateRole change le rôle du collaborateur
func (pc *PlatformCollaborator) UpdateRole(newRole PlatformRole, newPermissions PlatformPermissions) error {
	if !IsValidPlatformRole(newRole) {
		return ErrInvalidPlatformRole
	}

	pc.Role = newRole
	pc.Permissions = newPermissions
	pc.UpdatedAt = time.Now()

	return nil
}

// UpdateLastLogin met à jour la dernière connexion
func (pc *PlatformCollaborator) UpdateLastLogin() {
	now := time.Now()
	pc.LastLoginAt = &now
	pc.LastActivityAt = &now
	pc.UpdatedAt = now
}

// HasPermission vérifie si le collaborateur a une permission spécifique
func (pc *PlatformCollaborator) HasPermission(permission string) bool {
	switch permission {
	case "can_manage_shops":
		return pc.Permissions.CanManageShops
	case "can_view_shops":
		return pc.Permissions.CanViewShops
	case "can_approve_kyc":
		return pc.Permissions.CanApproveKYC
	case "can_view_kyc":
		return pc.Permissions.CanViewKYC
	case "can_view_commissions":
		return pc.Permissions.CanViewCommissions
	case "can_export_reports":
		return pc.Permissions.CanExportReports
	case "can_manage_schedulers":
		return pc.Permissions.CanManageSchedulers
	case "can_invite_collaborators":
		return pc.Permissions.CanInviteCollaborators
	case "can_manage_settings":
		return pc.Permissions.CanManageSettings
	default:
		return false
	}
}

// ToJSON sérialise les permissions en JSON
func (pc *PlatformCollaborator) ToJSON() ([]byte, error) {
	return json.Marshal(pc.Permissions)
}

// FromJSON désérialise les permissions depuis JSON
func (pc *PlatformCollaborator) FromJSON(data []byte) error {
	return json.Unmarshal(data, &pc.Permissions)
}
