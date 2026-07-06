-- ============================================================
-- Migration 023 : 2FA (Two-Factor Authentication)
-- Date: 2026-07-05
-- Version: v4.4.0
-- Description : Ajoute le support 2FA pour les administrateurs
--               avec TOTP (Google Authenticator) et codes de
--               recuperation.
-- ============================================================
--
-- 🎯 Objectif :
--   - Permettre l'activation de la 2FA pour les admins
--   - Supporter TOTP (Google Authenticator, Authy, 1Password)
--   - Codes de recuperation pour backup (10 codes uniques)
--   - Chiffrement AES du secret et des codes
--
-- 🔐 Securite :
--   - Secret TOTP chiffre avec AES-256
--   - Codes de recuperation chiffres
--   - Contrainte UNIQUE sur user_id (1 seul setup par user)
--   - Index pour recherche rapide
--
-- 📊 Tables creees :
--   - user_2fa : Configuration 2FA par utilisateur
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : TABLE user_2fa
-- ============================================================

CREATE TABLE IF NOT EXISTS user_2fa (
    -- Identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Configuration TOTP
    secret_encrypted TEXT NOT NULL,  -- Secret TOTP chiffre avec AES
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    
    -- Codes de recuperation (chiffres)
    recovery_codes_encrypted TEXT,  -- JSON array de 10 codes chiffres
    recovery_codes_used INTEGER[] DEFAULT '{}',  -- Index des codes utilises
    
    -- Securite
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_verified_at TIMESTAMPTZ,
    
    -- Audit
    setup_at TIMESTAMPTZ DEFAULT NOW(),
    enabled_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    last_ip_address INET,
    last_user_agent TEXT,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Contraintes
    CONSTRAINT user_2fa_user_unique UNIQUE (user_id),
    CONSTRAINT user_2fa_failed_attempts_check CHECK (failed_attempts >= 0)
);

-- ============================================================
-- PARTIE 2 : INDEX
-- ============================================================

-- Index pour recherche rapide par user_id
CREATE INDEX IF NOT EXISTS idx_user_2fa_user_id 
    ON user_2fa(user_id);

-- Index pour les utilisateurs avec 2FA activee
CREATE INDEX IF NOT EXISTS idx_user_2fa_enabled 
    ON user_2fa(user_id) WHERE is_enabled = true;

-- Index pour les comptes verrouilles
CREATE INDEX IF NOT EXISTS idx_user_2fa_locked 
    ON user_2fa(user_id) WHERE locked_until IS NOT NULL;

-- ============================================================
-- PARTIE 3 : TRIGGER updated_at
-- ============================================================

-- Trigger pour mise a jour automatique de updated_at
CREATE OR REPLACE FUNCTION update_user_2fa_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_2fa_updated_at ON user_2fa;
CREATE TRIGGER trigger_user_2fa_updated_at
    BEFORE UPDATE ON user_2fa
    FOR EACH ROW
    EXECUTE FUNCTION update_user_2fa_updated_at();

-- ============================================================
-- PARTIE 4 : VUES
-- ============================================================

-- Vue pour dashboard admin (sans donnees sensibles)
CREATE OR REPLACE VIEW v_user_2fa_status AS
SELECT 
    u.id AS user_id,
    u.email,
    u.role,
    COALESCE(u2fa.is_enabled, false) AS is_2fa_enabled,
    u2fa.setup_at,
    u2fa.enabled_at,
    u2fa.last_verified_at,
    u2fa.failed_attempts,
    CASE 
        WHEN u2fa.locked_until IS NOT NULL AND u2fa.locked_until > NOW() THEN true
        ELSE false
    END AS is_locked,
    u2fa.locked_until
FROM users u
LEFT JOIN user_2fa u2fa ON u.id = u2fa.user_id
WHERE u.role IN ('super_admin', 'admin', 'credit_analyst', 'support_agent', 'moderator');

-- Vue pour statistiques 2FA
CREATE OR REPLACE VIEW v_2fa_statistics AS
SELECT 
    COUNT(*) FILTER (WHERE is_enabled = true) AS total_enabled,
    COUNT(*) FILTER (WHERE is_enabled = false) AS total_disabled,
    COUNT(*) FILTER (WHERE locked_until > NOW()) AS total_locked,
    COUNT(*) AS total_users_with_2fa,
    AVG(failed_attempts) AS avg_failed_attempts,
    MAX(failed_attempts) AS max_failed_attempts
FROM user_2fa;

-- ============================================================
-- PARTIE 5 : COMMENTAIRES
-- ============================================================

COMMENT ON TABLE user_2fa IS 
    'Configuration 2FA pour les utilisateurs avec TOTP et codes de recuperation';

COMMENT ON COLUMN user_2fa.secret_encrypted IS 
    'Secret TOTP chiffre avec AES-256 (jamais en clair)';

COMMENT ON COLUMN user_2fa.recovery_codes_encrypted IS 
    'JSON array de 10 codes de recuperation chiffres avec AES-256';

COMMENT ON COLUMN user_2fa.recovery_codes_used IS 
    'Index des codes de recuperation deja utilises (0-9)';

COMMENT ON COLUMN user_2fa.failed_attempts IS 
    'Nombre de tentatives echouees consecutives';

COMMENT ON COLUMN user_2fa.locked_until IS 
    'Date/heure jusqu a laquelle le compte est verrouille apres trop d echecs';

COMMENT ON VIEW v_user_2fa_status IS 
    'Statut 2FA de tous les utilisateurs administrateurs (sans donnees sensibles)';

COMMENT ON VIEW v_2fa_statistics IS 
    'Statistiques globales sur l adoption de la 2FA';

-- ============================================================
-- PARTIE 6 : VERIFICATION
-- ============================================================

DO $$
BEGIN
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Migration 023 terminee avec succes';
    RAISE NOTICE '==============================================================';
    RAISE NOTICE 'Tables creees :';
    RAISE NOTICE '   - user_2fa (configuration 2FA)';
    RAISE NOTICE 'Vues creees :';
    RAISE NOTICE '   - v_user_2fa_status (statut par user)';
    RAISE NOTICE '   - v_2fa_statistics (stats globales)';
    RAISE NOTICE 'Securite :';
    RAISE NOTICE '   - Secret TOTP chiffre avec AES-256';
    RAISE NOTICE '   - Codes de recuperation chiffres';
    RAISE NOTICE '   - Contrainte UNIQUE sur user_id';
    RAISE NOTICE '   - Index optimises';
    RAISE NOTICE '==============================================================';
END $$;