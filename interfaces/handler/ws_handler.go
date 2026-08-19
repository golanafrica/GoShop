package handler

import (
	"net/http"
	"time"

	"Goshop/application/metrics"
	wsinfra "Goshop/infrastructure/websocket"
	"Goshop/interfaces/utils"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// En production, restreindre aux domaines autorisés
		return true
	},
}

// WSHandler gère les connexions WebSocket
type WSHandler struct {
	hub *wsinfra.Hub
}

// NewWSHandler crée une nouvelle instance
func NewWSHandler(hub *wsinfra.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// @Summary Connexion WebSocket pour les notifications temps réel
// @Description Établit une connexion WebSocket persistante pour recevoir des notifications en temps réel (commandes, paiements, etc.). Nécessite une authentification JWT valide.
// @Tags WebSocket
// @Success 101 "Switching Protocols (Connexion WebSocket établie avec succès)"
// @Failure 401 {object} utils.AppError "Non autorisé (JWT manquant ou invalide)"
// @Failure 500 {object} utils.AppError "Échec de la mise à niveau vers WebSocket"
// @Security ApiKeyAuth
// @Router /ws/notifications [get]
func (h *WSHandler) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())
	connStart := time.Now()

	// Récupérer l'ID utilisateur depuis le contexte
	userID, ok := utils.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		// 📊 MÉTRIQUE : Échec authentification
		metrics.WebSocketConnectionsTotal.WithLabelValues("auth_error").Inc()

		logger.Warn().Msg("Unauthorized WebSocket connection attempt: no user ID")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade la connexion HTTP vers WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 📊 MÉTRIQUE : Erreur d'upgrade
		metrics.WebSocketUpgradeErrors.Inc()
		metrics.WebSocketConnectionsTotal.WithLabelValues("upgrade_error").Inc()

		logger.Error().Err(err).
			Str("user_id", userID).
			Msg("Failed to upgrade to WebSocket")
		return
	}

	// 📊 MÉTRIQUES : Succès connexion
	metrics.WebSocketConnectionsTotal.WithLabelValues("success").Inc()
	metrics.WebSocketConnectionsActive.Inc()

	// Enregistrer le client dans le hub
	h.hub.Register(userID, conn)
	logger.Info().
		Str("user_id", userID).
		Msg("✅ WebSocket connection established")

	// Garder la connexion ouverte et gérer la déconnexion propre
	defer func() {
		h.hub.Unregister(userID)
		conn.Close()

		// 📊 MÉTRIQUES : Déconnexion
		duration := time.Since(connStart).Seconds()
		metrics.WebSocketConnectionsActive.Dec()
		metrics.WebSocketConnectionDuration.Observe(duration)

		logger.Info().
			Str("user_id", userID).
			Float64("duration_seconds", duration).
			Msg("WebSocket connection closed")
	}()

	// Lire les messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 📊 MÉTRIQUE : Fermeture inattendue
				metrics.WebSocketUnexpectedCloses.Inc()

				logger.Warn().Err(err).
					Str("user_id", userID).
					Msg("⚠️ WebSocket unexpected close")
			}
			break
		}

		// 📊 MÉTRIQUE : Message reçu
		metrics.WebSocketMessagesReceived.Inc()
	}
}
