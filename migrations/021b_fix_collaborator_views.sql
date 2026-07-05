-- ============================================================
-- Migration 021b : Correction des vues collaborateurs
-- Date: 2026-07-04
-- Version: v4.3.0-fix1
-- Description: Correction colonnes inexistantes (u.full_name)
-- ============================================================
--
-- 🐛 Problème :
--   Les vues v_platform_collaborators et v_shop_collaborators
--   référençaient u.full_name qui n'existe pas dans la table users.
--   La table users n'a que : id, email, password_hash, role, etc.
--
-- ✅ Solution :
--   Supprimer la colonne u.full_name des vues.
--   L'email suffit pour identifier le collaborateur.
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : CORRECTION v_platform_collaborators
-- ============================================================

DROP VIEW IF EXISTS v_platform_collaborators;

CREATE OR REPLACE VIEW v_platform_collaborators AS
SELECT 
    pc.id,
    pc.user_id,
    u.email,
    pc.role,
    pc.permissions,
    pc.is_active,
    pc.invited_by,
    inv.email as invited_by_email,
    pc.invited_at,
    pc.accepted_at,
    pc.last_login_at,
    pc.last_activity_at,
    EXTRACT(DAY FROM NOW() - pc.accepted_at) as days_since_acceptance
FROM platform_collaborators pc
JOIN users u ON u.id = pc.user_id
LEFT JOIN users inv ON inv.id = pc.invited_by
WHERE pc.is_active = true AND pc.deleted_at IS NULL
ORDER BY pc.created_at DESC;

COMMENT ON VIEW v_platform_collaborators IS 
    'Collaborateurs plateforme actifs. Colonnes : id, user_id, email, role, permissions, is_active, invited_by, invited_by_email, invited_at, accepted_at, last_login_at, last_activity_at, days_since_acceptance';

-- ============================================================
-- PARTIE 2 : CORRECTION v_shop_collaborators
-- ============================================================

DROP VIEW IF EXISTS v_shop_collaborators;

CREATE OR REPLACE VIEW v_shop_collaborators AS
SELECT 
    sc.id,
    sc.shop_id,
    s.name as shop_name,
    s.slug as shop_slug,
    sc.user_id,
    u.email,
    sc.role,
    sc.permissions,
    sc.is_active,
    sc.invited_by,
    inv.email as invited_by_email,
    sc.invited_at,
    sc.accepted_at,
    sc.last_login_at,
    sc.last_activity_at
FROM shop_collaborators sc
JOIN shops s ON s.id = sc.shop_id
JOIN users u ON u.id = sc.user_id
LEFT JOIN users inv ON inv.id = sc.invited_by
WHERE sc.is_active = true AND sc.deleted_at IS NULL
ORDER BY sc.created_at DESC;

COMMENT ON VIEW v_shop_collaborators IS 
    'Collaborateurs boutique actifs. Colonnes : id, shop_id, shop_name, shop_slug, user_id, email, role, permissions, is_active, invited_by, invited_by_email, invited_at, accepted_at, last_login_at, last_activity_at';

-- ============================================================
-- PARTIE 3 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE
    platform_count INT;
    shop_count INT;
BEGIN
    SELECT COUNT(*) INTO platform_count FROM v_platform_collaborators;
    SELECT COUNT(*) INTO shop_count FROM v_shop_collaborators;
    
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '✅ Migration 021b terminee avec succes';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '📊 Statistiques :';
    RAISE NOTICE '   - Collaborateurs plateforme : %', platform_count;
    RAISE NOTICE '   - Collaborateurs boutique   : %', shop_count;
    RAISE NOTICE '═══════════════════════════════════════════════════════';
END $$;