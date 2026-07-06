package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ============================================================
// 🆕 v4.4.3 : API KEY ENTITY
// ============================================================
//
// 🎯 Objectif :
//   Représenter une clé API pour accès programmatique avec :
//   - Génération sécurisée (crypto/rand)
//   - Préfixes lisibles (gsk_live_, gsk_test_)
//   - Hash SHA-256 (jamais stocker en clair)
//   - Scopes granulaires (permissions)
//   - Expiration configurable
//   - Rate limiting
//
// 🔐 Sécurité :
//   - 32 bytes d'entropie (256 bits)
//   - Hash SHA-256 en base
//   - Affichage complet UNE SEULE FOIS
//   - Révocation immédiate
//
// 📋 Règles métier :
//   - Préfixe obligatoire (gsk_live_ ou gsk_test_)
//   - Clé complète = préfixe + 48 caractères aléatoires
//   - Scopes multiples autorisés
//   - Expiration optionnelle (NULL = pas d'expiration)
//
// ============================================================

// ============================================================
// CONSTANTES
// ============================================================

const (
	// Préfixes des clés API
	APIKeyPrefixLive = "gsk_live_"
	APIKeyPrefixTest = "gsk_test_"

	// Longueur de la partie aléatoire (après le préfixe)
	APIKeyRandomLength = 48

	// Durée de vie par défaut
	APIKeyDefaultLifetime = 365 * 24 * time.Hour     // 1 an
	APIKeyMaxLifetime     = 5 * 365 * 24 * time.Hour // 5 ans max

	// Rate limiting par défaut
	APIKeyDefaultRateLimitMinute = 60
	APIKeyDefaultRateLimitDay    = 10000
	APIKeyMaxRateLimitMinute     = 1000
	APIKeyMaxRateLimitDay        = 100000
)

// ============================================================
// ERREURS
// ============================================================

var (
	ErrAPIKeyNotFound          = errors.New("api key not found")
	ErrAPIKeyExpired           = errors.New("api key has expired")
	ErrAPIKeyRevoked           = errors.New("api key has been revoked")
	ErrAPIKeyInvalid           = errors.New("invalid api key format")
	ErrAPIKeyInsufficientScope = errors.New("api key does not have required scope")
	ErrAPIKeyNameRequired      = errors.New("api key name is required")
	ErrAPIKeyAlreadyRevoked    = errors.New("api key is already revoked")
	ErrAPIKeyInvalidPrefix     = errors.New("invalid api key prefix (must be gsk_live_ or gsk_test_)")
)

// ============================================================
// TYPE SCOPES
// ============================================================

// APIKeyScope représente un scope (permission) pour une clé API
type APIKeyScope string

// Scopes disponibles
const (
	ScopeReadShops      APIKeyScope = "read:shops"
	ScopeWriteShops     APIKeyScope = "write:shops"
	ScopeReadOrders     APIKeyScope = "read:orders"
	ScopeWriteOrders    APIKeyScope = "write:orders"
	ScopeReadProducts   APIKeyScope = "read:products"
	ScopeWriteProducts  APIKeyScope = "write:products"
	ScopeReadCustomers  APIKeyScope = "read:customers"
	ScopeWriteCustomers APIKeyScope = "write:customers"
	ScopeReadPayments   APIKeyScope = "read:payments"
	ScopeWritePayments  APIKeyScope = "write:payments"
	ScopeReadWallet     APIKeyScope = "read:wallet"
	ScopeWriteWallet    APIKeyScope = "write:wallet"
	ScopeReadTontine    APIKeyScope = "read:tontine"
	ScopeWriteTontine   APIKeyScope = "write:tontine"
	ScopeReadCredit     APIKeyScope = "read:credit"
	ScopeWriteCredit    APIKeyScope = "write:credit"
	ScopeReadReports    APIKeyScope = "read:reports"
	ScopeAdmin          APIKeyScope = "admin"
)

// AllScopes retourne tous les scopes disponibles
func AllScopes() []APIKeyScope {
	return []APIKeyScope{
		ScopeReadShops, ScopeWriteShops,
		ScopeReadOrders, ScopeWriteOrders,
		ScopeReadProducts, ScopeWriteProducts,
		ScopeReadCustomers, ScopeWriteCustomers,
		ScopeReadPayments, ScopeWritePayments,
		ScopeReadWallet, ScopeWriteWallet,
		ScopeReadTontine, ScopeWriteTontine,
		ScopeReadCredit, ScopeWriteCredit,
		ScopeReadReports,
		ScopeAdmin,
	}
}

// IsValidScope vérifie si un scope est valide
func IsValidScope(scope string) bool {
	for _, s := range AllScopes() {
		if string(s) == scope {
			return true
		}
	}
	return false
}

// ============================================================
// STRUCTURE PRINCIPALE
// ============================================================

// APIKey représente une clé API pour accès programmatique
type APIKey struct {
	// Identification
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Informations
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Sécurité
	KeyPrefix string `json:"key_prefix"` // Affiché dans l'UI
	KeyHash   string `json:"-"`          // Jamais sérialisé

	// Permissions
	Scopes []APIKeyScope `json:"scopes"`

	// Expiration et statut
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	IsActive   bool       `json:"is_active"`

	// Rate limiting
	RateLimitPerMinute int `json:"rate_limit_per_minute"`
	RateLimitPerDay    int `json:"rate_limit_per_day"`

	// Révocation
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevokedBy        string     `json:"revoked_by,omitempty"`
	RevocationReason string     `json:"revocation_reason,omitempty"`

	// Audit
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedIP        string    `json:"created_ip,omitempty"`
	CreatedUserAgent string    `json:"created_user_agent,omitempty"`

	// Champ temporaire (non persisté) : clé complète affichée UNE SEULE FOIS
	TemporaryFullKey string `json:"temporary_full_key,omitempty"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// GenerateAPIKey génère une nouvelle clé API complète
// Retourne : (clé complète, préfixe, hash, erreur)
func GenerateAPIKey(isTest bool) (string, string, string, error) {
	// Déterminer le préfixe
	prefix := APIKeyPrefixLive
	if isTest {
		prefix = APIKeyPrefixTest
	}

	// Générer 48 caractères aléatoires (base36 pour lisibilité)
	randomBytes := make([]byte, 36) // 36 bytes → 48 caractères base36
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	// Encoder en hexadécimal (plus lisible que base64)
	randomPart := hex.EncodeToString(randomBytes)[:APIKeyRandomLength]

	// Construire la clé complète
	fullKey := prefix + randomPart

	// Hasher la clé
	keyHash := HashAPIKey(fullKey)

	return fullKey, prefix, keyHash, nil
}

// HashAPIKey génère le hash SHA-256 d'une clé API
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// NewAPIKey crée une nouvelle instance de APIKey
func NewAPIKey(
	userID, name, description string,
	scopes []APIKeyScope,
	isTest bool,
	lifetime time.Duration,
	ipAddress, userAgent string,
) (*APIKey, string, error) {
	// Validation du nom
	if name == "" {
		return nil, "", ErrAPIKeyNameRequired
	}

	// Validation des scopes
	if len(scopes) == 0 {
		return nil, "", errors.New("at least one scope is required")
	}

	for _, scope := range scopes {
		if !IsValidScope(string(scope)) {
			return nil, "", fmt.Errorf("invalid scope: %s", scope)
		}
	}

	// Validation de la durée de vie
	if lifetime <= 0 {
		lifetime = APIKeyDefaultLifetime
	}
	if lifetime > APIKeyMaxLifetime {
		lifetime = APIKeyMaxLifetime
	}

	// Générer la clé
	fullKey, prefix, keyHash, err := GenerateAPIKey(isTest)
	if err != nil {
		return nil, "", err
	}

	// Calculer la date d'expiration
	now := time.Now()
	expiresAt := now.Add(lifetime)

	return &APIKey{
		UserID:             userID,
		Name:               name,
		Description:        description,
		KeyPrefix:          prefix + fullKey[len(prefix):len(prefix)+12], // 12 premiers caractères après préfixe
		KeyHash:            keyHash,
		Scopes:             scopes,
		ExpiresAt:          &expiresAt,
		IsActive:           true,
		RateLimitPerMinute: APIKeyDefaultRateLimitMinute,
		RateLimitPerDay:    APIKeyDefaultRateLimitDay,
		CreatedAt:          now,
		UpdatedAt:          now,
		CreatedIP:          ipAddress,
		CreatedUserAgent:   userAgent,
		TemporaryFullKey:   fullKey, // ⚠️ Affiché UNE SEULE FOIS
	}, fullKey, nil
}

// ============================================================
// MÉTHODES DE VALIDATION
// ============================================================

// ValidateKey validates the API key format
func ValidateKey(key string) error {
	if len(key) < len(APIKeyPrefixLive)+APIKeyRandomLength {
		return ErrAPIKeyInvalid
	}

	if !strings.HasPrefix(key, APIKeyPrefixLive) && !strings.HasPrefix(key, APIKeyPrefixTest) {
		return ErrAPIKeyInvalidPrefix
	}

	return nil
}

// IsExpired vérifie si la clé a expiré
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false // Pas d'expiration
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsRevoked vérifie si la clé a été révoquée
func (k *APIKey) IsRevoked() bool {
	return !k.IsActive && k.RevokedAt != nil
}

// IsValid vérifie si la clé est valide (active + non expirée)
func (k *APIKey) IsValid() bool {
	return k.IsActive && !k.IsExpired() && !k.IsRevoked()
}

// HasScope vérifie si la clé a le scope requis
func (k *APIKey) HasScope(requiredScope APIKeyScope) bool {
	// Le scope "admin" donne accès à tout
	for _, scope := range k.Scopes {
		if scope == ScopeAdmin {
			return true
		}
		if scope == requiredScope {
			return true
		}
	}
	return false
}

// HasAnyScope vérifie si la clé a au moins un des scopes requis
func (k *APIKey) HasAnyScope(requiredScopes ...APIKeyScope) bool {
	for _, required := range requiredScopes {
		if k.HasScope(required) {
			return true
		}
	}
	return false
}

// HasAllScopes vérifie si la clé a tous les scopes requis
func (k *APIKey) HasAllScopes(requiredScopes ...APIKeyScope) bool {
	for _, required := range requiredScopes {
		if !k.HasScope(required) {
			return false
		}
	}
	return true
}

// CheckScope vérifie le scope et retourne une erreur si insuffisant
func (k *APIKey) CheckScope(requiredScope APIKeyScope) error {
	if !k.IsValid() {
		if k.IsRevoked() {
			return ErrAPIKeyRevoked
		}
		return ErrAPIKeyExpired
	}

	if !k.HasScope(requiredScope) {
		return ErrAPIKeyInsufficientScope
	}

	return nil
}

// ============================================================
// MÉTHODES D'AUDIT
// ============================================================

// MarkRevoked marque la clé comme révoquée
func (k *APIKey) MarkRevoked(revokedBy, reason string) error {
	if k.IsRevoked() {
		return ErrAPIKeyAlreadyRevoked
	}

	now := time.Now()
	k.IsActive = false
	k.RevokedAt = &now
	k.RevokedBy = revokedBy
	k.RevocationReason = reason
	k.UpdatedAt = now

	return nil
}

// MarkUsed enregistre l'utilisation de la clé
func (k *APIKey) MarkUsed() {
	now := time.Now()
	k.LastUsedAt = &now
}

// ExtendExpiration étend la date d'expiration
func (k *APIKey) ExtendExpiration(duration time.Duration) error {
	if !k.IsActive {
		return errors.New("cannot extend revoked key")
	}

	if duration <= 0 {
		duration = APIKeyDefaultLifetime
	}

	newExpires := time.Now().Add(duration)
	if newExpires.Sub(k.CreatedAt) > APIKeyMaxLifetime {
		return errors.New("extension would exceed maximum lifetime")
	}

	k.ExpiresAt = &newExpires
	k.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// MaskKey retourne une version masquée de la clé pour affichage
// Exemple : "gsk_live_a1b2...x9y8"
func (k *APIKey) MaskKey() string {
	if k.KeyPrefix == "" {
		return "***"
	}
	return k.KeyPrefix + "..."
}

// GetStatus retourne le statut de la clé
func (k *APIKey) GetStatus() string {
	switch {
	case k.IsRevoked():
		return "revoked"
	case k.IsExpired():
		return "expired"
	case k.LastUsedAt == nil:
		return "unused"
	case time.Since(*k.LastUsedAt) > 30*24*time.Hour:
		return "inactive"
	default:
		return "active"
	}
}

// GetDaysUntilExpiration retourne le nombre de jours avant expiration
func (k *APIKey) GetDaysUntilExpiration() int {
	if k.ExpiresAt == nil {
		return -1 // Pas d'expiration
	}

	days := int(time.Until(*k.ExpiresAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// IsExpiringSoon vérifie si la clé expire dans les 30 prochains jours
func (k *APIKey) IsExpiringSoon() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Until(*k.ExpiresAt) < 30*24*time.Hour
}

// ============================================================
// MÉTHODES D'AFFICHAGE
// ============================================================

// APIKeySummary représente un résumé de la clé pour l'API
type APIKeySummary struct {
	ID                  string        `json:"id"`
	UserID              string        `json:"user_id"`
	Name                string        `json:"name"`
	Description         string        `json:"description,omitempty"`
	KeyPrefix           string        `json:"key_prefix"`
	Scopes              []APIKeyScope `json:"scopes"`
	ExpiresAt           *time.Time    `json:"expires_at,omitempty"`
	LastUsedAt          *time.Time    `json:"last_used_at,omitempty"`
	Status              string        `json:"status"`
	RateLimitPerMinute  int           `json:"rate_limit_per_minute"`
	RateLimitPerDay     int           `json:"rate_limit_per_day"`
	CreatedAt           time.Time     `json:"created_at"`
	DaysUntilExpiration int           `json:"days_until_expiration"`
	IsExpiringSoon      bool          `json:"is_expiring_soon"`
}

// ToSummary convertit l'entité en résumé API
func (k *APIKey) ToSummary() *APIKeySummary {
	return &APIKeySummary{
		ID:                  k.ID,
		UserID:              k.UserID,
		Name:                k.Name,
		Description:         k.Description,
		KeyPrefix:           k.MaskKey(),
		Scopes:              k.Scopes,
		ExpiresAt:           k.ExpiresAt,
		LastUsedAt:          k.LastUsedAt,
		Status:              k.GetStatus(),
		RateLimitPerMinute:  k.RateLimitPerMinute,
		RateLimitPerDay:     k.RateLimitPerDay,
		CreatedAt:           k.CreatedAt,
		DaysUntilExpiration: k.GetDaysUntilExpiration(),
		IsExpiringSoon:      k.IsExpiringSoon(),
	}
}

// APIKeyCreationResponse représente la réponse de création
type APIKeyCreationResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	APIKey  *APIKeySummary `json:"api_key"`
	FullKey string         `json:"full_key"` // ⚠️ UNE SEULE FOIS
	Warning string         `json:"warning"`
}

// ============================================================
// STATISTIQUES
// ============================================================

// APIKeyStatistics contient les statistiques globales
type APIKeyStatistics struct {
	TotalKeys         int `json:"total_keys"`
	ActiveKeys        int `json:"active_keys"`
	RevokedKeys       int `json:"revoked_keys"`
	ExpiredKeys       int `json:"expired_keys"`
	UnusedKeys        int `json:"unused_keys"`
	UniqueUsers       int `json:"unique_users"`
	CallsToday        int `json:"calls_today"`
	CallsLast7Days    int `json:"calls_last_7_days"`
	TotalCallsAllTime int `json:"total_calls_all_time"`
}

// APIKeyUsageLog représente un log d'utilisation
type APIKeyUsageLog struct {
	ID           string    `json:"id"`
	APIKeyID     string    `json:"api_key_id"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	StatusCode   int       `json:"status_code"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent,omitempty"`
	ResponseTime int       `json:"response_time_ms"`
	CreatedAt    time.Time `json:"created_at"`
}
