-- ============================================================
-- Migration 026 : API Keys Management
-- Date: 2026-07-06
-- Version: v4.4.3
-- Description : Ajoute la gestion des cles API pour acces
--               programmatique a l'API avec permissions
--               granulaires (scopes) et audit trail.
-- ============================================================

-- ============================================================
-- PARTIE 1 : TYPE ENUM pour scopes
-- ============================================================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'api_key_scope') THEN
        CREATE TYPE api_key_scope AS ENUM (
            'read:shops', 'write:shops', 'read:orders', 'write:orders',
            'read:products', 'write:products', 'read:customers', 'write:customers',
            'read:payments', 'write:payments', 'read:wallet', 'write:wallet',
            'read:tontine', 'write:tontine', 'read:credit', 'write:credit',
            'read:reports', 'admin'
        );
    END IF;
END $$;

-- ============================================================
-- PARTIE 2 : TABLE api_keys
-- ============================================================

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    key_prefix VARCHAR(50) NOT NULL,  -- ✅ Déjà corrigé en 50 pour éviter le hotfix 026c
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    scopes api_key_scope[] NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(255),
    revocation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_ip INET,
    created_user_agent TEXT,
    
    CONSTRAINT api_keys_name_check CHECK (name <> ''),
    CONSTRAINT api_keys_rate_limit_minute_check CHECK (rate_limit_per_minute > 0),
    CONSTRAINT api_keys_rate_limit_day_check CHECK (rate_limit_per_day > 0)
);

-- ============================================================
-- PARTIE 3 : INDEX
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(user_id, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires ON api_keys(expires_at) WHERE expires_at IS NOT NULL AND is_active = true;

-- ============================================================
-- PARTIE 4 : TABLE api_key_usage_logs (Audit Trail)
-- ============================================================

CREATE TABLE IF NOT EXISTS api_key_usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    method VARCHAR(10) NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    response_time_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_key_id ON api_key_usage_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_created ON api_key_usage_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_key_date ON api_key_usage_logs(api_key_id, created_at DESC);

-- ============================================================
-- PARTIE 5 : TRIGGERS
-- ============================================================

CREATE OR REPLACE FUNCTION update_api_keys_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_api_keys_updated_at ON api_keys;
CREATE TRIGGER trigger_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_api_keys_updated_at();

CREATE OR REPLACE FUNCTION update_api_key_last_used()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE api_keys SET last_used_at = NEW.created_at WHERE id = NEW.api_key_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_api_key_last_used ON api_key_usage_logs;
CREATE TRIGGER trigger_api_key_last_used
    AFTER INSERT ON api_key_usage_logs
    FOR EACH ROW
    EXECUTE FUNCTION update_api_key_last_used();

-- ============================================================
-- PARTIE 6 : VUES
-- ============================================================

CREATE OR REPLACE VIEW v_active_api_keys AS
SELECT 
    ak.id, ak.user_id, u.email AS user_email, u.role AS user_role,
    ak.name, ak.description, ak.key_prefix, ak.scopes, ak.expires_at,
    ak.last_used_at, ak.is_active, ak.rate_limit_per_minute, ak.rate_limit_per_day, ak.created_at,
    COALESCE(usage_stats.total_calls, 0) AS total_calls,
    COALESCE(usage_stats.calls_today, 0) AS calls_today,
    COALESCE(usage_stats.avg_response_ms, 0) AS avg_response_ms,
    CASE 
        WHEN ak.expires_at IS NOT NULL AND ak.expires_at < NOW() THEN 'expired'
        WHEN NOT ak.is_active THEN 'revoked'
        WHEN ak.last_used_at IS NULL THEN 'unused'
        WHEN ak.last_used_at < NOW() - INTERVAL '30 days' THEN 'inactive'
        ELSE 'active'
    END AS status
FROM api_keys ak
JOIN users u ON ak.user_id = u.id
LEFT JOIN (
    SELECT api_key_id, COUNT(*) AS total_calls,
           COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') AS calls_today,
           AVG(response_time_ms) AS avg_response_ms
    FROM api_key_usage_logs GROUP BY api_key_id
) usage_stats ON ak.id = usage_stats.api_key_id;

-- ✅ CORRECTION CRITIQUE : Ajout du FROM et des alias ak.
CREATE OR REPLACE VIEW v_api_key_statistics AS
SELECT 
    COUNT(*) AS total_keys,
    COUNT(*) FILTER (WHERE ak.is_active = true) AS active_keys,
    COUNT(*) FILTER (WHERE ak.is_active = false) AS revoked_keys,
    COUNT(*) FILTER (WHERE ak.expires_at < NOW()) AS expired_keys,
    COUNT(*) FILTER (WHERE ak.last_used_at IS NULL) AS unused_keys,
    COUNT(DISTINCT ak.user_id) AS unique_users,
    (SELECT COUNT(*) FROM api_key_usage_logs WHERE created_at > NOW() - INTERVAL '24 hours') AS calls_today,
    (SELECT COUNT(*) FROM api_key_usage_logs WHERE created_at > NOW() - INTERVAL '7 days') AS calls_last_7_days,
    (SELECT COUNT(*) FROM api_key_usage_logs) AS total_calls_all_time
FROM api_keys ak;

-- ============================================================
-- PARTIE 7 : FONCTIONS UTILITAIRES
-- ============================================================

CREATE OR REPLACE FUNCTION cleanup_old_api_key_logs(retention_days INTEGER DEFAULT 90)
RETURNS INTEGER AS $$
DECLARE deleted_count INTEGER;
BEGIN
    DELETE FROM api_key_usage_logs WHERE created_at < NOW() - (retention_days || ' days')::INTERVAL;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION revoke_all_user_api_keys(p_user_id VARCHAR(255), p_revoked_by VARCHAR(255), p_reason TEXT)
RETURNS INTEGER AS $$
DECLARE revoked_count INTEGER;
BEGIN
    UPDATE api_keys SET is_active = false, revoked_at = NOW(), revoked_by = p_revoked_by, revocation_reason = p_reason
    WHERE user_id = p_user_id AND is_active = true;
    GET DIAGNOSTICS revoked_count = ROW_COUNT;
    RETURN revoked_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- PARTIE 8 : COMMENTAIRES
-- ============================================================

COMMENT ON TABLE api_keys IS 'Cles API pour acces programmatique avec scopes granulaires';
COMMENT ON COLUMN api_keys.key_prefix IS 'Prefixe lisible de la cle (ex: gsk_live_abc123)';
COMMENT ON COLUMN api_keys.key_hash IS 'Hash SHA-256 de la cle complete';
COMMENT ON COLUMN api_keys.scopes IS 'Tableau de scopes (permissions) accordees a la cle';
COMMENT ON TABLE api_key_usage_logs IS 'Audit trail des utilisations de cles API';
COMMENT ON VIEW v_active_api_keys IS 'Cles API actives avec statistiques d utilisation';
COMMENT ON VIEW v_api_key_statistics IS 'Statistiques globales sur les cles API';

-- ============================================================
-- PARTIE 9 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 026 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Tables creees : api_keys, api_key_usage_logs';
    RAISE NOTICE 'Type enum cree : api_key_scope';
    RAISE NOTICE 'Vues creees : v_active_api_keys, v_api_key_statistics';
    RAISE NOTICE '==============================================================';
END $$;