package entity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// PresignedURL représente une URL pré-signée pour accéder à un fichier
type PresignedURL struct {
	FileID    string    `json:"file_id"`
	UserID    string    `json:"user_id"`
	FilePath  string    `json:"file_path"`
	Signature string    `json:"signature"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewPresignedURL crée une nouvelle URL pré-signée
func NewPresignedURL(fileID, userID, filePath string, ttl time.Duration) *PresignedURL {
	return &PresignedURL{
		FileID:    fileID,
		UserID:    userID,
		FilePath:  filePath,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// GenerateSignature génère une signature HMAC-SHA256
func (p *PresignedURL) GenerateSignature(secretKey string) string {
	data := fmt.Sprintf("%s:%s:%s:%d", p.FileID, p.UserID, p.FilePath, p.ExpiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature vérifie si la signature est valide
func (p *PresignedURL) VerifySignature(signature, secretKey string) bool {
	expectedSignature := p.GenerateSignature(secretKey)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// IsExpired vérifie si l'URL a expiré
func (p *PresignedURL) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}
