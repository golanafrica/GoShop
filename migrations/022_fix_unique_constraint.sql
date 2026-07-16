-- ============================================================
-- Migration 022 : Fix unique constraints for soft delete
-- Date: 2026-07-05
-- Version: v4.3.1
-- Description: Modifier les contraintes UNIQUE pour ignorer
--              les enregistrements soft-deleted (deleted_at IS NOT NULL)
-- ============================================================
--
-- 🐛 Problème :
--   Les contraintes UNIQUE actuelles empêchent la création
--   d'un nouveau collaborateur si un ancien existe (soft deleted).
--
-- ✅ Solution :
--   Remplacer les contraintes UNIQUE par des partial unique indexes
--   qui ne s'appliquent qu'aux enregistrements actifs (deleted_at IS NULL).
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : PLATFORM_COLLABORATORS
-- ============================================================

-- Supprimer l'ancienne contrainte UNIQUE
ALTER TABLE platform_collaborators DROP CONSTRAINT IF EXISTS platform_collaborators_user_unique;

-- Créer un partial unique index (seulement pour deleted_at IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_collaborators_user_active 
    ON platform_collaborators(user_id) 
    WHERE deleted_at IS NULL;

-- ============================================================
-- PARTIE 2 : SHOP_COLLABORATORS
-- ============================================================

-- Supprimer l'ancienne contrainte UNIQUE
ALTER TABLE shop_collaborators DROP CONSTRAINT IF EXISTS shop_collaborators_shop_user_unique;

-- Créer un partial unique index (seulement pour deleted_at IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_shop_collaborators_shop_user_active 
    ON shop_collaborators(shop_id, user_id) 
    WHERE deleted_at IS NULL;

-- ============================================================
-- PARTIE 3 : VÉRIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE '✅ Migration 022 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE '📋 Changements :';
    RAISE NOTICE '   - platform_collaborators : partial unique index sur user_id';
    RAISE NOTICE '   - shop_collaborators : partial unique index sur (shop_id, user_id)';
    RAISE NOTICE '   - Les enregistrements soft-deleted ne bloquent plus';
    RAISE NOTICE '==============================================================';
END $$;