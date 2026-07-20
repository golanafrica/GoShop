package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// NotificationMessage représente le payload envoyé au client
type NotificationMessage struct {
	Type    string      `json:"type"` // ex: "kyc_approved", "order_delivered"
	Title   string      `json:"title"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Hub gère les connexions WebSocket et la distribution des messages
type Hub struct {
	clients     map[string]*websocket.Conn // userID -> connexion
	mu          sync.RWMutex
	redisClient *redis.Client
	logger      zerolog.Logger
	pubsub      *redis.PubSub
}

// NewHub crée une nouvelle instance du Hub
func NewHub(redisClient *redis.Client, logger zerolog.Logger) *Hub {
	h := &Hub{
		clients:     make(map[string]*websocket.Conn),
		redisClient: redisClient,
		logger:      logger.With().Str("component", "websocket_hub").Logger(),
	}

	// S'abonner au canal de notification global
	h.pubsub = redisClient.Subscribe(context.Background(), "goshop:notifications")

	// Démarrer l'écoute des messages Redis en arrière-plan
	go h.listenRedis()

	return h
}

// Register ajoute un client au hub
func (h *Hub) Register(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Fermer l'ancienne connexion si elle existe
	if oldConn, exists := h.clients[userID]; exists {
		oldConn.Close()
	}

	h.clients[userID] = conn
	h.logger.Info().Str("user_id", userID).Msg("Client WebSocket registered")
}

// Unregister retire un client du hub
func (h *Hub) Unregister(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conn, exists := h.clients[userID]; exists {
		conn.Close()
		delete(h.clients, userID)
		h.logger.Info().Str("user_id", userID).Msg("Client WebSocket unregistered")
	}
}

// SendToUser envoie une notification à un utilisateur spécifique via Redis
func (h *Hub) SendToUser(ctx context.Context, userID string, msg NotificationMessage) error {
	payload, err := json.Marshal(map[string]interface{}{
		"target_user": userID,
		"message":     msg,
	})
	if err != nil {
		return err
	}

	// Publier sur Redis (tous les nœuds de l'application recevront ce message)
	return h.redisClient.Publish(ctx, "goshop:notifications", payload).Err()
}

// listen écoute les messages de Redis et les route vers les bons clients
func (h *Hub) listenRedis() {
	ch := h.pubsub.Channel()

	for msg := range ch {
		var payload struct {
			TargetUser string              `json:"target_user"`
			Message    NotificationMessage `json:"message"`
		}

		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			h.logger.Error().Err(err).Msg("Failed to unmarshal Redis notification")
			continue
		}

		h.mu.RLock()
		conn, exists := h.clients[payload.TargetUser]
		h.mu.RUnlock()

		if !exists {
			continue // L'utilisateur n'est pas connecté, on ignore
		}

		// Écrire sur la connexion WebSocket
		if err := conn.WriteJSON(payload.Message); err != nil {
			h.logger.Warn().Err(err).Str("user_id", payload.TargetUser).Msg("Failed to write to WebSocket, closing connection")
			h.Unregister(payload.TargetUser)
		}
	}
}

// Close ferme proprement le hub
func (h *Hub) Close() {
	h.pubsub.Close()
	h.mu.Lock()
	for _, conn := range h.clients {
		conn.Close()
	}
	h.mu.Unlock()
}
