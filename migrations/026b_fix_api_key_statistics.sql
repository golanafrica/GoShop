-- ============================================================
-- Migration 026b : Correction vue v_api_key_statistics
-- Date: 2026-07-06
-- Version: v4.4.3
-- Description : Recree la vue v_api_key_statistics qui a
--               echoue lors de la migration 026.
-- ============================================================

-- ============================================================
-- PARTIE 1 : SUPPRESSION ANCIENNE VUE (si existe)
-- ============================================================

DROP VIEW IF EXISTS v_api_key_statistics CASCADE;

-- ============================================================
-- PARTIE 2 : RECREATION VUE
-- ============================================================

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
-- PARTIE 3 : COMMENTAIRE
-- ============================================================

COMMENT ON VIEW v_api_key_statistics IS 
    'Statistiques globales sur les cles API';

-- ============================================================
-- PARTIE 4 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 026b terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Vue recreee :';
    RAISE NOTICE '   - v_api_key_statistics (stats globales)';
    RAISE NOTICE '==============================================================';
END $$;