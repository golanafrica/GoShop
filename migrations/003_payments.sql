-- migrations/003_payments.sql
-- ============================================
-- PHASE 2 : Système de paiement
-- ============================================

-- 1. Table payments (paiements)
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    
    -- Méthode de paiement
    provider VARCHAR(50) NOT NULL,  -- 'orange_money', 'moov_money', 'wave', 'cash'
    provider_ref VARCHAR(255),       -- Référence côté provider (unique)
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency VARCHAR(3) DEFAULT 'XOF' CHECK (currency IN ('XOF', 'XAF')),
    
    -- Informations client
    customer_phone VARCHAR(20),
    customer_email VARCHAR(255),
    description TEXT,
    
    -- Statut (machine à états)
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- pending, processing, success, failed, refunded, cancelled, expired
    
    -- Métadonnées (JSON flexible pour chaque provider)
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Timestamps
    initiated_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Contraintes
    CONSTRAINT payments_provider_check CHECK (provider IN (
        'orange_money', 'moov_money', 'wave', 'cash', 'mock'
    )),
    CONSTRAINT payments_status_check CHECK (status IN (
        'pending', 'processing', 'success', 'failed', 
        'refunded', 'cancelled', 'expired'
    )),
    CONSTRAINT payments_provider_ref_unique UNIQUE (provider, provider_ref)
);

-- 2. Table payment_webhooks (audit trail)
CREATE TABLE IF NOT EXISTS payment_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    external_id VARCHAR(255),  -- ID externe du webhook
    
    -- Contenu
    payload JSONB NOT NULL,
    signature VARCHAR(255),
    signature_validated BOOLEAN DEFAULT false,
    
    -- Traitement
    processed BOOLEAN DEFAULT false,
    processing_error TEXT,
    
    -- Timestamps
    received_at TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

-- 3. Index pour performance
CREATE INDEX IF NOT EXISTS idx_payments_shop ON payments(shop_id);
CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_provider ON payments(provider);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_provider_ref ON payments(provider, provider_ref);

CREATE INDEX IF NOT EXISTS idx_payment_webhooks_payment ON payment_webhooks(payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_webhooks_provider ON payment_webhooks(provider);
CREATE INDEX IF NOT EXISTS idx_payment_webhooks_external_id ON payment_webhooks(external_id);

-- 4. Trigger updated_at
CREATE OR REPLACE FUNCTION update_payments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_payments_updated_at ON payments;
CREATE TRIGGER trigger_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION update_payments_updated_at();

-- 5. Données de test (paiement de démo)
DO $$
DECLARE
    demo_shop_id UUID := 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    demo_order_id UUID;
BEGIN
    -- Récupérer une commande existante du shop de démo
    SELECT id INTO demo_order_id 
    FROM orders 
    WHERE shop_id = demo_shop_id 
    LIMIT 1;
    
    IF demo_order_id IS NOT NULL THEN
        -- Créer un paiement de démo (mock)
        INSERT INTO payments (
            shop_id, order_id, provider, provider_ref, 
            amount_cents, status, customer_phone,
            initiated_at, completed_at
        )
        VALUES (
            demo_shop_id, demo_order_id, 'mock', 'MOCK-DEMO-001',
            15000, 'success', '+22670000000',
            NOW(), NOW()
        )
        ON CONFLICT DO NOTHING;
    END IF;
END $$;