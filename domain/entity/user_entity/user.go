package userentity

import (
	"errors"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// CONSTANTES RBAC
// ============================================================

// Rôles système
const (
	RoleSuperAdmin    = "super_admin"
	RoleAdmin         = "admin"
	RoleCreditAnalyst = "credit_analyst"
	RoleSupportAgent  = "support_agent"
	RoleModerator     = "moderator"
	RoleMerchant      = "merchant"
)

// Statuts utilisateur
const (
	StatusActive    = "active"
	StatusPending   = "pending"
	StatusSuspended = "suspended"
	StatusBanned    = "banned"
	StatusDeleted   = "deleted"
)

// Seuils de sécurité
const (
	MaxFailedLoginAttempts = 5
	LockoutDurationMinutes = 30
)

// ============================================================
// ERREURS
// ============================================================

var (
	ErrUserInactive      = errors.New("user account is inactive")
	ErrUserSuspended     = errors.New("user account is suspended")
	ErrUserBanned        = errors.New("user account is banned")
	ErrUserLocked        = errors.New("account is temporarily locked due to failed login attempts")
	ErrInvalidRole       = errors.New("invalid role")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrInsufficientPerms = errors.New("insufficient permissions")
)

// ============================================================
// ENTITÉ USER
// ============================================================

// UserEntity représente un utilisateur avec système RBAC
type UserEntity struct {
	// Identifiants
	ID    string `json:"id" db:"id"`
	Email string `json:"email" db:"email"`

	// Sécurité
	Password string `json:"-" db:"password"` // Jamais sérialisé en JSON

	// 🆕 v4.0.0 : RBAC
	Role        string                 `json:"role" db:"role"`
	Active      bool                   `json:"is_active" db:"is_active"` // ✅ Renommé : IsActive → Active
	Status      string                 `json:"status" db:"status"`
	Permissions map[string]interface{} `json:"permissions,omitempty" db:"permissions"`

	// 🆕 v4.0.0 : Sécurité avancée
	LastLoginAt         *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	FailedLoginAttempts int        `json:"failed_login_attempts" db:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until,omitempty" db:"locked_until"`

	// 🆕 v4.0.0 : Audit
	CreatedBy *string   `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy *string   `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewUserEntity crée un nouvel utilisateur avec valeurs par défaut
func NewUserEntity(id, email, password string) (*UserEntity, error) {
	if id == "" {
		return nil, errors.New("id is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}
	if password == "" {
		return nil, errors.New("password is required")
	}

	// Hash du mot de passe
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &UserEntity{
		ID:                  id,
		Email:               email,
		Password:            hashedPassword,
		Role:                RoleMerchant, // Rôle par défaut
		Active:              true,         // ✅ Renommé
		Status:              StatusActive,
		FailedLoginAttempts: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// NewSuperAdmin crée un super administrateur
func NewSuperAdmin(id, email, password string) (*UserEntity, error) {
	user, err := NewUserEntity(id, email, password)
	if err != nil {
		return nil, err
	}
	user.Role = RoleSuperAdmin
	return user, nil
}

// ============================================================
// VALIDATION DES RÔLES
// ============================================================

// IsValidRole vérifie si un rôle est valide
func IsValidRole(role string) bool {
	switch role {
	case RoleSuperAdmin, RoleAdmin, RoleCreditAnalyst,
		RoleSupportAgent, RoleModerator, RoleMerchant:
		return true
	}
	return false
}

// IsValidStatus vérifie si un statut est valide
func IsValidStatus(status string) bool {
	switch status {
	case StatusActive, StatusPending, StatusSuspended, StatusBanned, StatusDeleted:
		return true
	}
	return false
}

// ============================================================
// HELPERS RBAC
// ============================================================

// IsSuperAdmin vérifie si l'utilisateur est super admin
func (u *UserEntity) IsSuperAdmin() bool {
	return u.Role == RoleSuperAdmin
}

// IsAdmin vérifie si l'utilisateur est admin (admin ou super_admin)
func (u *UserEntity) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleSuperAdmin
}

// IsStaff vérifie si l'utilisateur est un membre du staff (pas un marchand)
func (u *UserEntity) IsStaff() bool {
	return u.Role != RoleMerchant
}

// HasRole vérifie si l'utilisateur a un des rôles spécifiés
func (u *UserEntity) HasRole(roles ...string) bool {
	for _, role := range roles {
		if u.Role == role {
			return true
		}
	}
	return false
}

// ============================================================
// HELPERS STATUT
// ============================================================

// IsActive vérifie si l'utilisateur est actif (méthode)
// ✅ Résout le conflit : le champ s'appelle maintenant `Active`
func (u *UserEntity) IsActive() bool {
	return u.Active && u.Status == StatusActive
}

// IsSuspended vérifie si l'utilisateur est suspendu
func (u *UserEntity) IsSuspended() bool {
	return u.Status == StatusSuspended
}

// IsBanned vérifie si l'utilisateur est banni
func (u *UserEntity) IsBanned() bool {
	return u.Status == StatusBanned
}

// IsLocked vérifie si le compte est temporairement verrouillé
func (u *UserEntity) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// CanLogin vérifie si l'utilisateur peut se connecter
func (u *UserEntity) CanLogin() error {
	if !u.Active { // ✅ Utilise le champ Active
		return ErrUserInactive
	}

	switch u.Status {
	case StatusSuspended:
		return ErrUserSuspended
	case StatusBanned:
		return ErrUserBanned
	case StatusDeleted:
		return ErrUserInactive
	}

	if u.IsLocked() {
		return ErrUserLocked
	}

	return nil
}

// ============================================================
// GESTION DES TENTATIVES DE CONNEXION
// ============================================================

// RecordFailedLogin enregistre une tentative de connexion échouée
func (u *UserEntity) RecordFailedLogin() {
	u.FailedLoginAttempts++
	u.UpdatedAt = time.Now()

	// Verrouiller le compte si trop de tentatives
	if u.FailedLoginAttempts >= MaxFailedLoginAttempts {
		lockUntil := time.Now().Add(time.Duration(LockoutDurationMinutes) * time.Minute)
		u.LockedUntil = &lockUntil
	}
}

// RecordSuccessfulLogin enregistre une connexion réussie
func (u *UserEntity) RecordSuccessfulLogin() {
	now := time.Now()
	u.LastLoginAt = &now
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = now
}

// ResetLockout réinitialise le verrouillage (par un admin)
func (u *UserEntity) ResetLockout() {
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = time.Now()
}

// ============================================================
// GESTION DU STATUT
// ============================================================

// Suspend suspend l'utilisateur
func (u *UserEntity) Suspend(updatedBy string) error {
	if u.Status == StatusDeleted {
		return errors.New("cannot suspend a deleted user")
	}
	u.Status = StatusSuspended
	u.Active = false // ✅ Renommé
	u.UpdatedBy = &updatedBy
	u.UpdatedAt = time.Now()
	return nil
}

// Activate réactive l'utilisateur
func (u *UserEntity) Activate(updatedBy string) error {
	u.Status = StatusActive
	u.Active = true // ✅ Renommé
	u.UpdatedBy = &updatedBy
	u.UpdatedAt = time.Now()
	return nil
}

// Ban ban l'utilisateur
func (u *UserEntity) Ban(updatedBy string) error {
	u.Status = StatusBanned
	u.Active = false // ✅ Renommé
	u.UpdatedBy = &updatedBy
	u.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// GESTION DES RÔLES
// ============================================================

// SetRole change le rôle de l'utilisateur
func (u *UserEntity) SetRole(newRole string, updatedBy string) error {
	if !IsValidRole(newRole) {
		return ErrInvalidRole
	}

	// Un super_admin ne peut pas être rétrogradé sans vérification
	if u.Role == RoleSuperAdmin && newRole != RoleSuperAdmin {
		// En production, on pourrait exiger une confirmation supplémentaire
	}

	u.Role = newRole
	u.UpdatedBy = &updatedBy
	u.UpdatedAt = time.Now()
	return nil
}

// PromoteToAdmin promeut l'utilisateur au rang d'admin
func (u *UserEntity) PromoteToAdmin(updatedBy string) error {
	return u.SetRole(RoleAdmin, updatedBy)
}

// ============================================================
// MOT DE PASSE
// ============================================================

// VerifyPassword vérifie si le mot de passe correspond
func (u *UserEntity) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// SetPassword change le mot de passe
func (u *UserEntity) SetPassword(newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hashed, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	u.Password = hashed
	u.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// FONCTIONS UTILITAIRES
// ============================================================

// getBcryptCost retourne le coût bcrypt selon l'environnement
func getBcryptCost() int {
	if costStr := os.Getenv("BCRYPT_COST"); costStr != "" {
		if cost, err := strconv.Atoi(costStr); err == nil && cost >= 4 && cost <= 12 {
			return cost
		}
	}

	// Défaut selon l'environnement
	env := os.Getenv("APP_ENV")
	switch env {
	case "production":
		return 12
	case "staging":
		return 10
	default: // development, test
		return 4 // ⚡ Ultra rapide en dev
	}
}

// HashPassword hashe un mot de passe avec bcrypt
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), getBcryptCost())
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// ============================================================
// MÉTHODES DE CONVERSION
// ============================================================

// ToSafeJSON retourne une version JSON safe (sans password)
func (u *UserEntity) ToSafeJSON() map[string]interface{} {
	result := map[string]interface{}{
		"id":                    u.ID,
		"email":                 u.Email,
		"role":                  u.Role,
		"is_active":             u.Active, // ✅ Utilise le champ Active
		"status":                u.Status,
		"failed_login_attempts": u.FailedLoginAttempts,
		"created_at":            u.CreatedAt,
		"updated_at":            u.UpdatedAt,
	}

	if u.LastLoginAt != nil {
		result["last_login_at"] = u.LastLoginAt
	}

	if u.LockedUntil != nil {
		result["locked_until"] = u.LockedUntil
		result["is_locked"] = u.IsLocked()
	}

	return result
}
