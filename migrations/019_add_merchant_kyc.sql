-- ============================================================
-- Migration 019 : KYC Marchand progressif
-- Date: 2026-07-03
-- Version: v4.1.0
-- Description: Workflow KYC pour marchands avec vérification progressive
-- ============================================================
--
-- 🎯 Objectif :
--   - Marchand peut créer une boutique immédiatement (is_active = true)
--   - Marchand peut vendre (COD, crédit, tontine, mobile money)
--   - Retraits (cash-out) BLOQUÉS jusqu'à vérification KYC
--   - Badge "✅ Marchand vérifié" après validation admin
--
-- 📊 Workflow :
--   unverified → pending → verified (ou rejected)
--
-- 🔐 Rôles pouvant valider :
--   - super_admin
--   - admin
--   - collaborator (à créer)
--   - support_agent (à créer)
--
-- ============================================================

-- ============================================================
-- PARTIE 1 : AJOUT COLONNES KYC SUR SHOPS
-- ============================================================

-- 1.1 Statut KYC (unverified, pending, verified, rejected)
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_status VARCHAR(20) NOT NULL DEFAULT 'unverified';

-- 1.2 Date de soumission des documents
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_submitted_at TIMESTAMPTZ;

-- 1.3 Date de vérification
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_verified_at TIMESTAMPTZ;

-- 1.4 Admin qui a validé (audit trail)
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_verified_by VARCHAR(36);

-- 1.5 Raison du rejet (si rejeté)
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_rejection_reason TEXT;

-- 1.6 Nombre de tentatives de soumission
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_submissions_count INT NOT NULL DEFAULT 0;

-- 1.7 Date de dernière soumission
ALTER TABLE shops
    ADD COLUMN IF NOT EXISTS kyc_last_submission_at TIMESTAMPTZ;

-- ============================================================
-- PARTIE 2 : CONTRAINTES ET INDEX
-- ============================================================

-- 2.1 Contrainte CHECK sur kyc_status
ALTER TABLE shops
    ADD CONSTRAINT shops_kyc_status_check 
        CHECK (kyc_status IN ('unverified', 'pending', 'verified', 'rejected'));

-- 2.2 Index pour performance (recherche par statut KYC)
CREATE INDEX IF NOT EXISTS idx_shops_kyc_status ON shops(kyc_status);
CREATE INDEX IF NOT EXISTS idx_shops_kyc_verified_at ON shops(kyc_verified_at DESC);
CREATE INDEX IF NOT EXISTS idx_shops_kyc_submitted_at ON shops(kyc_submitted_at DESC);

-- 2.3 Index composite pour dashboard admin
CREATE INDEX IF NOT EXISTS idx_shops_kyc_dashboard 
    ON shops(kyc_status, created_at DESC);

-- ============================================================
-- PARTIE 3 : TABLE SHOP_KYC_DOCUMENTS (documents individuels)
-- ============================================================

CREATE TABLE IF NOT EXISTS shop_kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Type de document
    document_type VARCHAR(30) NOT NULL,
    
    -- Métadonnées fichier
    file_path VARCHAR(500) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT NOT NULL CHECK (file_size_bytes > 0),
    mime_type VARCHAR(100) NOT NULL,
    
    -- Statut du document
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    
    -- Audit
    reviewed_by VARCHAR(36),
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Contraintes
    CONSTRAINT shop_kyc_documents_type_check 
        CHECK (document_type IN (
            'identity_card',      -- CNI
            'passport',           -- Passeport
            'business_registry',  -- Registre de commerce
            'tax_certificate',    -- Attestation fiscale
            'bank_statement',     -- Relevé bancaire
            'other'               -- Autre
        ))
);

-- Index pour performance
CREATE INDEX IF NOT EXISTS idx_kyc_docs_shop ON shop_kyc_documents(shop_id);
CREATE INDEX IF NOT EXISTS idx_kyc_docs_status ON shop_kyc_documents(status);
CREATE INDEX IF NOT EXISTS idx_kyc_docs_type ON shop_kyc_documents(document_type);

-- Contrainte unique : un seul document actif par type et par shop
CREATE UNIQUE INDEX IF NOT EXISTS idx_kyc_docs_unique_active
    ON shop_kyc_documents(shop_id, document_type)
    WHERE status IN ('pending', 'approved');

-- ============================================================
-- PARTIE 4 : TRIGGER AUTO-UPDATE updated_at
-- ============================================================

CREATE OR REPLACE FUNCTION update_shop_kyc_documents_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_shop_kyc_docs_updated_at ON shop_kyc_documents;
CREATE TRIGGER trigger_shop_kyc_docs_updated_at
    BEFORE UPDATE ON shop_kyc_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_shop_kyc_documents_updated_at();

-- ============================================================
-- PARTIE 5 : VUES POUR MONITORING
-- ============================================================

-- 5.1 Vue : Statistiques KYC globales
CREATE OR REPLACE VIEW v_shops_kyc_stats AS
SELECT 
    kyc_status,
    COUNT(*) as count,
    ROUND(COUNT(*)::numeric / NULLIF((SELECT COUNT(*) FROM shops), 0) * 100, 2) as percentage
FROM shops
GROUP BY kyc_status
ORDER BY count DESC;

-- 5.2 Vue : Shops en attente de vérification (dashboard admin)
CREATE OR REPLACE VIEW v_shops_kyc_pending AS
SELECT 
    s.id,
    s.name,
    s.slug,
    s.owner_id,
    s.email,
    s.kyc_status,
    s.kyc_submitted_at,
    s.kyc_submissions_count,
    s.created_at,
    EXTRACT(DAY FROM NOW() - s.kyc_submitted_at) as days_waiting,
    COUNT(d.id) as documents_count
FROM shops s
LEFT JOIN shop_kyc_documents d ON d.shop_id = s.id AND d.status = 'pending'
WHERE s.kyc_status = 'pending'
GROUP BY s.id, s.name, s.slug, s.owner_id, s.email, 
         s.kyc_status, s.kyc_submitted_at, s.kyc_submissions_count, s.created_at
ORDER BY s.kyc_submitted_at ASC;

-- 5.3 Vue : Shops vérifiés récemment
CREATE OR REPLACE VIEW v_shops_kyc_recently_verified AS
SELECT 
    s.id,
    s.name,
    s.slug,
    s.kyc_verified_at,
    s.kyc_verified_by,
    u.email as verified_by_email,
    EXTRACT(DAY FROM NOW() - s.kyc_verified_at) as days_since_verification
FROM shops s
LEFT JOIN users u ON u.id = s.kyc_verified_by
WHERE s.kyc_status = 'verified'
ORDER BY s.kyc_verified_at DESC
LIMIT 100;

-- 5.4 Vue : Documents KYC par shop
CREATE OR REPLACE VIEW v_shops_kyc_documents_summary AS
SELECT 
    s.id as shop_id,
    s.name as shop_name,
    s.slug,
    s.kyc_status,
    COUNT(d.id) as total_documents,
    COUNT(CASE WHEN d.status = 'approved' THEN 1 END) as approved_count,
    COUNT(CASE WHEN d.status = 'pending' THEN 1 END) as pending_count,
    COUNT(CASE WHEN d.status = 'rejected' THEN 1 END) as rejected_count,
    ARRAY_AGG(DISTINCT d.document_type) FILTER (WHERE d.status = 'approved') as approved_types
FROM shops s
LEFT JOIN shop_kyc_documents d ON d.shop_id = s.id
GROUP BY s.id, s.name, s.slug, s.kyc_status;

-- ============================================================
-- PARTIE 6 : FONCTIONS UTILITAIRES
-- ============================================================

-- 6.1 Fonction pour vérifier si un shop peut faire des retraits
CREATE OR REPLACE FUNCTION shop_can_withdraw(shop_id_param UUID)
RETURNS BOOLEAN AS $$
DECLARE
    can_withdraw BOOLEAN;
BEGIN
    SELECT (kyc_status = 'verified') INTO can_withdraw
    FROM shops
    WHERE id = shop_id_param;
    
    RETURN COALESCE(can_withdraw, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6.2 Fonction pour compter les shops par statut KYC
CREATE OR REPLACE FUNCTION count_shops_by_kyc_status()
RETURNS TABLE(status VARCHAR(20), count BIGINT) AS $$
BEGIN
    RETURN QUERY
    SELECT kyc_status, COUNT(*)::BIGINT
    FROM shops
    GROUP BY kyc_status
    ORDER BY count DESC;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- PARTIE 7 : MISE À JOUR DES DONNÉES EXISTANTES
-- ============================================================

-- 7.1 Tous les shops existants sont considérés "unverified"
-- (valeur par défaut déjà appliquée)
UPDATE shops SET kyc_status = 'unverified' WHERE kyc_status IS NULL;

-- 7.2 Les shops actifs avec paiements peuvent être considérés "trusted"
-- (optionnel, à activer manuellement si souhaité)
-- UPDATE shops 
-- SET kyc_status = 'verified',
--     kyc_verified_at = NOW(),
--     kyc_verified_by = 'system-migration-019'
-- WHERE is_active = true 
--   AND created_at < NOW() - INTERVAL '6 months'
--   AND id IN (SELECT DISTINCT shop_id FROM payments WHERE status = 'success');

-- ============================================================
-- PARTIE 8 : DOCUMENTATION
-- ============================================================

COMMENT ON COLUMN shops.kyc_status IS 
    'Statut KYC du marchand : unverified (initial), pending (documents soumis), verified (approuvé), rejected (rejeté)';

COMMENT ON COLUMN shops.kyc_submitted_at IS 
    'Date de la dernière soumission de documents KYC';

COMMENT ON COLUMN shops.kyc_verified_at IS 
    'Date de vérification par un admin';

COMMENT ON COLUMN shops.kyc_verified_by IS 
    'ID de l''admin qui a vérifié (audit trail)';

COMMENT ON TABLE shop_kyc_documents IS 
    'Documents KYC soumis par les marchands (CNI, registre commerce, etc.)';

-- ============================================================
-- PARTIE 9 : VÉRIFICATION
-- ============================================================

DO $$
DECLARE
    shops_count INT;
    pending_count INT;
    verified_count INT;
    unverified_count INT;
BEGIN
    SELECT COUNT(*) INTO shops_count FROM shops;
    SELECT COUNT(*) INTO pending_count FROM shops WHERE kyc_status = 'pending';
    SELECT COUNT(*) INTO verified_count FROM shops WHERE kyc_status = 'verified';
    SELECT COUNT(*) INTO unverified_count FROM shops WHERE kyc_status = 'unverified';
    
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '✅ Migration 019 terminée avec succès';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '📊 Statistiques KYC :';
    RAISE NOTICE '   - Total shops: %', shops_count;
    RAISE NOTICE '   - Unverified: %', unverified_count;
    RAISE NOTICE '   - Pending: %', pending_count;
    RAISE NOTICE '   - Verified: %', verified_count;
    RAISE NOTICE '═══════════════════════════════════════════════════════';
    RAISE NOTICE '🎯 Prochaines étapes :';
    RAISE NOTICE '   1. Mettre à jour entity/shop.go';
    RAISE NOTICE '   2. Mettre à jour shop_repository.go';
    RAISE NOTICE '   3. Créer usecases KYC';
    RAISE NOTICE '   4. Créer handlers KYC';
    RAISE NOTICE '   5. Modifier CreateWithdrawalUsecase';
    RAISE NOTICE '═══════════════════════════════════════════════════════';
END $$;