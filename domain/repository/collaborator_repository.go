package repository

//go:generate mockgen -destination=../../mocks/repository/mock_platform_collaborator_repository.go -package=repository . PlatformCollaboratorRepository
//go:generate mockgen -destination=../../mocks/repository/mock_shop_collaborator_repository.go -package=repository . ShopCollaboratorRepository
//go:generate mockgen -destination=../../mocks/repository/mock_collaborator_invitation_repository.go -package=repository . CollaboratorInvitationRepository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_platform_collaborator_repository.go -package=repository . PlatformCollaboratorRepository
//go:generate mockgen -destination=../../mocks/repository/mock_shop_collaborator_repository.go -package=repository . ShopCollaboratorRepository
//go:generate mockgen -destination=../../mocks/repository/mock_collaborator_invitation_repository.go -package=repository . CollaboratorInvitationRepository

// ============================================================
// 🆕 v4.3.0 : PLATFORM COLLABORATOR REPOSITORY (Niveau 1)
// ============================================================

// PlatformCollaboratorRepository définit les opérations sur les collaborateurs plateforme
type PlatformCollaboratorRepository interface {
	// ============ CRUD ============

	// Create crée un nouveau collaborateur plateforme
	Create(ctx context.Context, collab *entity.PlatformCollaborator) error

	// FindByID retourne un collaborateur par son ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.PlatformCollaborator, error)

	// FindByUserID retourne un collaborateur par son user_id (unique)
	FindByUserID(ctx context.Context, userID string) (*entity.PlatformCollaborator, error)

	// FindAll retourne tous les collaborateurs actifs avec pagination
	FindAll(ctx context.Context, limit, offset int) ([]*entity.PlatformCollaborator, int, error)

	// FindByRole retourne les collaborateurs par rôle
	FindByRole(ctx context.Context, role entity.PlatformRole, limit, offset int) ([]*entity.PlatformCollaborator, int, error)

	// Update met à jour un collaborateur
	Update(ctx context.Context, collab *entity.PlatformCollaborator) error

	// Deactivate désactive un collaborateur (soft delete avec audit)
	Deactivate(ctx context.Context, id uuid.UUID, deletedBy, reason string) error

	// Reactivate réactive un collaborateur
	Reactivate(ctx context.Context, id uuid.UUID) error

	// ============ MÉTHODES SPÉCIFIQUES ============

	// UpdateLastLogin met à jour la dernière connexion
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error

	// UpdateRole change le rôle et les permissions
	UpdateRole(ctx context.Context, id uuid.UUID, role entity.PlatformRole, permissions entity.PlatformPermissions) error

	// HasPermission vérifie si un user a une permission spécifique
	HasPermission(ctx context.Context, userID string, permission string) (bool, error)

	// CountActive compte les collaborateurs actifs
	CountActive(ctx context.Context) (int, error)

	// CountByRole compte les collaborateurs par rôle
	CountByRole(ctx context.Context) (map[entity.PlatformRole]int, error)

	// IsPlatformCollaborator vérifie si un user est collaborateur plateforme
	IsPlatformCollaborator(ctx context.Context, userID string) (bool, error)

	WithTX(tx Tx) PlatformCollaboratorRepository
}

// ============================================================
// 🆕 v4.3.0 : SHOP COLLABORATOR REPOSITORY (Niveau 2)
// ============================================================

// ShopCollaboratorRepository définit les opérations sur les collaborateurs boutique
type ShopCollaboratorRepository interface {
	// ============ CRUD ============

	// Create crée un nouveau collaborateur boutique
	Create(ctx context.Context, collab *entity.ShopCollaborator) error

	// FindByID retourne un collaborateur par son ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopCollaborator, error)

	// FindByShopIDAndUserID retourne un collaborateur par shop_id et user_id (unique)
	FindByShopIDAndUserID(ctx context.Context, shopID uuid.UUID, userID string) (*entity.ShopCollaborator, error)

	// FindByShopID retourne tous les collaborateurs d'une boutique
	FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.ShopCollaborator, int, error)

	// FindByUserID retourne tous les collaborateurs d'un user (multi-boutique)
	FindByUserID(ctx context.Context, userID string) ([]*entity.ShopCollaborator, error)

	// FindByRole retourne les collaborateurs par rôle dans une boutique
	FindByRole(ctx context.Context, shopID uuid.UUID, role entity.ShopRole) ([]*entity.ShopCollaborator, error)

	// Update met à jour un collaborateur
	Update(ctx context.Context, collab *entity.ShopCollaborator) error

	// Deactivate désactive un collaborateur (soft delete avec audit)
	Deactivate(ctx context.Context, id uuid.UUID, deletedBy, reason string) error

	// DeactivateByShopIDAndUserID désactive un collaborateur spécifique
	DeactivateByShopIDAndUserID(ctx context.Context, shopID uuid.UUID, userID string, deletedBy, reason string) error

	// Reactivate réactive un collaborateur
	Reactivate(ctx context.Context, id uuid.UUID) error

	// ============ MÉTHODES SPÉCIFIQUES ============

	// UpdateLastLogin met à jour la dernière connexion
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error

	// UpdateRole change le rôle et les permissions
	UpdateRole(ctx context.Context, id uuid.UUID, role entity.ShopRole, permissions entity.ShopPermissions) error

	// HasPermission vérifie si un user a une permission sur une boutique
	HasPermission(ctx context.Context, userID string, shopID uuid.UUID, permission string) (bool, error)

	// CountByShopID compte les collaborateurs actifs d'une boutique
	CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error)

	// CountByUserID compte les boutiques où un user est collaborateur
	CountByUserID(ctx context.Context, userID string) (int, error)

	// IsShopCollaborator vérifie si un user est collaborateur d'une boutique
	IsShopCollaborator(ctx context.Context, userID string, shopID uuid.UUID) (bool, error)

	// IsShopAdmin vérifie si un user est shop_admin d'une boutique
	IsShopAdmin(ctx context.Context, userID string, shopID uuid.UUID) (bool, error)

	// DeleteAllByShopID supprime tous les collaborateurs d'une boutique (cascade)
	DeleteAllByShopID(ctx context.Context, shopID uuid.UUID) error

	WithTX(tx Tx) ShopCollaboratorRepository
}

// ============================================================
// 🆕 v4.3.0 : COLLABORATOR INVITATION REPOSITORY
// ============================================================

// CollaboratorInvitationRepository définit les opérations sur les invitations
type CollaboratorInvitationRepository interface {
	// ============ CRUD ============

	// Create crée une nouvelle invitation
	Create(ctx context.Context, invitation *entity.CollaboratorInvitation) error

	// FindByID retourne une invitation par son ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.CollaboratorInvitation, error)

	// FindByToken retourne une invitation par son token (pour acceptation)
	FindByToken(ctx context.Context, token string) (*entity.CollaboratorInvitation, error)

	// FindByEmail retourne toutes les invitations pour un email
	FindByEmail(ctx context.Context, email string) ([]*entity.CollaboratorInvitation, error)

	// FindPending retourne toutes les invitations en attente
	FindPending(ctx context.Context, limit, offset int) ([]*entity.CollaboratorInvitation, int, error)

	// FindPendingByShopID retourne les invitations en attente pour une boutique
	FindPendingByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.CollaboratorInvitation, error)

	// FindPendingByType retourne les invitations en attente par type
	FindPendingByType(ctx context.Context, invType entity.InvitationType) ([]*entity.CollaboratorInvitation, error)

	// Update met à jour une invitation
	Update(ctx context.Context, invitation *entity.CollaboratorInvitation) error

	// Delete supprime une invitation (hard delete)
	Delete(ctx context.Context, id uuid.UUID) error

	// ============ MÉTHODES SPÉCIFIQUES ============

	// MarkAccepted marque une invitation comme acceptée
	MarkAccepted(ctx context.Context, token string) error

	// Cancel annule une invitation
	Cancel(ctx context.Context, id uuid.UUID) error

	// CleanupExpired nettoie les invitations expirées
	CleanupExpired(ctx context.Context) (int, error)

	// CountPending compte les invitations en attente
	CountPending(ctx context.Context) (int, error)

	// CountByStatus compte les invitations par statut
	CountByStatus(ctx context.Context) (map[entity.InvitationStatus]int, error)

	// IsTokenValid vérifie si un token est valide (existe + pas expiré + pending)
	IsTokenValid(ctx context.Context, token string) (bool, error)

	// FindByInvitedBy retourne les invitations envoyées par un user
	FindByInvitedBy(ctx context.Context, invitedBy string) ([]*entity.CollaboratorInvitation, error)

	WithTX(tx Tx) CollaboratorInvitationRepository
}

// ============================================================
// 🆕 v4.3.0 : FILTRES POUR RECHERCHE
// ============================================================

// PlatformCollaboratorFilters représente les filtres pour la recherche
type PlatformCollaboratorFilters struct {
	Role      *entity.PlatformRole
	IsActive  *bool
	Search    string // Recherche sur email
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// ShopCollaboratorFilters représente les filtres pour la recherche
type ShopCollaboratorFilters struct {
	ShopID    *uuid.UUID
	UserID    *string
	Role      *entity.ShopRole
	IsActive  *bool
	Search    string // Recherche sur email
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// InvitationFilters représente les filtres pour la recherche
type InvitationFilters struct {
	Type      *entity.InvitationType
	ShopID    *uuid.UUID
	Email     *string
	Status    *entity.InvitationStatus
	InvitedBy *string
	Limit     int
	Offset    int
}
