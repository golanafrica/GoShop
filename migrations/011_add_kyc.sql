-- Migration 011 : KYC (Know Your Customer) pour les clients
-- Date: 2026-06-28
-- Description: Système de vérification d'identité pour participer aux tontines
-- Dépend de : 001 (customers, shops), 002 (shops)

-- ============================================================
-- 1. Ajout du niveau KYC sur les customers
-- ============================================================
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS kyc_level VARCHAR(20) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS kyc_validated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS kyc_validated_by UUID;

-- Contrainte sur les valeurs autorisées (idempotente)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'customers_kyc_level_check'
    ) THEN
        ALTER TABLE customers
        ADD CONSTRAINT customers_kyc_level_check
        CHECK (kyc_level IN ('none', 'pending', 'verified', 'rejected'));
    END IF;
END $$;

COMMENT ON COLUMN customers.kyc_level IS 'Niveau KYC : none (non vérifié), pending (en attente), verified (validé), rejected (rejeté)';
COMMENT ON COLUMN customers.kyc_validated_at IS 'Date de validation KYC';
COMMENT ON COLUMN customers.kyc_validated_by IS 'ID du marchand (shop owner) qui a validé';

CREATE INDEX IF NOT EXISTS idx_customers_kyc ON customers(kyc_level);

-- ============================================================
-- 2. Documents KYC uploadés par les clients
-- ============================================================
CREATE TABLE IF NOT EXISTS customer_kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,

    document_type VARCHAR(20) NOT NULL,  -- 'cni', 'passport', 'other'
    file_path VARCHAR(500) NOT NULL,
    file_size_bytes BIGINT NOT NULL CHECK (file_size_bytes > 0),
    mime_type VARCHAR(100) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 'pending', 'approved', 'rejected'

    reviewed_by UUID,                    -- user_id du marchand
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kyc_documents_type_check
        CHECK (document_type IN ('cni', 'passport', 'other')),
    CONSTRAINT kyc_documents_status_check
        CHECK (status IN ('pending', 'approved', 'rejected'))
);

COMMENT ON TABLE customer_kyc_documents IS 'Documents KYC uploadés par les clients pour vérification d''identité';

-- ============================================================
-- INDEX
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_kyc_documents_customer ON customer_kyc_documents(customer_id);
CREATE INDEX IF NOT EXISTS idx_kyc_documents_shop ON customer_kyc_documents(shop_id);
CREATE INDEX IF NOT EXISTS idx_kyc_documents_status ON customer_kyc_documents(status);