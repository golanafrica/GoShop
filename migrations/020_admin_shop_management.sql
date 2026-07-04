-- ============================================================
-- Migration 020 : Admin Shop Management + Health Score
-- Date: 2026-07-04
-- Version: v4.2.0
-- Description: Gestion cross-tenant des shops + Account Health Score
-- Inspiration: Amazon Seller Central Account Health Dashboard
-- ============================================================
--
-- 🎯 Objectifs :
--   1. Permettre aux admins de suspendre/activer des shops
--   2. Calculer un Account Health Score (0-1000)
--   3. Avoir un audit trail complet des actions admin
--   4. Stocker des notes internes admin
--
-- 📊 Health Score Levels :
--   - excellent : 800-1000 (🟢 Vert)
--   - good      : 600-799  (🔵 Bleu)
--   - warning   : 400-599  (🟡 Jaune)
--   - critical  : 0-399    (🔴 Rouge)
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : AJOUT COLONNES ADMIN SUR SHOPS
-- ============================================================

-- 1.1 Suspension (audit trail)
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ;

ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS suspended_by VARCHAR(36);

ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS suspension_reason TEXT;

-- 1.2 Account Health Score (inspiré d'Amazon)
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS health_score INT NOT NULL DEFAULT 1000
        CHECK (health_score >= 0 AND health_score <= 1000);

ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS health_level VARCHAR(20) NOT NULL DEFAULT 'excellent'
        CHECK (health_level IN ('excellent', 'good', 'warning', 'critical'));

ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS health_updated_at TIMESTAMPTZ;

-- 1.3 Notes internes admin
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS admin_notes TEXT;

-- 1.4 Revue admin
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS last_reviewed_at TIMESTAMPTZ;

ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS last_reviewed_by VARCHAR(36);

-- ============================================================
-- PARTIE 2 : INDEX POUR PERFORMANCE
-- ============================================================

-- Index pour recherche admin
CREATE INDEX IF NOT EXISTS idx_shops_health_score ON shops(health_score DESC);
CREATE INDEX IF NOT EXISTS idx_shops_health_level ON shops(health_level);
CREATE INDEX IF NOT EXISTS idx_shops_suspended ON shops(suspended_at DESC) 
    WHERE suspended_at IS NOT NULL;

-- Index composite pour dashboard admin
CREATE INDEX IF NOT EXISTS idx_shops_admin_dashboard 
    ON shops(health_level, is_active, created_at DESC);

-- ============================================================
-- PARTIE 3 : TABLE SHOP_ADMIN_ACTIONS (audit trail)
-- ============================================================

CREATE TABLE IF NOT EXISTS shop_admin_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Qui a fait l'action
    admin_id VARCHAR(36) NOT NULL,
    admin_email VARCHAR(255),
    admin_role VARCHAR(50),
    
    -- Type d'action
    action_type VARCHAR(50) NOT NULL
        CHECK (action_type IN (
            'suspend',           -- Suspendre shop
            'activate',          -- Réactiver shop
            'change_plan',       -- Changer plan
            'update_health',     -- Mise à jour health score
            'add_note',          -- Ajouter note
            'review',            -- Revue admin
            'export_data'        -- Export données
        )),
    
    -- Détails
    old_value JSONB,
    new_value JSONB,
    reason TEXT,
    
    -- Contexte
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index pour performance
CREATE INDEX IF NOT EXISTS idx_admin_actions_shop ON shop_admin_actions(shop_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_admin ON shop_admin_actions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_type ON shop_admin_actions(action_type);
CREATE INDEX IF NOT EXISTS idx_admin_actions_created ON shop_admin_actions(created_at DESC);

-- ============================================================
-- PARTIE 4 : TRIGGER AUTO-UPDATE health_level
-- ============================================================

CREATE OR REPLACE FUNCTION update_shop_health_level()
RETURNS TRIGGER AS $$
BEGIN
    -- Calculer le niveau selon le score
    IF NEW.health_score >= 800 THEN
        NEW.health_level := 'excellent';
    ELSIF NEW.health_score >= 600 THEN
        NEW.health_level := 'good';
    ELSIF NEW.health_score >= 400 THEN
        NEW.health_level := 'warning';
    ELSE
        NEW.health_level := 'critical';
    END IF;
    
    NEW.health_updated_at := NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_shop_health_level ON shops;
CREATE TRIGGER trigger_update_shop_health_level
    BEFORE UPDATE OF health_score ON shops
    FOR EACH ROW
    EXECUTE FUNCTION update_shop_health_level();

-- ============================================================
-- PARTIE 5 : VUES POUR DASHBOARD ADMIN
-- ============================================================

-- 5.1 Vue : Shops par niveau de santé
CREATE OR REPLACE VIEW v_shops_health_distribution AS
SELECT 
    health_level,
    COUNT(*) as count,
    ROUND(AVG(health_score)::numeric, 2) as avg_score,
    MIN(health_score) as min_score,
    MAX(health_score) as max_score,
    ROUND(COUNT(*)::numeric / NULLIF((SELECT COUNT(*) FROM shops), 0) * 100, 2) as percentage
FROM shops
GROUP BY health_level
ORDER BY 
    CASE health_level
        WHEN 'excellent' THEN 1
        WHEN 'good' THEN 2
        WHEN 'warning' THEN 3
        WHEN 'critical' THEN 4
    END;

-- 5.2 Vue : Shops critiques (action requise)
CREATE OR REPLACE VIEW v_shops_critical AS
SELECT 
    s.id,
    s.name,
    s.slug,
    s.owner_id,
    u.email as owner_email,
    s.health_score,
    s.health_level,
    s.is_active,
    s.kyc_status,
    s.suspended_at,
    s.suspension_reason,
    s.created_at,
    EXTRACT(DAY FROM NOW() - s.health_updated_at) as days_since_health_update
FROM shops s
LEFT JOIN users u ON u.id = s.owner_id
WHERE s.health_level = 'critical'
ORDER BY s.health_score ASC, s.created_at ASC;

-- 5.3 Vue : Shops suspendus
CREATE OR REPLACE VIEW v_shops_suspended AS
SELECT 
    s.id,
    s.name,
    s.slug,
    s.owner_id,
    u.email as owner_email,
    s.suspended_at,
    s.suspended_by,
    a.email as suspended_by_email,
    s.suspension_reason,
    EXTRACT(DAY FROM NOW() - s.suspended_at) as days_suspended
FROM shops s
LEFT JOIN users u ON u.id = s.owner_id
LEFT JOIN users a ON a.id = s.suspended_by
WHERE s.suspended_at IS NOT NULL
ORDER BY s.suspended_at DESC;

-- 5.4 Vue : Actions admin récentes
CREATE OR REPLACE VIEW v_shop_admin_actions_recent AS
SELECT 
    a.id,
    a.created_at,
    a.shop_id,
    s.name as shop_name,
    a.admin_id,
    u.email as admin_email,
    a.admin_role,
    a.action_type,
    a.reason,
    a.old_value,
    a.new_value
FROM shop_admin_actions a
JOIN shops s ON s.id = a.shop_id
LEFT JOIN users u ON u.id = a.admin_id
ORDER BY a.created_at DESC
LIMIT 100;

-- 5.5 Vue : Statistiques admin
CREATE OR REPLACE VIEW v_shop_admin_stats AS
SELECT 
    (SELECT COUNT(*) FROM shops) as total_shops,
    (SELECT COUNT(*) FROM shops WHERE is_active = true) as active_shops,
    (SELECT COUNT(*) FROM shops WHERE is_active = false) as inactive_shops,
    (SELECT COUNT(*) FROM shops WHERE suspended_at IS NOT NULL) as suspended_shops,
    (SELECT COUNT(*) FROM shops WHERE health_level = 'critical') as critical_shops,
    (SELECT COUNT(*) FROM shops WHERE health_level = 'warning') as warning_shops,
    (SELECT COUNT(*) FROM shops WHERE kyc_status = 'pending') as pending_kyc,
    (SELECT COUNT(*) FROM shop_admin_actions WHERE created_at > NOW() - INTERVAL '24 hours') as actions_last_24h,
    (SELECT COUNT(*) FROM shop_admin_actions WHERE created_at > NOW() - INTERVAL '7 days') as actions_last_7d;

-- ============================================================
-- PARTIE 6 : FONCTIONS UTILITAIRES
-- ============================================================

-- 6.1 Fonction pour logger une action admin
CREATE OR REPLACE FUNCTION log_shop_admin_action(
    p_shop_id UUID,
    p_admin_id VARCHAR(36),
    p_admin_email VARCHAR(255),
    p_admin_role VARCHAR(50),
    p_action_type VARCHAR(50),
    p_old_value JSONB,
    p_new_value JSONB,
    p_reason TEXT,
    p_ip_address INET,
    p_user_agent TEXT,
    p_request_id VARCHAR(100)
) RETURNS UUID AS $$
DECLARE
    action_id UUID;
BEGIN
    INSERT INTO shop_admin_actions (
        shop_id, admin_id, admin_email, admin_role,
        action_type, old_value, new_value, reason,
        ip_address, user_agent, request_id
    ) VALUES (
        p_shop_id, p_admin_id, p_admin_email, p_admin_role,
        p_action_type, p_old_value, p_new_value, p_reason,
        p_ip_address, p_user_agent, p_request_id
    ) RETURNING id INTO action_id;
    
    RETURN action_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6.2 Fonction pour calculer le health score (simplifiée)
-- La version complète sera implémentée en Go
CREATE OR REPLACE FUNCTION calculate_shop_health_score(shop_id_param UUID)
RETURNS INT AS $$
DECLARE
    score INT := 1000;
    shop_record RECORD;
BEGIN
    SELECT * INTO shop_record FROM shops WHERE id = shop_id_param;
    
    IF shop_record IS NULL THEN
        RETURN 0;
    END IF;
    
    -- KYC non vérifié : -200
    IF shop_record.kyc_status != 'verified' THEN
        score := score - 200;
    END IF;
    
    -- Shop inactif : -300
    IF NOT shop_record.is_active THEN
        score := score - 300;
    END IF;
    
    -- Shop suspendu : -500
    IF shop_record.suspended_at IS NOT NULL THEN
        score := score - 500;
    END IF;
    
    -- Ancienneté > 6 mois : +100
    IF shop_record.created_at < NOW() - INTERVAL '6 months' THEN
        score := score + 100;
    END IF;
    
    -- Limiter entre 0 et 1000
    RETURN GREATEST(0, LEAST(1000, score));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- PARTIE 7 : MISE À JOUR DES DONNÉES EXISTANTES
-- ============================================================

-- 7.1 Initialiser health_score à 1000 pour tous les shops existants
UPDATE shops 
SET health_score = 1000,
    health_level = 'excellent',
    health_updated_at = NOW()
WHERE health_score IS NULL OR health_score = 0;

-- 7.2 Recalculer le score basé sur les critères actuels
UPDATE shops 
SET health_score = calculate_shop_health_score(id)
WHERE id IN (SELECT id FROM shops);

-- ============================================================
-- PARTIE 8 : DOCUMENTATION
-- ============================================================

COMMENT ON COLUMN shops.health_score IS 
    'Score de santé de la boutique (0-1000). Inspiré d''Amazon Account Health Dashboard.
     Calculé automatiquement basé sur : KYC, paiements, litiges, ancienneté.';

COMMENT ON COLUMN shops.health_level IS 
    'Niveau de santé : excellent (800-1000), good (600-799), warning (400-599), critical (0-399)';

COMMENT ON COLUMN shops.suspended_at IS 
    'Date de suspension par un admin. NULL si non suspendu.';

COMMENT ON COLUMN shops.suspension_reason IS 
    'Raison de la suspension (obligatoire si suspendu)';

COMMENT ON COLUMN shops.admin_notes IS 
    'Notes internes admin (visibles uniquement par les admins)';

COMMENT ON TABLE shop_admin_actions IS 
    'Audit trail de toutes les actions admin sur les shops';

-- ============================================================
-- PARTIE 9 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE
    total_shops INT;
    excellent_count INT;
    good_count INT;
    warning_count INT;
    critical_count INT;
BEGIN
    SELECT COUNT(*) INTO total_shops FROM shops;
    SELECT COUNT(*) INTO excellent_count FROM shops WHERE health_level = 'excellent';
    SELECT COUNT(*) INTO good_count FROM shops WHERE health_level = 'good';
    SELECT COUNT(*) INTO warning_count FROM shops WHERE health_level = 'warning';
    SELECT COUNT(*) INTO critical_count FROM shops WHERE health_level = 'critical';
    
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '✅ Migration 020 terminée avec succès';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '📊 Distribution Health Score :';
    RAISE NOTICE '   - Total shops : %', total_shops;
    RAISE NOTICE '   - Excellent   : % (🟢)', excellent_count;
    RAISE NOTICE '   - Good        : % (🔵)', good_count;
    RAISE NOTICE '   - Warning     : % (🟡)', warning_count;
    RAISE NOTICE '   - Critical    : % (🔴)', critical_count;
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '🎯 Prochaines étapes :';
    RAISE NOTICE '   1. Mettre à jour domain/entity/shop.go';
    RAISE NOTICE '   2. Mettre à jour shop_repository.go';
    RAISE NOTICE '   3. Créer AdminShopUsecase (5 usecases)';
    RAISE NOTICE '   4. Créer AdminShopHandler (endpoints HTTP)';
    RAISE NOTICE '   5. Ajouter routes dans app.go';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
END $$;