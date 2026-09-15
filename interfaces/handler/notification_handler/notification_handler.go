package notificationhandler

import (
	"net/http"
	"strconv"

	"Goshop/domain/repository"
	"Goshop/interfaces/middl" // 🆕 AJOUTÉ : Pour le middleware ErrorHandler
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
)

// NotificationHandler gère les endpoints REST du centre de notifications in-app
type NotificationHandler struct {
	notifRepo repository.NotificationRepository
}

// NewNotificationHandler crée une nouvelle instance du handler
func NewNotificationHandler(notifRepo repository.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{
		notifRepo: notifRepo,
	}
}

// GetNotifications retourne la liste paginée des notifications de l'utilisateur connecté
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// 1. Récupérer l'ID de l'utilisateur authentifié
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	// 2. Gérer la pagination (défaut: 20 par page, max 100)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// 3. Récupérer les notifications et le compteur de non-lues
	notifications, err := h.notifRepo.FindByUserID(ctx, authUserID, limit, offset)
	if err != nil {
		return utils.NewAppError("NOTIFICATIONS_FETCH_FAILED", err.Error(), http.StatusInternalServerError)
	}

	unreadCount, err := h.notifRepo.CountUnread(ctx, authUserID)
	if err != nil {
		// On log l'erreur mais on ne bloque pas la réponse si on a quand même les notifications
		unreadCount = 0
	}

	// 4. Retourner la réponse structurée pour le frontend
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"data":         notifications,
		"unread_count": unreadCount,
		"limit":        limit,
		"offset":       offset,
	})
	return nil
}

// MarkAsRead marque une notification spécifique comme lue
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	notificationID := chi.URLParam(r, "id")
	if notificationID == "" {
		return utils.NewAppError("INVALID_PAYLOAD", "notification ID is required", http.StatusBadRequest)
	}

	if err := h.notifRepo.MarkAsRead(ctx, notificationID, authUserID); err != nil {
		return utils.NewAppError("MARK_READ_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notification marquée comme lue",
	})
	return nil
}

// MarkAllAsRead marque toutes les notifications de l'utilisateur comme lues
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	if err := h.notifRepo.MarkAllAsRead(ctx, authUserID); err != nil {
		return utils.NewAppError("MARK_ALL_READ_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Toutes les notifications ont été marquées comme lues",
	})
	return nil
}

// RegisterRoutes enregistre les routes du centre de notifications
func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	// 🆕 CORRECTION : Enveloppement avec middl.ErrorHandler comme dans le reste du projet
	r.Get("/", middl.ErrorHandler(h.GetNotifications))
	r.Post("/{id}/read", middl.ErrorHandler(h.MarkAsRead))
	r.Post("/read-all", middl.ErrorHandler(h.MarkAllAsRead))
}
