package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ============================================================
// 🆕 v4.4.2 : USER SESSION ENTITY
// 🆕 v4.4.3 : ParseUserAgent amélioré (ordre de priorité corrigé)
// ============================================================
//
// 🎯 Objectif :
//   Représenter une session utilisateur active avec :
//   - Hash SHA-256 du token JWT (sécurité)
//   - Device info flexible (JSONB)
//   - Statuts dynamiques (active/away/idle)
//   - Audit trail (revocation)
//
// 🔐 Sécurité :
//   - Token jamais stocké en clair
//   - Hash SHA-256 avec salt implicite (token lui-même)
//   - Expiration automatique (7 jours par défaut)
//   - Revocation avec audit (qui, quand)
//
// 📋 Règles métier :
//   - 1 session = 1 token unique (session_id = jti JWT)
//   - Statut calculé dynamiquement selon last_activity
//   - Nettoyage automatique des sessions expirées
//
// ============================================================

// ============================================================
// CONSTANTES
// ============================================================

const (
	// Durée de vie des sessions
	DefaultSessionDuration = 7 * 24 * time.Hour  // 7 jours
	MaxSessionDuration     = 30 * 24 * time.Hour // 30 jours max

	// Seuils de statut
	IdleThresholdMinutes     = 15 // Après 15 min → away
	LongIdleThresholdMinutes = 60 // Après 60 min → idle

	// User-Agent parsing
	UnknownDevice = "Unknown"
)

// ============================================================
// ERREURS
// ============================================================

var (
	ErrSessionNotFound       = errors.New("session not found")
	ErrSessionExpired        = errors.New("session has expired")
	ErrSessionRevoked        = errors.New("session has been revoked")
	ErrSessionInvalidToken   = errors.New("invalid session token")
	ErrSessionAlreadyActive  = errors.New("session is already active")
	ErrSessionUserIDRequired = errors.New("user_id is required")
)

// ============================================================
// STRUCTURES DE DONNÉES
// ============================================================

// DeviceInfo représente les informations sur l'appareil
type DeviceInfo struct {
	UserAgent string `json:"user_agent"`
	Browser   string `json:"browser,omitempty"`
	OS        string `json:"os,omitempty"`
	Device    string `json:"device,omitempty"`
	Platform  string `json:"platform,omitempty"`
}

// UserSession représente une session utilisateur active
type UserSession struct {
	// Identification
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"` // jti dans JWT

	// Sécurité
	SessionTokenHash string `json:"-"` // Jamais sérialisé

	// Device info
	DeviceInfo DeviceInfo `json:"device_info"`

	// Localisation
	IPAddress string `json:"ip_address"`

	// Activité
	LastActivity time.Time `json:"last_activity"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`

	// Statut
	IsActive  bool       `json:"is_active"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	RevokedBy string     `json:"revoked_by,omitempty"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewUserSession crée une nouvelle session
func NewUserSession(userID, sessionID, token, ipAddress, userAgent string, duration time.Duration) (*UserSession, error) {
	if userID == "" {
		return nil, ErrSessionUserIDRequired
	}

	if duration <= 0 {
		duration = DefaultSessionDuration
	}
	if duration > MaxSessionDuration {
		duration = MaxSessionDuration
	}

	now := time.Now()
	expiresAt := now.Add(duration)

	// Hasher le token avec SHA-256
	tokenHash := HashToken(token)

	// Parser le user agent
	deviceInfo := ParseUserAgent(userAgent)

	return &UserSession{
		UserID:           userID,
		SessionID:        sessionID,
		SessionTokenHash: tokenHash,
		DeviceInfo:       deviceInfo,
		IPAddress:        ipAddress,
		LastActivity:     now,
		CreatedAt:        now,
		ExpiresAt:        expiresAt,
		IsActive:         true,
	}, nil
}

// ============================================================
// MÉTHODES DE STATUT
// ============================================================

// GetStatus retourne le statut actuel de la session
func (s *UserSession) GetStatus() string {
	// Session révoquée
	if !s.IsActive {
		return "revoked"
	}

	// Session expirée
	if s.IsExpired() {
		return "expired"
	}

	// Calculer le temps d'inactivité
	minutesInactive := time.Since(s.LastActivity).Minutes()

	switch {
	case minutesInactive > LongIdleThresholdMinutes:
		return "idle"
	case minutesInactive > IdleThresholdMinutes:
		return "away"
	default:
		return "active"
	}
}

// IsActiveSession vérifie si la session est active et valide
func (s *UserSession) IsActiveSession() bool {
	return s.IsActive && !s.IsExpired()
}

// IsExpired vérifie si la session a expiré
func (s *UserSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsRevoked vérifie si la session a été révoquée
func (s *UserSession) IsRevoked() bool {
	return !s.IsActive && s.RevokedAt != nil
}

// IsIdle vérifie si la session est inactive depuis longtemps
func (s *UserSession) IsIdle() bool {
	return time.Since(s.LastActivity).Minutes() > LongIdleThresholdMinutes
}

// GetMinutesInactive retourne le nombre de minutes d'inactivité
func (s *UserSession) GetMinutesInactive() float64 {
	return time.Since(s.LastActivity).Minutes()
}

// GetTimeRemaining retourne le temps restant avant expiration
func (s *UserSession) GetTimeRemaining() time.Duration {
	remaining := time.Until(s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ============================================================
// MÉTHODES D'AUDIT
// ============================================================

// MarkRevoked marque la session comme révoquée
func (s *UserSession) MarkRevoked(revokedBy string) {
	now := time.Now()
	s.IsActive = false
	s.RevokedAt = &now
	s.RevokedBy = revokedBy
}

// UpdateLastActivity met à jour la dernière activité
func (s *UserSession) UpdateLastActivity() {
	s.LastActivity = time.Now()
}

// ExtendSession étend la durée de la session
func (s *UserSession) ExtendSession(duration time.Duration) error {
	if !s.IsActiveSession() {
		return ErrSessionExpired
	}

	if duration <= 0 {
		duration = DefaultSessionDuration
	}

	newExpires := time.Now().Add(duration)
	if newExpires.Sub(s.CreatedAt) > MaxSessionDuration {
		return errors.New("session duration exceeds maximum allowed")
	}

	s.ExpiresAt = newExpires
	s.LastActivity = time.Now()
	return nil
}

// ============================================================
// MÉTHODES DE SÉCURITÉ
// ============================================================

// HashToken génère un hash SHA-256 du token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ValidateToken vérifie si le token correspond au hash stocké
func (s *UserSession) ValidateToken(token string) bool {
	tokenHash := HashToken(token)
	return s.SessionTokenHash == tokenHash
}

// ============================================================
// 🆕 v4.4.3 : MÉTHODES UTILITAIRES - ParseUserAgent AMÉLIORÉ
// ============================================================

// ParseUserAgent parse le user agent pour extraire les infos device
// 🆕 v4.4.3 : Ordre de priorité corrigé pour une détection plus précise
func ParseUserAgent(userAgent string) DeviceInfo {
	info := DeviceInfo{
		UserAgent: userAgent,
		Browser:   UnknownDevice,
		OS:        UnknownDevice,
		Device:    UnknownDevice,
	}

	if userAgent == "" {
		return info
	}

	ua := strings.ToLower(userAgent)

	// ============================================================
	// 🆕 v4.4.3 : NAVIGATEURS - Ordre de priorité corrigé
	// ============================================================
	// Les navigateurs basés sur Chromium (Opera, Edge, Brave) contiennent
	// "chrome" dans leur UA. Il faut les détecter AVANT Chrome.
	// Ordre : Opera → Edge → Chrome → Firefox → Safari → IE
	switch {
	case strings.Contains(ua, "opr") || strings.Contains(ua, "opera"):
		info.Browser = "Opera"
	case strings.Contains(ua, "edg"):
		info.Browser = "Edge"
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "crios"):
		info.Browser = "Chrome"
	case strings.Contains(ua, "firefox") || strings.Contains(ua, "fxios"):
		info.Browser = "Firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		info.Browser = "Safari"
	case strings.Contains(ua, "msie") || strings.Contains(ua, "trident"):
		info.Browser = "Internet Explorer"
	}

	// ============================================================
	// 🆕 v4.4.3 : OS - Ordre de priorité corrigé
	// ============================================================
	// iPad/iPhone contiennent "Mac OS X" dans leur UA moderne.
	// Il faut détecter iOS AVANT macOS.
	// Ordre : iOS → Android → Windows → macOS → Linux
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ipod"):
		info.OS = "iOS"
	case strings.Contains(ua, "android"):
		info.OS = "Android"
	case strings.Contains(ua, "windows"):
		info.OS = "Windows"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macintosh"):
		info.OS = "macOS"
	case strings.Contains(ua, "linux"):
		info.OS = "Linux"
	}

	// ============================================================
	// 🆕 v4.4.3 : DEVICE - Ordre de priorité corrigé
	// ============================================================
	// iPad contient "Mobile" dans son UA. Il faut détecter Tablet AVANT Mobile.
	// Ordre : Tablet → Mobile → Desktop
	switch {
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") || strings.Contains(ua, "sm-t") || strings.Contains(ua, "kindle"):
		info.Device = "Tablet"
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "pixel") || strings.Contains(ua, "sm-"):
		info.Device = "Mobile"
	default:
		info.Device = "Desktop"
	}

	// ============================================================
	// PLATEFORME
	// ============================================================
	switch {
	case strings.Contains(ua, "windows"):
		info.Platform = "Windows"
	case strings.Contains(ua, "mac") || strings.Contains(ua, "ipad") || strings.Contains(ua, "iphone"):
		info.Platform = "Mac"
	case strings.Contains(ua, "linux") && !strings.Contains(ua, "android"):
		info.Platform = "Linux"
	case strings.Contains(ua, "android"):
		info.Platform = "Android"
	}

	return info
}

// DeviceInfoToJSON convertit DeviceInfo en JSON string
func (d *DeviceInfo) ToJSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}

// DeviceInfoFromJSON parse un JSON string en DeviceInfo
func DeviceInfoFromJSON(jsonStr string) (DeviceInfo, error) {
	var info DeviceInfo
	if jsonStr == "" {
		return DeviceInfo{}, nil
	}
	err := json.Unmarshal([]byte(jsonStr), &info)
	return info, err
}

// ============================================================
// MÉTHODES D'AFFICHAGE
// ============================================================

// ToSummary retourne un résumé de la session pour l'API
type UserSessionSummary struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	UserID          string    `json:"user_id"`
	Browser         string    `json:"browser"`
	OS              string    `json:"os"`
	Device          string    `json:"device"`
	IPAddress       string    `json:"ip_address"`
	Status          string    `json:"status"`
	LastActivity    time.Time `json:"last_activity"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	MinutesInactive float64   `json:"minutes_inactive"`
	IsCurrent       bool      `json:"is_current"` // true si c'est la session actuelle
}

// ToSummary convertit l'entité en résumé API
func (s *UserSession) ToSummary(currentSessionID string) *UserSessionSummary {
	return &UserSessionSummary{
		ID:              s.ID,
		SessionID:       s.SessionID,
		UserID:          s.UserID,
		Browser:         s.DeviceInfo.Browser,
		OS:              s.DeviceInfo.OS,
		Device:          s.DeviceInfo.Device,
		IPAddress:       s.IPAddress,
		Status:          s.GetStatus(),
		LastActivity:    s.LastActivity,
		CreatedAt:       s.CreatedAt,
		ExpiresAt:       s.ExpiresAt,
		MinutesInactive: s.GetMinutesInactive(),
		IsCurrent:       s.SessionID == currentSessionID,
	}
}

// ============================================================
// STATISTIQUES
// ============================================================

// SessionStatistics contient les statistiques globales des sessions
type SessionStatistics struct {
	TotalActive       int     `json:"total_active"`
	TotalRevoked      int     `json:"total_revoked"`
	TotalExpired      int     `json:"total_expired"`
	UniqueUsersActive int     `json:"unique_users_active"`
	AvgIdleMinutes    float64 `json:"avg_idle_minutes"`
}
