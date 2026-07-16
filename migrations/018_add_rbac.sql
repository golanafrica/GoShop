-- Migration 018 : RBAC (Role-Based Access Control) + Audit Trail
-- Date: 2026-07-02
-- Description: Ajout système de rôles, permissions et audit pour admin

-- ============================================================
-- PARTIE 1 : AJOUT COLONNES RBAC SUR USERS
-- ============================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(50) NOT NULL DEFAULT 'merchant';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(30) NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '[]'::jsonb;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_by VARCHAR(36);
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_by VARCHAR(36);

-- Contraintes (idempotentes)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_role_check' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users DROP CONSTRAINT users_role_check;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_status_check' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users DROP CONSTRAINT users_status_check;
    END IF;

    ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('super_admin', 'admin', 'credit_analyst', 'support_agent', 'moderator', 'merchant'));
    ALTER TABLE users ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'pending', 'suspended', 'banned', 'deleted'));
END $$;

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_last_login ON users(last_login_at DESC);

-- ============================================================
-- PARTIE 2 : TABLE ROLES
-- ============================================================

CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (id, name, description, permissions, is_system) VALUES
    ('super_admin', 'Super Administrateur', 'Accès total au système', '["*"]'::jsonb, true),
    ('admin', 'Administrateur', 'Gestion des boutiques et utilisateurs', '["users.read", "users.write", "shops.read", "shops.write", "shops.suspend", "transactions.read", "transactions.refund", "reports.read", "reports.export"]'::jsonb, true),
    ('credit_analyst', 'Analyste Crédit', 'Analyse des demandes de crédit', '["credit.read", "credit.analyze", "credit.approve", "credit.reject", "reports.read"]'::jsonb, true),
    ('support_agent', 'Agent Support', 'Support client et modération', '["users.read", "shops.read", "orders.read", "customers.read", "tickets.read", "tickets.write"]'::jsonb, true),
    ('moderator', 'Modérateur', 'Modération de contenu et KYC', '["users.read", "kyc.read", "kyc.validate", "kyc.reject", "content.moderate"]'::jsonb, true),
    ('merchant', 'Marchand', 'Accès à sa propre boutique', '["shop.own.read", "shop.own.write", "products.own", "orders.own", "customers.own", "payments.own"]'::jsonb, true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- PARTIE 3 : TABLE PERMISSIONS
-- ============================================================

CREATE TABLE IF NOT EXISTS permissions (
    id VARCHAR(100) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO permissions (id, name, description, category) VALUES
    ('users.read', 'Lire les utilisateurs', 'Voir la liste des utilisateurs', 'users'),
    ('users.write', 'Modifier les utilisateurs', 'Créer/modifier des utilisateurs', 'users'),
    ('users.delete', 'Supprimer les utilisateurs', 'Supprimer des utilisateurs', 'users'),
    ('shops.read', 'Lire les boutiques', 'Voir la liste des boutiques', 'shops'),
    ('shops.write', 'Modifier les boutiques', 'Créer/modifier des boutiques', 'shops'),
    ('shops.suspend', 'Suspendre les boutiques', 'Suspendre/activer des boutiques', 'shops'),
    ('shops.billing', 'Gérer la facturation', 'Gérer les abonnements', 'shops'),
    ('transactions.read', 'Lire les transactions', 'Voir les transactions', 'transactions'),
    ('transactions.refund', 'Rembourser', 'Effectuer des remboursements', 'transactions'),
    ('transactions.freeze', 'Geler les comptes', 'Geler/dégeler des comptes', 'transactions'),
    ('credit.read', 'Lire les crédits', 'Voir les demandes de crédit', 'credit'),
    ('credit.analyze', 'Analyser les crédits', 'Analyser les demandes', 'credit'),
    ('credit.approve', 'Approuver les crédits', 'Approuver les demandes', 'credit'),
    ('credit.reject', 'Rejeter les crédits', 'Rejeter les demandes', 'credit'),
    ('reports.read', 'Lire les rapports', 'Voir les rapports', 'reports'),
    ('reports.export', 'Exporter les rapports', 'Exporter en CSV/PDF', 'reports'),
    ('reports.bceao', 'Rapports BCEAO', 'Générer rapports réglementaires', 'reports'),
    ('system.config', 'Configuration système', 'Modifier la config globale', 'system'),
    ('system.audit', 'Audit système', 'Voir les logs d''audit', 'system'),
    ('system.backup', 'Sauvegardes', 'Gérer les sauvegardes', 'system'),
    ('shop.own.read', 'Lire sa boutique', 'Voir sa propre boutique', 'own'),
    ('shop.own.write', 'Modifier sa boutique', 'Modifier sa propre boutique', 'own'),
    ('products.own', 'Gérer ses produits', 'CRUD produits personnels', 'own'),
    ('orders.own', 'Gérer ses commandes', 'CRUD commandes personnelles', 'own'),
    ('customers.own', 'Gérer ses clients', 'CRUD clients personnels', 'own'),
    ('payments.own', 'Gérer ses paiements', 'Voir ses paiements', 'own')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- PARTIE 4 : TABLE ADMIN_AUDIT_LOGS
-- ============================================================

CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50),
    target_id VARCHAR(36),
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_admin ON admin_audit_logs(admin_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON admin_audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON admin_audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON admin_audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_ip ON admin_audit_logs(ip_address);

-- ============================================================
-- PARTIE 5 : MIGRATION DES DONNÉES EXISTANTES
-- ============================================================

UPDATE users SET role = 'merchant' WHERE role IS NULL OR role = '';
UPDATE users SET is_active = true, status = 'active' WHERE is_active IS NULL;

DO $$
DECLARE super_admin_exists BOOLEAN;
BEGIN
    SELECT EXISTS(SELECT 1 FROM users WHERE role = 'super_admin') INTO super_admin_exists;
    IF NOT super_admin_exists THEN
        INSERT INTO users (id, email, password, role, is_active, status, created_at, updated_at)
        VALUES (gen_random_uuid()::varchar, 'admin@goshop.com', '$2a$04$YourBcryptHashHere', 'super_admin', true, 'active', NOW(), NOW());
        RAISE NOTICE '✅ Super admin créé : admin@goshop.com';
    END IF;
END $$;

-- ============================================================
-- PARTIE 6 : FONCTIONS UTILITAIRES
-- ============================================================

CREATE OR REPLACE FUNCTION user_has_permission(user_id_param VARCHAR(36), permission_param VARCHAR(100))
RETURNS BOOLEAN AS $$
DECLARE user_role VARCHAR(50); role_permissions JSONB;
BEGIN
    SELECT role INTO user_role FROM users WHERE id = user_id_param;
    IF user_role = 'super_admin' THEN RETURN true; END IF;
    SELECT permissions INTO role_permissions FROM roles WHERE id = user_role;
    RETURN role_permissions ? permission_param OR role_permissions ? '*';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION log_admin_action(admin_id VARCHAR(36), action_param VARCHAR(100), target_type_param VARCHAR(50), target_id_param VARCHAR(36), old_val JSONB, new_val JSONB, ip_param INET, ua_param TEXT, req_id VARCHAR(100))
RETURNS UUID AS $$
DECLARE log_id UUID;
BEGIN
    INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, old_values, new_values, ip_address, user_agent, request_id)
    VALUES (admin_id, action_param, target_type_param, target_id_param, old_val, new_val, ip_param, ua_param, req_id)
    RETURNING id INTO log_id;
    RETURN log_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- PARTIE 7 : VUES POUR RAPPORTS
-- ============================================================

CREATE OR REPLACE VIEW v_users_by_role AS
SELECT role, COUNT(*) as count,
    COUNT(CASE WHEN is_active = true THEN 1 END) as active_count,
    COUNT(CASE WHEN is_active = false THEN 1 END) as inactive_count
FROM users GROUP BY role ORDER BY count DESC;

CREATE OR REPLACE VIEW v_recent_admin_actions AS
SELECT a.id, a.created_at, u.email as admin_email, u.role as admin_role, a.action, a.target_type, a.target_id, a.ip_address
FROM admin_audit_logs a JOIN users u ON u.id = a.admin_user_id
ORDER BY a.created_at DESC LIMIT 100;

-- NOTE: La vue v_active_sessions a été définitivement supprimée d'ici.
-- Elle est gérée correctement par les migrations 025 et 025b.

-- ============================================================
-- PARTIE 8 : TRIGGERS D'AUDIT
-- ============================================================

CREATE OR REPLACE FUNCTION log_role_change() RETURNS TRIGGER AS $$
BEGIN
    IF OLD.role IS DISTINCT FROM NEW.role THEN
        PERFORM log_admin_action(NEW.updated_by, 'user_role_changed', 'user', NEW.id, jsonb_build_object('role', OLD.role), jsonb_build_object('role', NEW.role), NULL, NULL, NULL);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_log_role_change ON users;
CREATE TRIGGER trigger_log_role_change AFTER UPDATE ON users FOR EACH ROW WHEN (OLD.role IS DISTINCT FROM NEW.role) EXECUTE FUNCTION log_role_change();

CREATE OR REPLACE FUNCTION log_status_change() RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_active IS DISTINCT FROM NEW.is_active THEN
        PERFORM log_admin_action(NEW.updated_by, CASE WHEN NEW.is_active = true THEN 'user_activated' ELSE 'user_deactivated' END, 'user', NEW.id, jsonb_build_object('is_active', OLD.is_active, 'status', OLD.status), jsonb_build_object('is_active', NEW.is_active, 'status', NEW.status), NULL, NULL, NULL);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_log_status_change ON users;
CREATE TRIGGER trigger_log_status_change AFTER UPDATE ON users FOR EACH ROW WHEN (OLD.is_active IS DISTINCT FROM NEW.is_active) EXECUTE FUNCTION log_status_change();

-- ============================================================
-- PARTIE 9 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE users_count INT; super_admin_count INT;
BEGIN
    SELECT COUNT(*) INTO users_count FROM users;
    SELECT COUNT(*) INTO super_admin_count FROM users WHERE role = 'super_admin';
    RAISE NOTICE '✅ Migration 018 terminée';
    RAISE NOTICE '   - Total utilisateurs: %', users_count;
    RAISE NOTICE '   - Super admins: %', super_admin_count;
END $$;

COMMENT ON TABLE users IS 'Utilisateurs avec système RBAC';
COMMENT ON COLUMN users.role IS 'Rôle: super_admin, admin, credit_analyst, support_agent, moderator, merchant';
COMMENT ON COLUMN users.is_active IS 'Utilisateur actif ou désactivé';
COMMENT ON COLUMN users.status IS 'Statut: active, pending, suspended, banned, deleted';
COMMENT ON COLUMN users.permissions IS 'Permissions granulaires (JSONB array)';
COMMENT ON TABLE roles IS 'Définition des rôles système';
COMMENT ON TABLE permissions IS 'Définition des permissions granulaires';
COMMENT ON TABLE admin_audit_logs IS 'Audit trail des actions admin';