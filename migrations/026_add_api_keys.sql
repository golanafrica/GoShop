-- ============================================================
-- Migration 026 : API Keys Management
-- Date: 2026-07-06
-- Version: v4.4.3
-- Description : Ajoute la gestion des cles API pour acces
--               programmatique a l'API avec permissions
--               granulaires (scopes) et audit trail.
-- ============================================================
--
-- 🎯 Objectif :
--   - Permettre la creation de cles API avec scopes
--   - Hash SHA-256 des cles (jamais stockees en clair)
--   - Expiration configurable
--   - Rate limiting par cle
--   - Audit trail complet (usage logs)
--
-- 🔐 Securite :
--   - Cles API prefixees (gsk_live_, gsk_test_)
--   - Hash SHA-256 en base
--   - Affichage complet UNE SEULE FOIS a la creation
--   - Revocation immediate
--   - Scopes granulaires
--
-- 📊 Tables creees :
--   - api_keys : Cles API actives
--   - api_key_usage_logs : Audit trail des utilisations
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : TYPE ENUM pour scopes
-- ============================================================

-- Creation du type enum pour les scopes API
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'api_key_scope') THEN
        CREATE TYPE api_key_scope AS ENUM (
            'read:shops',
            'write:shops',
            'read:orders',
            'write:orders',
            'read:products',
            'write:products',
            'read:customers',
            'write:customers',
            'read:payments',
            'write:payments',
            'read:wallet',
            'write:wallet',
            'read:tontine',
            'write:tontine',
            'read:credit',
            'write:credit',
            'read:reports',
            'admin'
        );
    END IF;
END $$;

-- ============================================================
-- PARTIE 2 : TABLE api_keys
-- ============================================================

CREATE TABLE IF NOT EXISTS api_keys (
    -- Identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Informations
    name VARCHAR(255) NOT NULL,  -- Nom descriptif (ex: "Integration Shopify")
    description TEXT,
    
    -- Securite
    key_prefix VARCHAR(20) NOT NULL,  -- Prefixe lisible (ex: "gsk_live_abc123")
    key_hash VARCHAR(255) NOT NULL UNIQUE,  -- Hash SHA-256 de la cle complete
    
    -- Permissions
    scopes api_key_scope[] NOT NULL DEFAULT '{}',
    
    -- Expiration et statut
    expires_at TIMESTAMPTZ,  -- NULL = pas d'expiration
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Rate limiting
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    
    -- Revocation
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(255),
    revocation_reason TEXT,
    
    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_ip INET,
    created_user_agent TEXT,
    
    -- Contraintes
    CONSTRAINT api_keys_name_check CHECK (name <> ''),
    CONSTRAINT api_keys_rate_limit_minute_check CHECK (rate_limit_per_minute > 0),
    CONSTRAINT api_keys_rate_limit_day_check CHECK (rate_limit_per_day > 0)
);

-- ============================================================
-- PARTIE 3 : INDEX
-- ============================================================

-- Index pour recherche rapide par user_id
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id 
    ON api_keys(user_id);

-- Index pour les cles actives
CREATE INDEX IF NOT EXISTS idx_api_keys_active 
    ON api_keys(user_id, is_active) WHERE is_active = true;

-- Index pour recherche par hash (authentification)
CREATE INDEX IF NOT EXISTS idx_api_keys_hash 
    ON api_keys(key_hash);

-- Index pour recherche par prefixe
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix 
    ON api_keys(key_prefix);

-- Index pour les cles expirees (nettoyage)
CREATE INDEX IF NOT EXISTS idx_api_keys_expires 
    ON api_keys(expires_at) WHERE expires_at IS NOT NULL AND is_active = true;

-- ============================================================
-- PARTIE 4 : TABLE api_key_usage_logs (Audit Trail)
-- ============================================================

CREATE TABLE IF NOT EXISTS api_key_usage_logs (
    -- Identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    
    -- Requete
    method VARCHAR(10) NOT NULL,  -- GET, POST, PUT, DELETE
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    
    -- Client
    ip_address INET NOT NULL,
    user_agent TEXT,
    
    -- Performance
    response_time_ms INTEGER,
    
    -- Timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index pour recherche par api_key_id
CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_key_id 
    ON api_key_usage_logs(api_key_id);

-- Index pour recherche par date (partitionnement futur)
CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_created 
    ON api_key_usage_logs(created_at DESC);

-- Index composite pour statistiques
CREATE INDEX IF NOT EXISTS idx_api_key_usage_logs_key_date 
    ON api_key_usage_logs(api_key_id, created_at DESC);

-- ============================================================
-- PARTIE 5 : TRIGGERS
-- ============================================================

-- Trigger pour mise a jour automatique de updated_at
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

-- Trigger pour mise a jour de last_used_at
CREATE OR REPLACE FUNCTION update_api_key_last_used()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE api_keys 
    SET last_used_at = NEW.created_at
    WHERE id = NEW.api_key_id;
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

-- Vue pour dashboard admin (cles actives)
CREATE OR REPLACE VIEW v_active_api_keys AS
SELECT 
    ak.id,
    ak.user_id,
    u.email AS user_email,
    u.role AS user_role,
    ak.name,
    ak.description,
    ak.key_prefix,
    ak.scopes,
    ak.expires_at,
    ak.last_used_at,
    ak.is_active,
    ak.rate_limit_per_minute,
    ak.rate_limit_per_day,
    ak.created_at,
    -- Statistiques
    COALESCE(usage_stats.total_calls, 0) AS total_calls,
    COALESCE(usage_stats.calls_today, 0) AS calls_today,
    COALESCE(usage_stats.avg_response_ms, 0) AS avg_response_ms,
    -- Statut
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
    SELECT 
        api_key_id,
        COUNT(*) AS total_calls,
        COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') AS calls_today,
        AVG(response_time_ms) AS avg_response_ms
    FROM api_key_usage_logs
    GROUP BY api_key_id
) usage_stats ON ak.id = usage_stats.api_key_id;

-- Vue pour statistiques globales
CREATE OR REPLACE VIEW v_api_key_statistics AS
SELECT 
    COUNT(*) AS total_keys,
    COUNT(*) FILTER (WHERE is_active = true) AS active_keys,
    COUNT(*) FILTER (WHERE is_active = false) AS revoked_keys,
    COUNT(*) FILTER (WHERE expires_at < NOW()) AS expired_keys,
    COUNT(*) FILTER (WHERE last_used_at IS NULL) AS unused_keys,
    COUNT(DISTINCT user_id) AS unique_users,
    (SELECT COUNT(*) FROM api_key_usage_logs WHERE created_at > NOW() - INTERVAL '24 hours') AS calls_today,
    (SELECT COUNT(*) FROM api_key_usage_logs WHERE created_at > NOW() - INTERVAL '7 days') AS calls_last_7_days,
    (SELECT COUNT(*) FROM api_key_usage_logs) AS total_calls_all_time;

-- ============================================================
-- PARTIE 7 : FONCTIONS UTILITAIRES
-- ============================================================

-- Fonction pour nettoyer les vieux logs (> 90 jours)
CREATE OR REPLACE FUNCTION cleanup_old_api_key_logs(retention_days INTEGER DEFAULT 90)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM api_key_usage_logs
    WHERE created_at < NOW() - (retention_days || ' days')::INTERVAL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Fonction pour revoquer toutes les cles d'un user
CREATE OR REPLACE FUNCTION revoke_all_user_api_keys(
    p_user_id VARCHAR(255),
    p_revoked_by VARCHAR(255),
    p_reason TEXT
)
RETURNS INTEGER AS $$
DECLARE
    revoked_count INTEGER;
BEGIN
    UPDATE api_keys
    SET 
        is_active = false,
        revoked_at = NOW(),
        revoked_by = p_revoked_by,
        revocation_reason = p_reason
    WHERE user_id = p_user_id AND is_active = true;
    
    GET DIAGNOSTICS revoked_count = ROW_COUNT;
    RETURN revoked_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- PARTIE 8 : COMMENTAIRES
-- ============================================================

COMMENT ON TABLE api_keys IS 
    'Cles API pour acces programmatique avec scopes granulaires';

COMMENT ON COLUMN api_keys.key_prefix IS 
    'Prefixe lisible de la cle (ex: gsk_live_abc123) - affiche dans l UI';

COMMENT ON COLUMN api_keys.key_hash IS 
    'Hash SHA-256 de la cle complete - utilise pour authentification';

COMMENT ON COLUMN api_keys.scopes IS 
    'Tableau de scopes (permissions) accordees a la cle';

COMMENT ON TABLE api_key_usage_logs IS 
    'Audit trail des utilisations de cles API (conservation 90 jours)';

COMMENT ON VIEW v_active_api_keys IS 
    'Cles API actives avec statistiques d utilisation';

COMMENT ON VIEW v_api_key_statistics IS 
    'Statistiques globales sur les cles API';

COMMENT ON FUNCTION cleanup_old_api_key_logs(INTEGER) IS 
    'Nettoie les logs de plus de N jours (defaut 90)';

COMMENT ON FUNCTION revoke_all_user_api_keys(VARCHAR, VARCHAR, TEXT) IS 
    'Revoque toutes les cles API d un utilisateur';

-- ============================================================
-- PARTIE 9 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 026 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Tables creees :';
    RAISE NOTICE '   - api_keys (cles API avec scopes)';
    RAISE NOTICE '   - api_key_usage_logs (audit trail)';
    RAISE NOTICE 'Type enum cree :';
    RAISE NOTICE '   - api_key_scope (18 scopes disponibles)';
    RAISE NOTICE 'Vues creees :';
    RAISE NOTICE '   - v_active_api_keys (dashboard)';
    RAISE NOTICE '   - v_api_key_statistics (stats globales)';
    RAISE NOTICE 'Fonctions creees :';
    RAISE NOTICE '   - cleanup_old_api_key_logs()';
    RAISE NOTICE '   - revoke_all_user_api_keys()';
    RAISE NOTICE 'Securite :';
    RAISE NOTICE '   - Hash SHA-256 des cles';
    RAISE NOTICE '   - Prefixes lisibles (gsk_live_, gsk_test_)';
    RAISE NOTICE '   - Scopes granulaires';
    RAISE NOTICE '   - Rate limiting configurable';
    RAISE NOTICE '==============================================================';
END $$;