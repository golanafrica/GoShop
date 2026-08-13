package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// IdempotencyKey représente une clé d'idempotence pour prévenir les requêtes dupliquées
type IdempotencyKey struct {
	IdempotencyKey  string                 `json:"idempotency_key" db:"idempotency_key"`
	UserID          string                 `json:"user_id" db:"user_id"`
	Endpoint        string                 `json:"endpoint" db:"endpoint"`
	RequestHash     string                 `json:"request_hash" db:"request_hash"`
	ResponseStatus  int                    `json:"response_status" db:"response_status"`
	ResponseHeaders map[string]string      `json:"response_headers" db:"response_headers"`
	ResponseBody    map[string]interface{} `json:"response_body" db:"response_body"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	ExpiresAt       time.Time              `json:"expires_at" db:"expires_at"`
}

// IsExpired vérifie si la clé d'idempotence a expiré
func (ik *IdempotencyKey) IsExpired() bool {
	return time.Now().After(ik.ExpiresAt)
}

// IsSameRequest vérifie si le hash de la requête correspond
// (pour détecter si le client a modifié le body entre deux tentatives)
func (ik *IdempotencyKey) IsSameRequest(requestBody []byte) bool {
	hash := HashRequestBody(requestBody)
	return ik.RequestHash == hash
}

// HashRequestBody génère un hash SHA-256 du body de la requête
func HashRequestBody(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// ResponseHeadersToJSON convertit les headers en JSON pour stockage
func ResponseHeadersToJSON(headers map[string]string) ([]byte, error) {
	return json.Marshal(headers)
}

// ResponseBodyToJSON convertit le body en JSON pour stockage
func ResponseBodyToJSON(body map[string]interface{}) ([]byte, error) {
	return json.Marshal(body)
}

// NewIdempotencyKey crée une nouvelle clé d'idempotence
func NewIdempotencyKey(key, userID, endpoint string, requestBody []byte, ttl time.Duration) *IdempotencyKey {
	return &IdempotencyKey{
		IdempotencyKey:  key,
		UserID:          userID,
		Endpoint:        endpoint,
		RequestHash:     HashRequestBody(requestBody),
		ResponseStatus:  0, // Sera rempli après exécution
		ResponseHeaders: make(map[string]string),
		ResponseBody:    make(map[string]interface{}),
		CreatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(ttl),
	}
}
