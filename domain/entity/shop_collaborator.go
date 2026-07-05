package entity

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 🆕 v4.3.0 : SHOP COLLABORATOR (Niveau 2)
// ============================================================

// ShopCollaborator représente un collaborateur boutique
type ShopCollaborator struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	ShopID      uuid.UUID       `json:"shop_id" db:"shop_id"`
	UserID      string          `json:"user_id" db:"user_id"`
	Role        ShopRole        `json:"role" db:"role"`
	Permissions ShopPermissions `json:"permissions" db:"permissions"`

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

// NewShopCollaborator crée un nouveau collaborateur boutique
func NewShopCollaborator(
	shopID uuid.UUID,
	userID string,
	role ShopRole,
	permissions ShopPermissions,
	invitedBy string,
) (*ShopCollaborator, error) {
	if shopID == uuid.Nil {
		return nil, errors.New("shop_id is required")
	}
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if !IsValidShopRole(role) {
		return nil, ErrInvalidShopRole
	}
	if invitedBy == "" {
		return nil, errors.New("invited_by is required")
	}

	now := time.Now()
	return &ShopCollaborator{
		ID:          uuid.New(),
		ShopID:      shopID,
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
func (sc *ShopCollaborator) MarkAccepted() {
	now := time.Now()
	sc.AcceptedAt = &now
	sc.UpdatedAt = now
}

// Deactivate désactive le collaborateur (soft delete)
func (sc *ShopCollaborator) Deactivate(deletedBy, reason string) error {
	if reason == "" {
		return ErrDeletionReasonRequired
	}

	now := time.Now()
	sc.IsActive = false
	sc.DeletedAt = &now
	sc.DeletedBy = &deletedBy
	sc.DeletionReason = &reason
	sc.UpdatedAt = now

	return nil
}

// Reactivate réactive le collaborateur
func (sc *ShopCollaborator) Reactivate() {
	now := time.Now()
	sc.IsActive = true
	sc.DeletedAt = nil
	sc.DeletedBy = nil
	sc.DeletionReason = nil
	sc.UpdatedAt = now
}

// UpdateRole change le rôle du collaborateur
func (sc *ShopCollaborator) UpdateRole(newRole ShopRole, newPermissions ShopPermissions) error {
	if !IsValidShopRole(newRole) {
		return ErrInvalidShopRole
	}

	sc.Role = newRole
	sc.Permissions = newPermissions
	sc.UpdatedAt = time.Now()

	return nil
}

// UpdateLastLogin met à jour la dernière connexion
func (sc *ShopCollaborator) UpdateLastLogin() {
	now := time.Now()
	sc.LastLoginAt = &now
	sc.LastActivityAt = &now
	sc.UpdatedAt = now
}

// HasPermission vérifie si le collaborateur a une permission spécifique
func (sc *ShopCollaborator) HasPermission(permission string) bool {
	// shop_admin a toutes les permissions
	if sc.Role == ShopRoleShopAdmin {
		return true
	}

	switch permission {
	case "can_manage_products":
		return sc.Permissions.CanManageProducts
	case "can_view_products":
		return sc.Permissions.CanViewProducts
	case "can_manage_orders":
		return sc.Permissions.CanManageOrders
	case "can_view_orders":
		return sc.Permissions.CanViewOrders
	case "can_manage_customers":
		return sc.Permissions.CanManageCustomers
	case "can_view_customers":
		return sc.Permissions.CanViewCustomers
	case "can_view_payments":
		return sc.Permissions.CanViewPayments
	case "can_withdraw":
		return sc.Permissions.CanWithdraw
	case "can_view_reports":
		return sc.Permissions.CanViewReports
	case "can_manage_settings":
		return sc.Permissions.CanManageSettings
	default:
		return false
	}
}

// ToJSON sérialise les permissions en JSON
func (sc *ShopCollaborator) ToJSON() ([]byte, error) {
	return json.Marshal(sc.Permissions)
}

// FromJSON désérialise les permissions depuis JSON
func (sc *ShopCollaborator) FromJSON(data []byte) error {
	return json.Unmarshal(data, &sc.Permissions)
}
