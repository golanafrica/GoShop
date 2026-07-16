-- Migration 016 : Commissions sur paiements tontine
-- Permet de tracer la collecte automatique des commissions

-- ============================================================
-- AJOUT DE COLONNES À LA TABLE tontine_payments
-- ============================================================

ALTER TABLE tontine_payments
    ADD COLUMN IF NOT EXISTS commission_status TEXT DEFAULT 'pending', -- pending, collected, failed
    ADD COLUMN IF NOT EXISTS commission_collected_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS commission_batch_id UUID;

-- Index pour les requêtes du scheduler
CREATE INDEX IF NOT EXISTS idx_tontine_payments_commission_status 
    ON tontine_payments(commission_status) 
    WHERE status = 'DONE';

CREATE INDEX IF NOT EXISTS idx_tontine_payments_paid_at 
    ON tontine_payments(paid_at DESC) 
    WHERE status = 'DONE';

-- ============================================================
-- TAUX PAR DÉFAUT POUR LES TONTINES
-- ============================================================

-- Insérer les taux par défaut pour toutes les boutiques
INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    s.id as shop_id,
    'tontine_commercial' as transaction_type,
    200 as rate_bps, -- 2%%
    0 as min_commission_cents,
    5000000 as max_commission_cents, -- 50 000 FCFA max
    'system' as created_by
FROM shops s
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    s.id as shop_id,
    'tontine_corporate' as transaction_type,
    150 as rate_bps, -- 1.5%%
    0 as min_commission_cents,
    5000000 as max_commission_cents,
    'system' as created_by
FROM shops s
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    s.id as shop_id,
    'tontine_family' as transaction_type,
    150 as rate_bps, -- 1.5%%
    0 as min_commission_cents,
    5000000 as max_commission_cents,
    'system' as created_by
FROM shops s
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

-- ============================================================
-- MESSAGE DE SUCCÈS
-- ============================================================
DO $$
BEGIN
    RAISE NOTICE 'Migration 016 appliquee avec succes';
    RAISE NOTICE '   - Colonnes commission ajoutees a tontine_payments';
    RAISE NOTICE '   - Taux par defaut inseres (commercial: 2%%, corporate/family: 1.5%%)';
END $$;