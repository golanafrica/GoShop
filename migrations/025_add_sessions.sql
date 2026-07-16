-- ============================================================
-- Migration 025 : Session Management
-- Date: 2026-07-06
-- Version: v4.4.2
-- Description : Ajoute la gestion des sessions utilisateurs
--               pour permettre aux admins de voir et revoquer
--               leurs sessions actives.
-- ============================================================

-- ============================================================
-- PARTIE 1 : TABLE user_sessions (Mise à jour forcée)
-- ============================================================

-- 1. Supprimer les anciennes vues qui pourraient dépendre de l'ancienne structure
DROP VIEW IF EXISTS v_active_sessions CASCADE;
DROP VIEW IF EXISTS v_session_statistics CASCADE;

-- 2. Supprimer l'ancienne table (celle créée prématurément par la migration 018)
DROP TABLE IF EXISTS user_sessions CASCADE;

-- 3. Créer la nouvelle structure complète et correcte
CREATE TABLE user_sessions (
    -- Identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Session
    session_token_hash VARCHAR(255) NOT NULL,  -- Hash SHA-256 du token JWT
    session_id VARCHAR(255) NOT NULL UNIQUE,   -- ID unique de session (jti dans JWT)
    
    -- Device info (JSONB)
    device_info JSONB NOT NULL DEFAULT '{}',
    
    -- Localisation
    ip_address INET NOT NULL,
    
    -- Activite
    last_activity TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    
    -- Statut
    is_active BOOLEAN NOT NULL DEFAULT true,
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(255),
    
    -- Contraintes
    CONSTRAINT user_sessions_expires_check CHECK (expires_at > created_at)
);

-- ============================================================
-- PARTIE 2 : INDEX
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id 
    ON user_sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_user_sessions_active 
    ON user_sessions(user_id, is_active) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_user_sessions_expires 
    ON user_sessions(expires_at) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_user_sessions_session_id 
    ON user_sessions(session_id);

-- ============================================================
-- PARTIE 3 : TRIGGERS
-- ============================================================

CREATE OR REPLACE FUNCTION update_user_sessions_last_activity()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_activity = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_sessions_last_activity ON user_sessions;
CREATE TRIGGER trigger_user_sessions_last_activity
    BEFORE UPDATE OF last_activity ON user_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_user_sessions_last_activity();

-- ============================================================
-- PARTIE 4 : VUES
-- ============================================================

CREATE OR REPLACE VIEW v_active_sessions AS
SELECT 
    us.id,
    us.user_id,
    u.email,
    u.role,
    us.session_id,
    us.device_info,
    us.ip_address,
    us.last_activity,
    us.created_at,
    us.expires_at,
    EXTRACT(EPOCH FROM (NOW() - us.last_activity)) / 60 AS minutes_inactive,
    CASE 
        WHEN EXTRACT(EPOCH FROM (NOW() - us.last_activity)) / 60 > 60 THEN 'idle'
        WHEN EXTRACT(EPOCH FROM (NOW() - us.last_activity)) / 60 > 15 THEN 'away'
        ELSE 'active'
    END AS status
FROM user_sessions us
JOIN users u ON us.user_id = u.id
WHERE us.is_active = true AND us.expires_at > NOW();

CREATE OR REPLACE VIEW v_session_statistics AS
SELECT 
    COUNT(*) FILTER (WHERE is_active = true AND expires_at > NOW()) AS total_active,
    COUNT(*) FILTER (WHERE is_active = false) AS total_revoked,
    COUNT(*) FILTER (WHERE expires_at <= NOW()) AS total_expired,
    COUNT(DISTINCT user_id) FILTER (WHERE is_active = true AND expires_at > NOW()) AS unique_users_active,
    AVG(EXTRACT(EPOCH FROM (NOW() - last_activity)) / 60) FILTER (WHERE is_active = true) AS avg_idle_minutes
FROM user_sessions;

-- ============================================================
-- PARTIE 5 : FONCTION DE NETTOYAGE
-- ============================================================

CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM user_sessions
    WHERE expires_at <= NOW() OR (is_active = false AND revoked_at < NOW() - INTERVAL '30 days');
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- PARTIE 6 : COMMENTAIRES
-- ============================================================

COMMENT ON TABLE user_sessions IS 'Sessions actives des utilisateurs pour gestion et audit';
COMMENT ON COLUMN user_sessions.session_token_hash IS 'Hash SHA-256 du token JWT (jamais stocker le token en clair)';
COMMENT ON COLUMN user_sessions.session_id IS 'ID unique de session (jti claim dans JWT)';
COMMENT ON COLUMN user_sessions.device_info IS 'Informations sur l appareil (user agent, browser, OS)';
COMMENT ON COLUMN user_sessions.is_active IS 'true si session active, false si revoquee';
COMMENT ON VIEW v_active_sessions IS 'Sessions actives avec statut (active/away/idle)';
COMMENT ON VIEW v_session_statistics IS 'Statistiques globales sur les sessions';
COMMENT ON FUNCTION cleanup_expired_sessions() IS 'Nettoie les sessions expirees et revoquees depuis plus de 30 jours';

-- ============================================================
-- PARTIE 7 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 025 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Tables recreees :';
    RAISE NOTICE '   - user_sessions (structure v2 avec session_id et device_info)';
    RAISE NOTICE 'Vues recreees :';
    RAISE NOTICE '   - v_active_sessions';
    RAISE NOTICE '   - v_session_statistics';
    RAISE NOTICE '==============================================================';
END $$;