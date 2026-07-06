-- ============================================================
-- Migration 025b : Correction table user_sessions
-- Date: 2026-07-06
-- Version: v4.4.2
-- Description : Supprime l'ancienne table user_sessions et
--               la recree avec la bonne structure.
-- ============================================================

-- ============================================================
-- PARTIE 1 : SUPPRESSION ANCIENNE STRUCTURE
-- ============================================================

-- Supprimer les vues dependantes
DROP VIEW IF EXISTS v_active_sessions CASCADE;
DROP VIEW IF EXISTS v_session_statistics CASCADE;

-- Supprimer l'ancienne table
DROP TABLE IF EXISTS user_sessions CASCADE;

-- ============================================================
-- PARTIE 2 : RECREATION TABLE
-- ============================================================

CREATE TABLE user_sessions (
    -- Identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Session
    session_token_hash VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL UNIQUE,
    
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
-- PARTIE 3 : INDEX
-- ============================================================

CREATE INDEX idx_user_sessions_user_id 
    ON user_sessions(user_id);

CREATE INDEX idx_user_sessions_active 
    ON user_sessions(user_id, is_active) WHERE is_active = true;

CREATE INDEX idx_user_sessions_expires 
    ON user_sessions(expires_at) WHERE is_active = true;

CREATE INDEX idx_user_sessions_session_id 
    ON user_sessions(session_id);

-- ============================================================
-- PARTIE 4 : TRIGGERS
-- ============================================================

CREATE OR REPLACE FUNCTION update_user_sessions_last_activity()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_activity = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_user_sessions_last_activity
    BEFORE UPDATE OF last_activity ON user_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_user_sessions_last_activity();

-- ============================================================
-- PARTIE 5 : VUES
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
-- PARTIE 6 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 025b terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Table recreee :';
    RAISE NOTICE '   - user_sessions (avec toutes les colonnes)';
    RAISE NOTICE 'Vues recreees :';
    RAISE NOTICE '   - v_active_sessions';
    RAISE NOTICE '   - v_session_statistics';
    RAISE NOTICE '==============================================================';
END $$;