-- Migration 017 : Commissions sur échéances de crédit
-- Date: 2026-07-01
-- Description: Ajoute le tracking des commissions pour le scheduler credit
-- Taux par défaut : 0.5% (50 bps) par échéance payée

-- ============================================================
-- AJOUT DE COLONNES À LA TABLE credit_installments
-- ============================================================

ALTER TABLE credit_installments
    ADD COLUMN IF NOT EXISTS commission_status TEXT DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS commission_collected_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS commission_batch_id UUID,
    ADD COLUMN IF NOT EXISTS commission_cents BIGINT DEFAULT 0;

-- ============================================================
-- INDEX pour les requêtes du scheduler
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_credit_installments_commission_status 
    ON credit_installments(commission_status) 
    WHERE status = 'paid';

CREATE INDEX IF NOT EXISTS idx_credit_installments_paid_at 
    ON credit_installments(paid_at DESC) 
    WHERE status = 'paid';

-- ============================================================
-- TAUX PAR DÉFAUT pour le crédit (0.5% = 50 bps)
-- ============================================================

INSERT INTO commission_rates (shop_id, transaction_type, rate_bps, min_commission_cents, max_commission_cents, created_by)
SELECT 
    s.id as shop_id,
    'credit' as transaction_type,
    50 as rate_bps, -- 0.5%
    0 as min_commission_cents,
    2000000 as max_commission_cents, -- 20 000 FCFA max par échéance
    'system' as created_by
FROM shops s
ON CONFLICT (shop_id, transaction_type) DO NOTHING;

-- ============================================================
-- MESSAGE DE SUCCÈS
-- ============================================================
DO $$
BEGIN
    RAISE NOTICE '%', '✅ Migration 017 appliquée avec succès';
    RAISE NOTICE '%', '   - Colonnes commission ajoutées à credit_installments';
    RAISE NOTICE '%', '   - Taux par défaut inséré (credit: 0.5%)';
END $$;