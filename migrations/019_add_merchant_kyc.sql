-- ============================================================
-- Migration 019 : KYC Marchand progressif
-- Date: 2026-07-03
-- Version: v4.1.0
-- ============================================================

-- PARTIE 1 : AJOUT COLONNES KYC SUR SHOPS
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_status VARCHAR(20) NOT NULL DEFAULT 'unverified';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_submitted_at TIMESTAMPTZ;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_verified_at TIMESTAMPTZ;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_verified_by VARCHAR(36);
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_rejection_reason TEXT;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_submissions_count INT NOT NULL DEFAULT 0;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS kyc_last_submission_at TIMESTAMPTZ;

-- PARTIE 2 : CONTRAINTES ET INDEX
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'shops_kyc_status_check' AND conrelid = 'shops'::regclass) THEN
        ALTER TABLE shops DROP CONSTRAINT shops_kyc_status_check;
    END IF;
    ALTER TABLE shops ADD CONSTRAINT shops_kyc_status_check CHECK (kyc_status IN ('unverified', 'pending', 'verified', 'rejected'));
END $$;

CREATE INDEX IF NOT EXISTS idx_shops_kyc_status ON shops(kyc_status);
CREATE INDEX IF NOT EXISTS idx_shops_kyc_verified_at ON shops(kyc_verified_at DESC);
CREATE INDEX IF NOT EXISTS idx_shops_kyc_submitted_at ON shops(kyc_submitted_at DESC);
CREATE INDEX IF NOT EXISTS idx_shops_kyc_dashboard ON shops(kyc_status, created_at DESC);

-- PARTIE 3 : TABLE SHOP_KYC_DOCUMENTS
CREATE TABLE IF NOT EXISTS shop_kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    document_type VARCHAR(30) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT NOT NULL CHECK (file_size_bytes > 0),
    mime_type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by VARCHAR(36),
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_kyc_documents_type_check CHECK (document_type IN ('identity_card', 'passport', 'business_registry', 'tax_certificate', 'bank_statement', 'other'))
);

CREATE INDEX IF NOT EXISTS idx_kyc_docs_shop ON shop_kyc_documents(shop_id);
CREATE INDEX IF NOT EXISTS idx_kyc_docs_status ON shop_kyc_documents(status);
CREATE INDEX IF NOT EXISTS idx_kyc_docs_type ON shop_kyc_documents(document_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kyc_docs_unique_active ON shop_kyc_documents(shop_id, document_type) WHERE status IN ('pending', 'approved');

-- PARTIE 4 : TRIGGER
CREATE OR REPLACE FUNCTION update_shop_kyc_documents_updated_at() RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_shop_kyc_docs_updated_at ON shop_kyc_documents;
CREATE TRIGGER trigger_shop_kyc_docs_updated_at BEFORE UPDATE ON shop_kyc_documents FOR EACH ROW EXECUTE FUNCTION update_shop_kyc_documents_updated_at();

-- PARTIE 5 : VUES (CORRIGÉ : Nettoyage avant recréation)
DROP VIEW IF EXISTS v_shops_kyc_stats CASCADE;
DROP VIEW IF EXISTS v_shops_kyc_pending CASCADE;
DROP VIEW IF EXISTS v_shops_kyc_recently_verified CASCADE;
DROP VIEW IF EXISTS v_shops_kyc_documents_summary CASCADE;

CREATE OR REPLACE VIEW v_shops_kyc_stats AS
SELECT kyc_status, COUNT(*) as count, ROUND(COUNT(*)::numeric / NULLIF((SELECT COUNT(*) FROM shops), 0) * 100, 2) as percentage
FROM shops GROUP BY kyc_status ORDER BY count DESC;

CREATE OR REPLACE VIEW v_shops_kyc_pending AS
SELECT s.id, s.name, s.slug, s.owner_id, u.email, s.kyc_status, s.kyc_submitted_at, s.kyc_submissions_count, s.created_at,
       EXTRACT(DAY FROM NOW() - s.kyc_submitted_at) as days_waiting, COUNT(d.id) as documents_count
FROM shops s
LEFT JOIN users u ON u.id = s.owner_id
LEFT JOIN shop_kyc_documents d ON d.shop_id = s.id AND d.status = 'pending'
WHERE s.kyc_status = 'pending'
GROUP BY s.id, s.name, s.slug, s.owner_id, u.email, s.kyc_status, s.kyc_submitted_at, s.kyc_submissions_count, s.created_at
ORDER BY s.kyc_submitted_at ASC;

CREATE OR REPLACE VIEW v_shops_kyc_recently_verified AS
SELECT s.id, s.name, s.slug, s.kyc_verified_at, s.kyc_verified_by, u.email as verified_by_email,
       EXTRACT(DAY FROM NOW() - s.kyc_verified_at) as days_since_verification
FROM shops s LEFT JOIN users u ON u.id = s.kyc_verified_by
WHERE s.kyc_status = 'verified' ORDER BY s.kyc_verified_at DESC LIMIT 100;

CREATE OR REPLACE VIEW v_shops_kyc_documents_summary AS
SELECT s.id as shop_id, s.name as shop_name, s.slug, s.kyc_status, COUNT(d.id) as total_documents,
       COUNT(CASE WHEN d.status = 'approved' THEN 1 END) as approved_count,
       COUNT(CASE WHEN d.status = 'pending' THEN 1 END) as pending_count,
       COUNT(CASE WHEN d.status = 'rejected' THEN 1 END) as rejected_count,
       ARRAY_AGG(DISTINCT d.document_type) FILTER (WHERE d.status = 'approved') as approved_types
FROM shops s LEFT JOIN shop_kyc_documents d ON d.shop_id = s.id
GROUP BY s.id, s.name, s.slug, s.kyc_status;

-- PARTIE 6 : FONCTIONS
CREATE OR REPLACE FUNCTION shop_can_withdraw(shop_id_param UUID) RETURNS BOOLEAN AS $$
DECLARE can_withdraw BOOLEAN;
BEGIN
    SELECT (kyc_status = 'verified') INTO can_withdraw FROM shops WHERE id = shop_id_param;
    RETURN COALESCE(can_withdraw, false);
END; $$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION count_shops_by_kyc_status() RETURNS TABLE(status VARCHAR(20), count BIGINT) AS $$
BEGIN
    RETURN QUERY SELECT kyc_status, COUNT(*)::BIGINT FROM shops GROUP BY kyc_status ORDER BY count DESC;
END; $$ LANGUAGE plpgsql SECURITY DEFINER;

-- PARTIE 7 : MISE À JOUR
UPDATE shops SET kyc_status = 'unverified' WHERE kyc_status IS NULL;

-- PARTIE 8 : DOCUMENTATION
COMMENT ON COLUMN shops.kyc_status IS 'Statut KYC : unverified, pending, verified, rejected';
COMMENT ON TABLE shop_kyc_documents IS 'Documents KYC soumis par les marchands';

-- PARTIE 9 : VÉRIFICATION
DO $$
DECLARE shops_count INT; pending_count INT; verified_count INT; unverified_count INT;
BEGIN
    SELECT COUNT(*) INTO shops_count FROM shops;
    SELECT COUNT(*) INTO pending_count FROM shops WHERE kyc_status = 'pending';
    SELECT COUNT(*) INTO verified_count FROM shops WHERE kyc_status = 'verified';
    SELECT COUNT(*) INTO unverified_count FROM shops WHERE kyc_status = 'unverified';
    RAISE NOTICE '✅ Migration 019 terminée. Total: %, Unverified: %, Pending: %, Verified: %', shops_count, unverified_count, pending_count, verified_count;
END $$;