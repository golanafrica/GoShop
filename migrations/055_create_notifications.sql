-- ============================================
-- Migration 055 : In-App Notification Center
-- ============================================
-- Description : Stockage permanent des notifications in-app
--               pour l'historique, le statut "non lu" et 
--               la récupération après une déconnexion.
-- Architecture : PostgreSQL (persistance) + Redis (temps réel via WebSocket)

CREATE TABLE IF NOT EXISTS notifications (
    -- id est VARCHAR(36) car généré en Go (comme les autres IDs)
    -- user_id est VARCHAR(36) car users.id est VARCHAR(36)
    -- shop_id est UUID car shops.id est UUID
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    
    -- Type de notification (ex: 'order_confirmed', 'tontine_cycle_paid')
    notification_type VARCHAR(50) NOT NULL,
    
    -- Titre et message affichés à l'utilisateur
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    
    -- Données supplémentaires au format JSON (ex: order_id, amount)
    data JSONB DEFAULT '{}',
    
    -- Statut de lecture
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMPTZ,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ -- Pour le nettoyage automatique des vieilles notifications
);

-- Index pour récupérer rapidement les notifications NON LUES d'un utilisateur
-- (Très important pour afficher le badge "🔴 3" sur l'icône de la cloche)
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread 
    ON notifications(user_id, is_read) 
    WHERE is_read = false;

-- Index pour le tri chronologique (le plus récent en premier)
CREATE INDEX IF NOT EXISTS idx_notifications_user_created 
    ON notifications(user_id, created_at DESC);

-- Index pour le nettoyage automatique des notifications expirées (cron job)
CREATE INDEX IF NOT EXISTS idx_notifications_expires 
    ON notifications(expires_at) 
    WHERE expires_at IS NOT NULL;

-- Commentaires pour la documentation de la base de données
COMMENT ON TABLE notifications IS 'Centre de notifications in-app GoShop (historique et statut lu/non lu)';
COMMENT ON COLUMN notifications.data IS 'Données JSON supplémentaires (ex: {"order_id": "123", "amount": 5000})';

DO $$
BEGIN
    RAISE NOTICE '✅ Migration 055 terminée : table "notifications" créée avec succès';
END $$;