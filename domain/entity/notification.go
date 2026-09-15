package entity

import (
	"encoding/json"
	"time"
)

// Notification représente une notification in-app persistée en base de données
// Architecture : PostgreSQL (persistance) + Redis/WebSocket (temps réel)
type Notification struct {
	// ID unique de la notification (généré en Go, stocké en VARCHAR(36))
	ID string `json:"id" db:"id"`

	// UserID de l'utilisateur destinataire (VARCHAR(36) car users.id est VARCHAR)
	UserID string `json:"user_id" db:"user_id"`

	// ShopID optionnel (UUID car shops.id est UUID, nullable)
	ShopID *string `json:"shop_id,omitempty" db:"shop_id"`

	// Type de notification (ex: "order_confirmed", "tontine_cycle_paid")
	NotificationType string `json:"notification_type" db:"notification_type"`

	// Titre court affiché en gras dans l'interface
	Title string `json:"title" db:"title"`

	// Message détaillé de la notification
	Message string `json:"message" db:"message"`

	// Données supplémentaires au format JSON (ex: {"order_id": "123", "amount": 5000})
	Data json.RawMessage `json:"data" db:"data"`

	// Statut de lecture
	IsRead bool `json:"is_read" db:"is_read"`

	// Timestamp de lecture (null si non lue)
	ReadAt *time.Time `json:"read_at,omitempty" db:"read_at"`

	// Timestamp de création
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Timestamp d'expiration optionnel (pour nettoyage automatique)
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// NewNotification crée une nouvelle notification avec les valeurs par défaut
func NewNotification(id, userID, notifType, title, message string, data map[string]interface{}) *Notification {
	// Convertir les données en JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		dataJSON = []byte("{}")
	}

	now := time.Now().UTC()

	return &Notification{
		ID:               id,
		UserID:           userID,
		NotificationType: notifType,
		Title:            title,
		Message:          message,
		Data:             dataJSON,
		IsRead:           false,
		CreatedAt:        now,
	}
}

// MarkAsRead marque la notification comme lue
func (n *Notification) MarkAsRead() {
	now := time.Now().UTC()
	n.IsRead = true
	n.ReadAt = &now
}

// IsExpired vérifie si la notification a expiré
func (n *Notification) IsExpired() bool {
	if n.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*n.ExpiresAt)
}

// SetExpiration définit une date d'expiration
func (n *Notification) SetExpiration(duration time.Duration) {
	expiration := time.Now().UTC().Add(duration)
	n.ExpiresAt = &expiration
}
