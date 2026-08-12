-- ============================================================
-- Migration 020 : Admin Shop Management + Health Score
-- Date: 2026-07-04
-- Version: v4.2.0
-- Description: Gestion cross-tenant des shops + Account Health Score
-- ============================================================

-- ============================================================
-- PARTIE 1 : AJOUT COLONNES ADMIN SUR SHOPS (Sans contraintes strictes immédiates)
-- ============================================================
ALTER TABLE shops ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS suspended_by VARCHAR(36);
ALTER TABLE shops ADD COLUMN IF NOT EXISTS suspension_reason TEXT;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS health_score INT DEFAULT 1000;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS health_level VARCHAR(20) DEFAULT 'excellent';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS health_updated_at TIMESTAMPTZ;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS admin_notes TEXT;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS last_reviewed_at TIMESTAMPTZ;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS last_reviewed_by VARCHAR(36);

-- 🚨 CRUCIAL : Nettoyer les données AVANT d'appliquer NOT NULL et CHECK
UPDATE shops SET health_score = 1000 WHERE health_score IS NULL;
UPDATE shops SET health_level = 'excellent' WHERE health_level IS NULL OR health_level NOT IN ('excellent', 'good', 'warning', 'critical');

-- Application des contraintes NOT NULL et CHECK (maintenant que les données sont propres)
ALTER TABLE shops ALTER COLUMN health_score SET NOT NULL;
ALTER TABLE shops ALTER COLUMN health_level SET NOT NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'shops_health_score_check' AND conrelid = 'shops'::regclass) THEN
        ALTER TABLE shops DROP CONSTRAINT shops_health_score_check;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'shops_health_level_check' AND conrelid = 'shops'::regclass) THEN
        ALTER TABLE shops DROP CONSTRAINT shops_health_level_check;
    END IF;
    ALTER TABLE shops ADD CONSTRAINT shops_health_score_check CHECK (health_score >= 0 AND health_score <= 1000);
    ALTER TABLE shops ADD CONSTRAINT shops_health_level_check CHECK (health_level IN ('excellent', 'good', 'warning', 'critical'));
END $$;

-- ============================================================
-- PARTIE 2 : INDEX POUR PERFORMANCE
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_shops_health_score ON shops(health_score DESC);
CREATE INDEX IF NOT EXISTS idx_shops_health_level ON shops(health_level);
CREATE INDEX IF NOT EXISTS idx_shops_suspended ON shops(suspended_at DESC) WHERE suspended_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shops_admin_dashboard ON shops(health_level, is_active, created_at DESC);

-- ============================================================
-- PARTIE 3 : TABLE SHOP_ADMIN_ACTIONS (audit trail)
-- ============================================================
CREATE TABLE IF NOT EXISTS shop_admin_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    admin_id VARCHAR(36) NOT NULL,
    admin_email VARCHAR(255),
    admin_role VARCHAR(50),
    action_type VARCHAR(50) NOT NULL CHECK (action_type IN ('suspend', 'activate', 'change_plan', 'update_health', 'add_note', 'review', 'export_data')),
    old_value JSONB,
    new_value JSONB,
    reason TEXT,
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_actions_shop ON shop_admin_actions(shop_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_admin ON shop_admin_actions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_type ON shop_admin_actions(action_type);
CREATE INDEX IF NOT EXISTS idx_admin_actions_created ON shop_admin_actions(created_at DESC);

-- ============================================================
-- PARTIE 4 : TRIGGER AUTO-UPDATE health_level
-- ============================================================
CREATE OR REPLACE FUNCTION update_shop_health_level() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.health_score >= 800 THEN NEW.health_level := 'excellent';
    ELSIF NEW.health_score >= 600 THEN NEW.health_level := 'good';
    ELSIF NEW.health_score >= 400 THEN NEW.health_level := 'warning';
    ELSE NEW.health_level := 'critical';
    END IF;
    NEW.health_updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_shop_health_level ON shops;
CREATE TRIGGER trigger_update_shop_health_level BEFORE UPDATE OF health_score ON shops FOR EACH ROW EXECUTE FUNCTION update_shop_health_level();

-- ============================================================
-- PARTIE 5 : VUES POUR DASHBOARD ADMIN
-- ============================================================
CREATE OR REPLACE VIEW v_shops_health_distribution AS
SELECT health_level, COUNT(*) as count, ROUND(AVG(health_score)::numeric, 2) as avg_score,
       MIN(health_score) as min_score, MAX(health_score) as max_score,
       ROUND(COUNT(*)::numeric / NULLIF((SELECT COUNT(*) FROM shops), 0) * 100, 2) as percentage
FROM shops GROUP BY health_level
ORDER BY CASE health_level WHEN 'excellent' THEN 1 WHEN 'good' THEN 2 WHEN 'warning' THEN 3 WHEN 'critical' THEN 4 END;

CREATE OR REPLACE VIEW v_shops_critical AS
SELECT s.id, s.name, s.slug, s.owner_id, u.email as owner_email, s.health_score, s.health_level, s.is_active,
       s.kyc_status, s.suspended_at, s.suspension_reason, s.created_at,
       EXTRACT(DAY FROM NOW() - s.health_updated_at) as days_since_health_update
FROM shops s LEFT JOIN users u ON u.id = s.owner_id WHERE s.health_level = 'critical'
ORDER BY s.health_score ASC, s.created_at ASC;

CREATE OR REPLACE VIEW v_shops_suspended AS
SELECT s.id, s.name, s.slug, s.owner_id, u.email as owner_email, s.suspended_at, s.suspended_by,
       a.email as suspended_by_email, s.suspension_reason, EXTRACT(DAY FROM NOW() - s.suspended_at) as days_suspended
FROM shops s LEFT JOIN users u ON u.id = s.owner_id LEFT JOIN users a ON a.id = s.suspended_by
WHERE s.suspended_at IS NOT NULL ORDER BY s.suspended_at DESC;

CREATE OR REPLACE VIEW v_shop_admin_actions_recent AS
SELECT a.id, a.created_at, a.shop_id, s.name as shop_name, a.admin_id, u.email as admin_email,
       a.admin_role, a.action_type, a.reason, a.old_value, a.new_value
FROM shop_admin_actions a JOIN shops s ON s.id = a.shop_id LEFT JOIN users u ON u.id = a.admin_id
ORDER BY a.created_at DESC LIMIT 100;

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
CREATE OR REPLACE FUNCTION log_shop_admin_action(p_shop_id UUID, p_admin_id VARCHAR(36), p_admin_email VARCHAR(255), p_admin_role VARCHAR(50), p_action_type VARCHAR(50), p_old_value JSONB, p_new_value JSONB, p_reason TEXT, p_ip_address INET, p_user_agent TEXT, p_request_id VARCHAR(100))
RETURNS UUID AS $$
DECLARE action_id UUID;
BEGIN
    INSERT INTO shop_admin_actions (shop_id, admin_id, admin_email, admin_role, action_type, old_value, new_value, reason, ip_address, user_agent, request_id)
    VALUES (p_shop_id, p_admin_id, p_admin_email, p_admin_role, p_action_type, p_old_value, p_new_value, p_reason, p_ip_address, p_user_agent, p_request_id)
    RETURNING id INTO action_id;
    RETURN action_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION calculate_shop_health_score(shop_id_param UUID) RETURNS INT AS $$
DECLARE score INT := 1000; shop_record RECORD;
BEGIN
    SELECT * INTO shop_record FROM shops WHERE id = shop_id_param;
    IF shop_record IS NULL THEN RETURN 0; END IF;
    IF shop_record.kyc_status != 'verified' THEN score := score - 200; END IF;
    IF NOT shop_record.is_active THEN score := score - 300; END IF;
    IF shop_record.suspended_at IS NOT NULL THEN score := score - 500; END IF;
    IF shop_record.created_at < NOW() - INTERVAL '6 months' THEN score := score + 100; END IF;
    RETURN GREATEST(0, LEAST(1000, score));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- PARTIE 7 : MISE À JOUR DES DONNÉES EXISTANTES (Recalcul final)
-- ============================================================
UPDATE shops SET health_score = calculate_shop_health_score(id) WHERE id IN (SELECT id FROM shops);

-- ============================================================
-- PARTIE 8 : DOCUMENTATION
-- ============================================================
COMMENT ON COLUMN shops.health_score IS 'Score de santé de la boutique (0-1000).';
COMMENT ON COLUMN shops.health_level IS 'Niveau de santé : excellent, good, warning, critical';
COMMENT ON COLUMN shops.suspended_at IS 'Date de suspension par un admin.';
COMMENT ON COLUMN shops.suspension_reason IS 'Raison de la suspension.';
COMMENT ON COLUMN shops.admin_notes IS 'Notes internes admin.';
COMMENT ON TABLE shop_admin_actions IS 'Audit trail de toutes les actions admin sur les shops';

-- ============================================================
-- PARTIE 9 : VÉRIFICATION
-- ============================================================
DO $$
DECLARE total_shops INT; excellent_count INT; good_count INT; warning_count INT; critical_count INT;
BEGIN
    SELECT COUNT(*) INTO total_shops FROM shops;
    SELECT COUNT(*) INTO excellent_count FROM shops WHERE health_level = 'excellent';
    SELECT COUNT(*) INTO good_count FROM shops WHERE health_level = 'good';
    SELECT COUNT(*) INTO warning_count FROM shops WHERE health_level = 'warning';
    SELECT COUNT(*) INTO critical_count FROM shops WHERE health_level = 'critical';
    
    RAISE NOTICE '✅ Migration 020 terminée avec succès';
    RAISE NOTICE '📊 Distribution Health Score : Total: %, Excellent: %, Good: %, Warning: %, Critical: %', total_shops, excellent_count, good_count, warning_count, critical_count;
END $$;