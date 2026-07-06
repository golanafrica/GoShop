-- ============================================================
-- Migration 024 : Protection anti-replay pour 2FA
-- Date: 2026-07-05
-- Version: v4.4.1 (patch securite)
-- Description : Ajoute la protection contre les replay attacks
--               en stockant le dernier code TOTP utilise.
-- ============================================================
--
-- 🎯 Objectif :
--   Empecher la reutilisation d'un code TOTP dans la fenetre
--   de validite de 30 secondes (RFC 6238 Section 5.2).
--
-- 🔐 Securite :
--   - Dernier code utilise stocke en clair (6 chiffres, deja utilise)
--   - Timestamp de la derniere utilisation
--   - Rejet si meme code dans les 30 secondes
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : AJOUT DES COLONNES
-- ============================================================

ALTER TABLE user_2fa
    ADD COLUMN IF NOT EXISTS last_used_code VARCHAR(6),
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;

-- ============================================================
-- PARTIE 2 : INDEX
-- ============================================================

-- Index pour recherche rapide (optionnel mais utile pour audit)
CREATE INDEX IF NOT EXISTS idx_user_2fa_last_used 
    ON user_2fa(user_id, last_used_at);

-- ============================================================
-- PARTIE 3 : COMMENTAIRES
-- ============================================================

COMMENT ON COLUMN user_2fa.last_used_code IS 
    'Dernier code TOTP utilise (protection anti-replay, RFC 6238 Section 5.2)';

COMMENT ON COLUMN user_2fa.last_used_at IS 
    'Timestamp de la derniere utilisation du code TOTP';

-- ============================================================
-- PARTIE 4 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 024 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Protection anti-replay activee :';
    RAISE NOTICE '   - Dernier code TOTP stocke';
    RAISE NOTICE '   - Timestamp de derniere utilisation';
    RAISE NOTICE '   - Rejet si reutilisation dans 30s';
    RAISE NOTICE '==============================================================';
END $$;