package repository

import (
	"context"

	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../mocks/repository/mock_notification_repository.go -package=repository . NotificationRepository

// NotificationRepository définit le contrat pour la persistance des notifications in-app
// Architecture : PostgreSQL (persistance) + Redis/WebSocket (temps réel)
type NotificationRepository interface {
	// Create sauvegarde une nouvelle notification en base de données
	// Retourne une erreur si la sauvegarde échoue
	Create(ctx context.Context, notification *entity.Notification) error

	// FindByUserID récupère les notifications d'un utilisateur avec pagination
	// - limit : nombre maximum de notifications à retourner
	// - offset : nombre de notifications à sauter (pour la pagination)
	// Les notifications sont triées par date de création (plus récent en premier)
	FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error)

	// CountUnread retourne le nombre de notifications non lues pour un utilisateur
	// Utilisé pour afficher le badge "🔴 3" sur l'icône de la cloche
	CountUnread(ctx context.Context, userID string) (int, error)

	// MarkAsRead marque une notification spécifique comme lue
	// Vérifie que la notification appartient bien à l'utilisateur (sécurité)
	MarkAsRead(ctx context.Context, notificationID, userID string) error

	// MarkAllAsRead marque toutes les notifications d'un utilisateur comme lues
	// Utile pour le bouton "Tout marquer comme lu" dans l'interface
	MarkAllAsRead(ctx context.Context, userID string) error

	// DeleteExpired supprime les notifications qui ont expiré
	// Retourne le nombre de notifications supprimées
	// Utilisé par le cron job de nettoyage automatique
	DeleteExpired(ctx context.Context) (int, error)

	// FindByID récupère une notification par son ID
	// Utile pour vérifier qu'une notification existe avant de la marquer comme lue
	FindByID(ctx context.Context, id string) (*entity.Notification, error)
}
