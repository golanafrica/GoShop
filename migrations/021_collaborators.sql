-- ============================================================
-- Migration 021 : Système de Collaborateurs à 2 niveaux
-- Date: 2026-07-04
-- Version: v4.3.0
-- Description: Collaborateurs plateforme + boutique avec invitations
-- ============================================================

-- ============================================================
-- PARTIE 1 : TABLE PLATFORM_COLLABORATORS
-- ============================================================

CREATE TABLE IF NOT EXISTS platform_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    role VARCHAR(50) NOT NULL
        CHECK (role IN (
            'finance_manager',
            'support_manager',
            'kyc_reviewer',
            'marketing_manager',
            'tech_admin'
        )),
    
    permissions JSONB NOT NULL DEFAULT '{}'::jsonb,
    invited_by VARCHAR(36) NOT NULL REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    deleted_at TIMESTAMPTZ,
    deleted_by VARCHAR(36) REFERENCES users(id),
    deletion_reason TEXT,
    last_login_at TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT platform_collaborators_user_unique UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_platform_collaborators_role ON platform_collaborators(role);
CREATE INDEX IF NOT EXISTS idx_platform_collaborators_active ON platform_collaborators(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_platform_collaborators_user ON platform_collaborators(user_id);
CREATE INDEX IF NOT EXISTS idx_platform_collaborators_deleted ON platform_collaborators(deleted_at) WHERE deleted_at IS NOT NULL;

-- ============================================================
-- PARTIE 2 : TABLE SHOP_COLLABORATORS
-- ============================================================

CREATE TABLE IF NOT EXISTS shop_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    role VARCHAR(50) NOT NULL
        CHECK (role IN (
            'shop_admin',
            'seller',
            'support',
            'accountant'
        )),
    
    permissions JSONB NOT NULL DEFAULT '{}'::jsonb,
    invited_by VARCHAR(36) NOT NULL REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    deleted_at TIMESTAMPTZ,
    deleted_by VARCHAR(36) REFERENCES users(id),
    deletion_reason TEXT,
    last_login_at TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT shop_collaborators_shop_user_unique UNIQUE (shop_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_shop_collaborators_shop ON shop_collaborators(shop_id);
CREATE INDEX IF NOT EXISTS idx_shop_collaborators_user ON shop_collaborators(user_id);
CREATE INDEX IF NOT EXISTS idx_shop_collaborators_role ON shop_collaborators(role);
CREATE INDEX IF NOT EXISTS idx_shop_collaborators_active ON shop_collaborators(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_shop_collaborators_deleted ON shop_collaborators(deleted_at) WHERE deleted_at IS NOT NULL;

-- ============================================================
-- PARTIE 3 : TABLE COLLABORATOR_INVITATIONS
-- ============================================================

CREATE TABLE IF NOT EXISTS collaborator_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invitation_type VARCHAR(20) NOT NULL
        CHECK (invitation_type IN ('platform', 'shop')),
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    permissions JSONB NOT NULL DEFAULT '{}'::jsonb,
    token VARCHAR(255) NOT NULL UNIQUE,
    invited_by VARCHAR(36) NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'expired', 'cancelled')),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitations_token ON collaborator_invitations(token);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON collaborator_invitations(email);
CREATE INDEX IF NOT EXISTS idx_invitations_status ON collaborator_invitations(status);
CREATE INDEX IF NOT EXISTS idx_invitations_expires ON collaborator_invitations(expires_at);
CREATE INDEX IF NOT EXISTS idx_invitations_type ON collaborator_invitations(invitation_type);

-- ============================================================
-- PARTIE 4 : TRIGGER AUTO-UPDATE updated_at
-- ============================================================

CREATE OR REPLACE FUNCTION update_collaborators_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_platform_collaborators_updated_at ON platform_collaborators;
CREATE TRIGGER trigger_platform_collaborators_updated_at
    BEFORE UPDATE ON platform_collaborators
    FOR EACH ROW
    EXECUTE FUNCTION update_collaborators_updated_at();

DROP TRIGGER IF EXISTS trigger_shop_collaborators_updated_at ON shop_collaborators;
CREATE TRIGGER trigger_shop_collaborators_updated_at
    BEFORE UPDATE ON shop_collaborators
    FOR EACH ROW
    EXECUTE FUNCTION update_collaborators_updated_at();

DROP TRIGGER IF EXISTS trigger_invitations_updated_at ON collaborator_invitations;
CREATE TRIGGER trigger_invitations_updated_at
    BEFORE UPDATE ON collaborator_invitations
    FOR EACH ROW
    EXECUTE FUNCTION update_collaborators_updated_at();

-- ============================================================
-- PARTIE 5 : VUES POUR DASHBOARD ADMIN
-- ============================================================

-- 5.1 Vue : Collaborateurs plateforme actifs
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

-- 5.2 Vue : Collaborateurs boutique actifs
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

-- 5.3 Vue : Invitations en attente
CREATE OR REPLACE VIEW v_pending_invitations AS
SELECT 
    ci.id,
    ci.invitation_type,
    ci.shop_id,
    s.name as shop_name,
    ci.email,
    ci.role,
    ci.permissions,
    ci.token,
    ci.invited_by,
    inv.email as invited_by_email,
    ci.invited_at,
    ci.expires_at,
    EXTRACT(DAY FROM ci.expires_at - NOW()) as days_until_expiration,
    ci.status,
    ci.message
FROM collaborator_invitations ci
LEFT JOIN shops s ON s.id = ci.shop_id
LEFT JOIN users inv ON inv.id = ci.invited_by
WHERE ci.status = 'pending' AND ci.expires_at > NOW()
ORDER BY ci.invited_at DESC;

-- 5.4 Vue : Statistiques collaborateurs
CREATE OR REPLACE VIEW v_collaborator_stats AS
SELECT 
    (SELECT COUNT(*) FROM platform_collaborators WHERE is_active = true AND deleted_at IS NULL) as platform_collaborators_count,
    (SELECT COUNT(*) FROM shop_collaborators WHERE is_active = true AND deleted_at IS NULL) as shop_collaborators_count,
    (SELECT COUNT(*) FROM collaborator_invitations WHERE status = 'pending' AND expires_at > NOW()) as pending_invitations_count,
    (SELECT COUNT(*) FROM collaborator_invitations WHERE status = 'accepted') as accepted_invitations_count,
    (SELECT COUNT(*) FROM collaborator_invitations WHERE status = 'expired') as expired_invitations_count,
    (SELECT COUNT(DISTINCT user_id) FROM shop_collaborators WHERE deleted_at IS NULL) as unique_collaborators_count;

-- 5.5 Vue : Permissions par rôle (référence)
CREATE OR REPLACE VIEW v_role_permissions AS
SELECT 
    'platform' as level,
    role,
    CASE role
        WHEN 'finance_manager' THEN '{"can_view_commissions": true, "can_export_reports": true, "can_view_shops": true}'::jsonb
        WHEN 'support_manager' THEN '{"can_manage_shops": true, "can_view_kyc": true}'::jsonb
        WHEN 'kyc_reviewer' THEN '{"can_approve_kyc": true, "can_view_kyc": true, "can_view_shops": true}'::jsonb
        WHEN 'marketing_manager' THEN '{"can_view_shops": true}'::jsonb
        WHEN 'tech_admin' THEN '{"can_manage_schedulers": true, "can_manage_settings": true, "can_invite_collaborators": true}'::jsonb
    END as default_permissions
FROM (VALUES ('finance_manager'), ('support_manager'), ('kyc_reviewer'), ('marketing_manager'), ('tech_admin')) AS roles(role)

UNION ALL

SELECT 
    'shop' as level,
    role,
    CASE role
        WHEN 'shop_admin' THEN '{"can_manage_products": true, "can_manage_orders": true, "can_manage_customers": true, "can_view_payments": true, "can_withdraw": true, "can_view_reports": true, "can_manage_settings": true}'::jsonb
        WHEN 'seller' THEN '{"can_manage_products": true, "can_manage_orders": true, "can_view_customers": true, "can_view_payments": true}'::jsonb
        WHEN 'support' THEN '{"can_view_products": true, "can_view_orders": true, "can_manage_customers": true, "can_view_payments": true}'::jsonb
        WHEN 'accountant' THEN '{"can_view_payments": true, "can_withdraw": true, "can_view_reports": true}'::jsonb
    END as default_permissions
FROM (VALUES ('shop_admin'), ('seller'), ('support'), ('accountant')) AS roles(role);

-- ============================================================
-- PARTIE 6 : FONCTIONS UTILITAIRES
-- ============================================================

CREATE OR REPLACE FUNCTION user_has_platform_permission(
    p_user_id VARCHAR(36),
    p_permission VARCHAR(100)
) RETURNS BOOLEAN AS $$
DECLARE
    has_perm BOOLEAN;
BEGIN
    SELECT EXISTS(
        SELECT 1 FROM platform_collaborators
        WHERE user_id = p_user_id
          AND is_active = true
          AND deleted_at IS NULL
          AND permissions @> jsonb_build_object(p_permission, true)
    ) INTO has_perm;
    
    RETURN COALESCE(has_perm, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION user_has_shop_permission(
    p_user_id VARCHAR(36),
    p_shop_id UUID,
    p_permission VARCHAR(100)
) RETURNS BOOLEAN AS $$
DECLARE
    has_perm BOOLEAN;
BEGIN
    SELECT EXISTS(
        SELECT 1 FROM shop_collaborators
        WHERE user_id = p_user_id
          AND shop_id = p_shop_id
          AND is_active = true
          AND deleted_at IS NULL
          AND (
              permissions @> jsonb_build_object(p_permission, true)
              OR permissions @> '{"shop_admin": true}'::jsonb
          )
    ) INTO has_perm;
    
    RETURN COALESCE(has_perm, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION generate_invitation_token()
RETURNS VARCHAR(255) AS $$
BEGIN
    RETURN encode(gen_random_bytes(32), 'hex');
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION cleanup_expired_invitations()
RETURNS INTEGER AS $$
DECLARE
    cleaned_count INTEGER;
BEGIN
    UPDATE collaborator_invitations
    SET status = 'expired',
        updated_at = NOW()
    WHERE status = 'pending'
      AND expires_at < NOW();
    
    GET DIAGNOSTICS cleaned_count = ROW_COUNT;
    
    RETURN cleaned_count;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- PARTIE 7 : DOCUMENTATION
-- ============================================================

COMMENT ON TABLE platform_collaborators IS 
    'Collaborateurs plateforme avec 5 rôles. Soft delete avec audit trail';

COMMENT ON TABLE shop_collaborators IS 
    'Collaborateurs boutique avec 4 rôles. Soft delete avec audit trail';

COMMENT ON TABLE collaborator_invitations IS 
    'Invitations en attente avec token sécurisé';

-- ============================================================
-- PARTIE 8 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE
    platform_count INT;
    shop_count INT;
    invitation_count INT;
BEGIN
    SELECT COUNT(*) INTO platform_count FROM platform_collaborators;
    SELECT COUNT(*) INTO shop_count FROM shop_collaborators;
    SELECT COUNT(*) INTO invitation_count FROM collaborator_invitations;
    
    RAISE NOTICE '✅ Migration 021 terminee avec succes';
    RAISE NOTICE '   - Collaborateurs plateforme : %', platform_count;
    RAISE NOTICE '   - Collaborateurs boutique   : %', shop_count;
    RAISE NOTICE '   - Invitations               : %', invitation_count;
END $$;