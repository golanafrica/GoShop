-- Migration 008 : Table des retraits (cash-out)
-- Date: 2026-06-26

CREATE TABLE IF NOT EXISTS withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL DEFAULT 'yenga_pay',
    provider_ref VARCHAR(255),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'XOF',
    fees BIGINT DEFAULT 0,
    net_amount BIGINT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'processing', 'success', 'failed', 'cancelled')),
    payment_method VARCHAR(50) NOT NULL
        CHECK (payment_method IN (
            'ORANGE_MONEY', 'MOOV_MONEY', 'TELECEL_MONEY', 
            'CORIS_MONEY', 'SANK_MONEY', 'MTN'
        )),
    destination_number VARCHAR(20) NOT NULL,
    destination_name VARCHAR(255),
    destination_email VARCHAR(255),
    description TEXT,
    error_message TEXT,
    operator_transaction_id VARCHAR(255),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index pour recherche rapide
CREATE INDEX idx_withdrawals_shop_id ON withdrawals(shop_id);
CREATE INDEX idx_withdrawals_status ON withdrawals(status);
CREATE INDEX idx_withdrawals_provider_ref ON withdrawals(provider_ref);
CREATE INDEX idx_withdrawals_created_at ON withdrawals(created_at DESC);

COMMENT ON TABLE withdrawals IS 'Retraits (cash-out) vers Mobile Money via Yenga Pay';
COMMENT ON COLUMN withdrawals.provider_ref IS 'ID de retrait retourné par Yenga Pay (ex: YPCO20250924.1546.40403.2921)';
COMMENT ON COLUMN withdrawals.operator_transaction_id IS 'ID de transaction côté opérateur mobile';