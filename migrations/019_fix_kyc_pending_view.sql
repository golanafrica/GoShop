-- ============================================================
-- Migration 019 FIX : Correction de la vue v_shops_kyc_pending
-- Date: 2026-07-03
-- Version: v4.1.0-fix1
-- Description: Corrige l'erreur "s.email n'existe pas"
-- ============================================================
--
-- 🐛 Problème original :
--   La table 'shops' n'a pas de colonne 'email'.
--   L'email est stocké dans la table 'users' (owner_id -> users.id)
--
-- ✅ Solution :
--   Ajouter un LEFT JOIN sur la table users pour récupérer owner_email
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : SUPPRESSION DE L'ANCIENNE VUE
-- ============================================================

DROP VIEW IF EXISTS v_shops_kyc_pending;

-- ============================================================
-- PARTIE 2 : RECRÉATION DE LA VUE CORRIGÉE
-- ============================================================

CREATE VIEW v_shops_kyc_pending AS
SELECT 
    s.id,
    s.name,
    s.slug,
    s.owner_id,
    u.email as owner_email,                          -- ✅ Email du propriétaire (via users)
    s.kyc_status,
    s.kyc_submitted_at,
    s.kyc_submissions_count,
    s.created_at,
    EXTRACT(DAY FROM NOW() - s.kyc_submitted_at) as days_waiting,
    COUNT(d.id) as documents_count
FROM shops s
LEFT JOIN users u ON u.id = s.owner_id               -- ✅ JOIN corrigé
LEFT JOIN shop_kyc_documents d 
    ON d.shop_id = s.id 
    AND d.status = 'pending'
WHERE s.kyc_status = 'pending'
GROUP BY 
    s.id, 
    s.name, 
    s.slug, 
    s.owner_id, 
    u.email,                                         -- ✅ Ajouté au GROUP BY
    s.kyc_status, 
    s.kyc_submitted_at, 
    s.kyc_submissions_count, 
    s.created_at
ORDER BY s.kyc_submitted_at ASC;

-- ============================================================
-- PARTIE 3 : DOCUMENTATION
-- ============================================================

COMMENT ON VIEW v_shops_kyc_pending IS 
    'Vue des shops en attente de vérification KYC (dashboard admin).
     Corrige v1 : JOIN sur users pour récupérer owner_email';

-- ============================================================
-- PARTIE 4 : VÉRIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '✅ Migration 019-fix appliquée avec succès';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '📊 Vue v_shops_kyc_pending recréée avec owner_email';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
END $$;