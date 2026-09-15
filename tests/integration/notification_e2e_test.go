package integration

import (
	"context"
	"testing"

	"Goshop/domain/entity"
	notificationinfra "Goshop/infrastructure/notification"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepository_Integration(t *testing.T) {
	// 1. Setup
	ctx := context.Background()

	// Créer un utilisateur de test pour associer les notifications
	testUserID := uuid.New().String()

	// Nettoyer au cas où des tests précédents auraient laissé des traces
	sharedDB.Exec("DELETE FROM notifications WHERE user_id = $1", testUserID)
	sharedDB.Exec("DELETE FROM users WHERE id = $1", testUserID)

	_, err := sharedDB.Exec(`
		INSERT INTO users (id, email, password, role, is_active, status)
		VALUES ($1, $2, $3, $4, true, $5)
	`, testUserID, "test_notif@example.com", "hashed_password", "user", "active")
	require.NoError(t, err, "Failed to create test user")

	// Initialiser le repository
	notifRepo := notificationinfra.NewNotificationRepositoryInfrastructure(sharedDB)

	// Cleanup après le test
	t.Cleanup(func() {
		sharedDB.Exec("DELETE FROM notifications WHERE user_id = $1", testUserID)
		sharedDB.Exec("DELETE FROM users WHERE id = $1", testUserID)
	})

	// 2. Test : Création d'une notification
	notif := entity.NewNotification(
		uuid.New().String(),
		testUserID,
		"order_confirmed",
		"Commande confirmée ✅",
		"Votre commande a été acceptée par le marchand.",
		map[string]interface{}{"order_id": "ORD-123"},
	)

	err = notifRepo.Create(ctx, notif)
	require.NoError(t, err, "Failed to create notification")

	// 3. Test : Récupération par utilisateur (FindByUserID)
	notifications, err := notifRepo.FindByUserID(ctx, testUserID, 10, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.Equal(t, "Commande confirmée ✅", notifications[0].Title)
	assert.Equal(t, "Votre commande a été acceptée par le marchand.", notifications[0].Message)
	assert.False(t, notifications[0].IsRead)

	// 4. Test : Comptage des non-lues (CountUnread)
	unreadCount, err := notifRepo.CountUnread(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, 1, unreadCount, "Il devrait y avoir 1 notification non lue")

	// 5. Test : Marquer comme lu (MarkAsRead)
	err = notifRepo.MarkAsRead(ctx, notif.ID, testUserID)
	require.NoError(t, err)

	// Vérifier qu'elle est bien marquée comme lue
	updatedNotifs, err := notifRepo.FindByUserID(ctx, testUserID, 10, 0)
	require.NoError(t, err)
	require.Len(t, updatedNotifs, 1)
	assert.True(t, updatedNotifs[0].IsRead)
	assert.NotNil(t, updatedNotifs[0].ReadAt)

	// Vérifier que le compteur de non-lues est maintenant à 0
	unreadCount, err = notifRepo.CountUnread(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, 0, unreadCount, "Le compteur de non-lues devrait être à 0")

	// 6. Test : Tout marquer comme lu (MarkAllAsRead)
	// Créer une deuxième notification non lue
	notif2 := entity.NewNotification(
		uuid.New().String(),
		testUserID,
		"tontine_cycle_paid",
		"Cotisation reçue",
		"Un membre a payé sa tranche.",
		map[string]interface{}{},
	)
	err = notifRepo.Create(ctx, notif2)
	require.NoError(t, err)

	// Marquer tout comme lu
	err = notifRepo.MarkAllAsRead(ctx, testUserID)
	require.NoError(t, err)

	// Vérifier que tout est bien lu
	unreadCount, err = notifRepo.CountUnread(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, 0, unreadCount, "Toutes les notifications devraient être marquées comme lues")

	t.Log("✅ Tous les tests d'intégration du repository de notifications sont validés !")
}
