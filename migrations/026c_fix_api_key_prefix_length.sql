-- ============================================================
-- Migration 026c : Correction taille colonne key_prefix
-- Date: 2026-07-06
-- Version: v4.4.3 (hotfix)
-- Description : Augmente la taille de la colonne key_prefix
--               de VARCHAR(20) a VARCHAR(50) pour accommoder
--               les prefixes de cles API complets.
-- ============================================================
--
-- 🐛 Problème :
--   Le code genere des prefixes de 21 caracteres :
--   - gsk_test_ (9) + 12 chars aleatoires = 21
--   - gsk_live_ (9) + 12 chars aleatoires = 21
--   Mais la colonne etait limitee a 20 caracteres.
--
-- 🔧 Solution :
--   1. Supprimer les vues dependantes
--   2. Modifier la colonne
--   3. Recreer les vues
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : SUPPRESSION DES VUES DÉPENDANTES
-- ============================================================

DROP VIEW IF EXISTS v_active_api_keys CASCADE;
DROP VIEW IF EXISTS v_api_key_statistics CASCADE;

-- ============================================================
-- PARTIE 2 : MODIFICATION DE LA COLONNE
-- ============================================================

ALTER TABLE api_keys 
    ALTER COLUMN key_prefix TYPE VARCHAR(50);

-- ============================================================
-- PARTIE 3 : RECRÉATION DES VUES
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
-- PARTIE 4 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE
    col_type TEXT;
BEGIN
    SELECT data_type || '(' || character_maximum_length || ')'
    INTO col_type
    FROM information_schema.columns
    WHERE table_name = 'api_keys' AND column_name = 'key_prefix';
    
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 026c terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Colonne modifiee :';
    RAISE NOTICE '   - api_keys.key_prefix : %', col_type;
    RAISE NOTICE 'Taille precedente : VARCHAR(20)';
    RAISE NOTICE 'Nouvelle taille : VARCHAR(50)';
    RAISE NOTICE 'Marge disponible : +30 caracteres';
    RAISE NOTICE 'Vues recreees :';
    RAISE NOTICE '   - v_active_api_keys';
    RAISE NOTICE '   - v_api_key_statistics';
    RAISE NOTICE '==============================================================';
END $$;