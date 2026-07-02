-- Migration 015 : Commissions sur paiements en ligne (Orange/Moov/Yenga)
-- Permet de tracer la collecte automatique des commissions

-- ============================================================
-- AJOUT DE COLONNES À LA TABLE payments
-- ============================================================

-- Commission calculée
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS commission_rate_bps INTEGER DEFAULT 250, -- 2.5% par défaut
    ADD COLUMN IF NOT EXISTS commission_cents BIGINT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS commission_status TEXT DEFAULT 'pending', -- pending, collected, failed
    ADD COLUMN IF NOT EXISTS commission_collected_at TIMESTAMPTZ;

-- Index pour les requêtes du scheduler
CREATE INDEX IF NOT EXISTS idx_payments_commission_status 
    ON payments(commission_status) 
    WHERE status = 'completed';

CREATE INDEX IF NOT EXISTS idx_payments_completed_at 
    ON payments(completed_at DESC) 
    WHERE status = 'completed';

-- ============================================================
-- TABLE : commission_rates (taux configurables par boutique)
-- ============================================================
CREATE TABLE IF NOT EXISTS commission_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Type de transaction
    transaction_type TEXT NOT NULL, -- online_payment, cod, tontine_solo, tontine_group, credit
    
    -- Taux en basis points (100 bps = 1%)
    rate_bps INTEGER NOT NULL DEFAULT 250,
    
    -- Limites
    min_commission_cents BIGINT DEFAULT 0,
    max_commission_cents BIGINT DEFAULT 10000000, -- 100 000 FCFA max
    
    -- Statut
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT,
    
    UNIQUE(shop_id, transaction_type)
);

-- Index
CREATE INDEX IF NOT EXISTS idx_commission_rates_shop 
    ON commission_rates(shop_id);

CREATE INDEX IF NOT EXISTS idx_commission_rates_active 
    ON commission_rates(is_active);

-- ============================================================
-- TAUX PAR DÉFAUT (insérés pour toutes les boutiques existantes)
-- ============================================================

-- Insérer les taux par défaut pour toutes les boutiques
INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    id as shop_id,
    'online_payment' as transaction_type,
    250 as rate_bps, -- 2.5%
    0 as min_commission_cents,
    10000000 as max_commission_cents, -- 100 000 FCFA max
    'system' as created_by
FROM shops
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    id as shop_id,
    'cod' as transaction_type,
    250 as rate_bps, -- 2.5%
    0 as min_commission_cents,
    10000000 as max_commission_cents,
    'system' as created_by
FROM shops
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    id as shop_id,
    'tontine_solo' as transaction_type,
    200 as rate_bps, -- 2%
    0 as min_commission_cents,
    5000000 as max_commission_cents, -- 50 000 FCFA max
    'system' as created_by
FROM shops
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    id as shop_id,
    'tontine_group' as transaction_type,
    150 as rate_bps, -- 1.5%
    0 as min_commission_cents,
    5000000 as max_commission_cents,
    'system' as created_by
FROM shops
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    id as shop_id,
    'credit' as transaction_type,
    50 as rate_bps, -- 0.5%
    0 as min_commission_cents,
    2000000 as max_commission_cents, -- 20 000 FCFA max
    'system' as created_by
FROM shops
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

-- ============================================================
-- VUE : statistiques des commissions par type
-- ============================================================
CREATE OR REPLACE VIEW v_commission_by_type AS
SELECT 
    cr.transaction_type,
    COUNT(*) as total_transactions,
    SUM(CASE WHEN p.commission_status = 'collected' THEN 1 ELSE 0 END) as collected_count,
    SUM(CASE WHEN p.commission_status = 'pending' THEN 1 ELSE 0 END) as pending_count,
    SUM(CASE WHEN p.commission_status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(p.commission_cents) as total_commission_cents,
    AVG(cr.rate_bps) as avg_rate_bps
FROM payments p
JOIN commission_rates cr ON cr.shop_id = p.shop_id 
    AND cr.transaction_type = 'online_payment'
WHERE p.status = 'completed'
GROUP BY cr.transaction_type;

-- ============================================================
-- MESSAGE DE SUCCÈS
-- ============================================================
DO $$
BEGIN
    RAISE NOTICE '✅ Migration 015 appliquée avec succès';
    RAISE NOTICE '   - Colonnes commission ajoutées à payments';
    RAISE NOTICE '   - Table commission_rates créée';
    RAISE NOTICE '   - Taux par défaut insérés pour toutes les boutiques';
    RAISE NOTICE '   - Vue v_commission_by_type créée';
END $$;