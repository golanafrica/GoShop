package entity

import (
	"encoding/json"
	"errors"
)

// ============================================================
// 🆕 v4.3.0 : PERMISSIONS COLLABORATEURS
// ============================================================
//
// 🎯 Objectif :
//   Définir la structure JSONB booléenne pour les permissions
//   des collaborateurs plateforme et boutique.
//
// 📋 Règles :
//   - Structure booléenne simple (true/false)
//   - Permissions par défaut par rôle
//   - Pas de cumul plateforme/boutique
//   - Soft delete avec audit trail
//
// ============================================================

// ============================================================
// PERMISSIONS PLATEFORME (Niveau 1)
// ============================================================

// PlatformPermissions représente les permissions d'un collaborateur plateforme
type PlatformPermissions struct {
	// Gestion des shops
	CanManageShops bool `json:"can_manage_shops"` // Voir/suspendre/activer toutes les shops
	CanViewShops   bool `json:"can_view_shops"`   // Voir les shops (lecture seule)

	// KYC
	CanApproveKYC bool `json:"can_approve_kyc"` // Approuver/rejeter KYC marchands
	CanViewKYC    bool `json:"can_view_kyc"`    // Voir les documents KYC

	// Finances
	CanViewCommissions bool `json:"can_view_commissions"` // Voir les commissions
	CanExportReports   bool `json:"can_export_reports"`   // Exporter rapports BCEAO

	// Schedulers
	CanManageSchedulers bool `json:"can_manage_schedulers"` // Trigger schedulers manuellement

	// Gestion collaborateurs
	CanInviteCollaborators bool `json:"can_invite_collaborators"` // Inviter d'autres collaborateurs

	// Configuration
	CanManageSettings bool `json:"can_manage_settings"` // Modifier config plateforme
}

// 🆕 v4.3.0 : ToJSON sérialise les permissions plateforme en JSON
func (p PlatformPermissions) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// PlatformRole représente le rôle d'un collaborateur plateforme
type PlatformRole string

const (
	PlatformRoleFinanceManager   PlatformRole = "finance_manager"
	PlatformRoleSupportManager   PlatformRole = "support_manager"
	PlatformRoleKYCReviewer      PlatformRole = "kyc_reviewer"
	PlatformRoleMarketingManager PlatformRole = "marketing_manager"
	PlatformRoleTechAdmin        PlatformRole = "tech_admin"
)

// IsValidPlatformRole vérifie si le rôle est valide
func IsValidPlatformRole(role PlatformRole) bool {
	switch role {
	case PlatformRoleFinanceManager, PlatformRoleSupportManager,
		PlatformRoleKYCReviewer, PlatformRoleMarketingManager, PlatformRoleTechAdmin:
		return true
	}
	return false
}

// DefaultPlatformPermissions retourne les permissions par défaut pour un rôle
func DefaultPlatformPermissions(role PlatformRole) PlatformPermissions {
	switch role {
	case PlatformRoleFinanceManager:
		return PlatformPermissions{
			CanViewCommissions: true,
			CanExportReports:   true,
			CanViewShops:       true,
		}
	case PlatformRoleSupportManager:
		return PlatformPermissions{
			CanManageShops: true,
			CanViewKYC:     true,
		}
	case PlatformRoleKYCReviewer:
		return PlatformPermissions{
			CanApproveKYC: true,
			CanViewKYC:    true,
			CanViewShops:  true,
		}
	case PlatformRoleMarketingManager:
		return PlatformPermissions{
			CanViewShops: true,
		}
	case PlatformRoleTechAdmin:
		return PlatformPermissions{
			CanManageSchedulers:    true,
			CanManageSettings:      true,
			CanInviteCollaborators: true,
		}
	default:
		return PlatformPermissions{}
	}
}

// ============================================================
// PERMISSIONS BOUTIQUE (Niveau 2)
// ============================================================

// ShopPermissions représente les permissions d'un collaborateur boutique
type ShopPermissions struct {
	// Produits
	CanManageProducts bool `json:"can_manage_products"` // Créer/modifier/supprimer produits
	CanViewProducts   bool `json:"can_view_products"`   // Voir les produits

	// Commandes
	CanManageOrders bool `json:"can_manage_orders"` // Traiter/annuler commandes
	CanViewOrders   bool `json:"can_view_orders"`   // Voir les commandes

	// Clients
	CanManageCustomers bool `json:"can_manage_customers"` // Modifier/supprimer clients
	CanViewCustomers   bool `json:"can_view_customers"`   // Voir les clients

	// Paiements
	CanViewPayments bool `json:"can_view_payments"` // Voir les paiements
	CanWithdraw     bool `json:"can_withdraw"`      // Faire des retraits (si KYC vérifié)

	// Rapports
	CanViewReports bool `json:"can_view_reports"` // Voir les rapports boutique

	// Configuration
	CanManageSettings bool `json:"can_manage_settings"` // Modifier config boutique
}

// 🆕 v4.3.0 : ToJSON sérialise les permissions boutique en JSON
func (s ShopPermissions) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// ShopRole représente le rôle d'un collaborateur boutique
type ShopRole string

const (
	ShopRoleShopAdmin  ShopRole = "shop_admin"
	ShopRoleSeller     ShopRole = "seller"
	ShopRoleSupport    ShopRole = "support"
	ShopRoleAccountant ShopRole = "accountant"
)

// IsValidShopRole vérifie si le rôle est valide
func IsValidShopRole(role ShopRole) bool {
	switch role {
	case ShopRoleShopAdmin, ShopRoleSeller, ShopRoleSupport, ShopRoleAccountant:
		return true
	}
	return false
}

// DefaultShopPermissions retourne les permissions par défaut pour un rôle
func DefaultShopPermissions(role ShopRole) ShopPermissions {
	switch role {
	case ShopRoleShopAdmin:
		return ShopPermissions{
			CanManageProducts:  true,
			CanManageOrders:    true,
			CanManageCustomers: true,
			CanViewPayments:    true,
			CanWithdraw:        true,
			CanViewReports:     true,
			CanManageSettings:  true,
		}
	case ShopRoleSeller:
		return ShopPermissions{
			CanManageProducts: true,
			CanManageOrders:   true,
			CanViewCustomers:  true,
			CanViewPayments:   true,
		}
	case ShopRoleSupport:
		return ShopPermissions{
			CanViewProducts:    true,
			CanViewOrders:      true,
			CanManageCustomers: true,
			CanViewPayments:    true,
		}
	case ShopRoleAccountant:
		return ShopPermissions{
			CanViewPayments: true,
			CanWithdraw:     true,
			CanViewReports:  true,
		}
	default:
		return ShopPermissions{}
	}
}

// ============================================================
// STATUTS D'INVITATION
// ============================================================

// InvitationStatus représente le statut d'une invitation
type InvitationStatus string

const (
	InvitationStatusPending   InvitationStatus = "pending"
	InvitationStatusAccepted  InvitationStatus = "accepted"
	InvitationStatusExpired   InvitationStatus = "expired"
	InvitationStatusCancelled InvitationStatus = "cancelled"
)

// IsValidInvitationStatus vérifie si le statut est valide
func IsValidInvitationStatus(status InvitationStatus) bool {
	switch status {
	case InvitationStatusPending, InvitationStatusAccepted,
		InvitationStatusExpired, InvitationStatusCancelled:
		return true
	}
	return false
}

// InvitationType représente le type d'invitation
type InvitationType string

const (
	InvitationTypePlatform InvitationType = "platform"
	InvitationTypeShop     InvitationType = "shop"
)

// IsValidInvitationType vérifie si le type est valide
func IsValidInvitationType(invType InvitationType) bool {
	switch invType {
	case InvitationTypePlatform, InvitationTypeShop:
		return true
	}
	return false
}

// ============================================================
// ERREURS
// ============================================================

var (
	ErrInvalidPlatformRole     = errors.New("invalid platform role")
	ErrInvalidShopRole         = errors.New("invalid shop role")
	ErrInvalidInvitationStatus = errors.New("invalid invitation status")
	ErrInvalidInvitationType   = errors.New("invalid invitation type")
	ErrInvitationExpired       = errors.New("invitation has expired")
	ErrInvitationAlreadyUsed   = errors.New("invitation already used")
	ErrInvitationCancelled     = errors.New("invitation has been cancelled")
	ErrUserAlreadyCollaborator = errors.New("user is already a collaborator")
	ErrCollaboratorNotFound    = errors.New("collaborator not found")
	ErrInvitationNotFound      = errors.New("invitation not found")
	ErrDeletionReasonRequired  = errors.New("deletion reason is required")
)
