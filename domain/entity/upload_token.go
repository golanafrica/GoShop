package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// UPLOAD TOKEN - Domain Entity
// ============================================================
//
// 🎯 Objectif :
//   Lier cryptographiquement un upload à une soumission KYC.
//   Empêche les attaques IDOR et path traversal.
//
// 🔐 Règles métier :
//   - Token unique (UUID v4)
//   - Expiration : 1 heure
//   - Usage unique (marqué used=true après utilisation)
//   - Lié à un user_id (anti-cross-tenant)
//
// ============================================================

// UploadToken représente un token d'upload sécurisé
type UploadToken struct {
	ID        string    `json:"id" db:"id"`                 // Token UUID
	UserID    string    `json:"user_id" db:"user_id"`       // User qui a uploadé
	FilePath  string    `json:"file_path" db:"file_path"`   // Chemin du fichier
	FileName  string    `json:"file_name" db:"file_name"`   // Nom original
	FileSize  int64     `json:"file_size" db:"file_size"`   // Taille en octets
	MimeType  string    `json:"mime_type" db:"mime_type"`   // Type MIME détecté
	Used      bool      `json:"used" db:"used"`             // Déjà utilisé ?
	CreatedAt time.Time `json:"created_at" db:"created_at"` // Date création
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"` // Date expiration
}

// NewUploadToken crée un nouveau token d'upload
func NewUploadToken(userID, filePath, fileName string, fileSize int64, mimeType string) (*UploadToken, error) {
	// Validations métier
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if filePath == "" {
		return nil, errors.New("file_path is required")
	}
	if fileName == "" {
		return nil, errors.New("file_name is required")
	}
	if fileSize <= 0 {
		return nil, errors.New("file_size must be positive")
	}
	if mimeType == "" {
		return nil, errors.New("mime_type is required")
	}

	now := time.Now().UTC()
	return &UploadToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		FilePath:  filePath,
		FileName:  fileName,
		FileSize:  fileSize,
		MimeType:  mimeType,
		Used:      false,
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour), // 1h de validité
	}, nil
}

// IsValid vérifie si le token est valide (non expiré, non utilisé)
func (t *UploadToken) IsValid() bool {
	if t.Used {
		return false
	}
	return time.Now().Before(t.ExpiresAt)
}

// MarkUsed marque le token comme utilisé (usage unique)
func (t *UploadToken) MarkUsed() {
	t.Used = true
}

// MatchesFilePath vérifie si le token correspond au FilePath fourni
func (t *UploadToken) MatchesFilePath(filePath string) bool {
	return t.FilePath == filePath
}

// MatchesUserID vérifie si le token appartient à l'utilisateur
func (t *UploadToken) MatchesUserID(userID string) bool {
	return t.UserID == userID
}

// IsExpired vérifie si le token a expiré
func (t *UploadToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
