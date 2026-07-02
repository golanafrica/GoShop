-- Migration 014 : Audit des batches de commissions automatiques
-- Permet de tracer chaque exécution du scheduler nocturne

-- ============================================================
-- TABLE : commission_batches (lot d'exécution)
-- ============================================================
CREATE TABLE IF NOT EXISTS commission_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Métadonnées d'exécution
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    
    -- Statistiques
    total_proofs INTEGER NOT NULL DEFAULT 0,
    successful_collections INTEGER NOT NULL DEFAULT 0,
    failed_collections INTEGER NOT NULL DEFAULT 0,
    skipped_proofs INTEGER NOT NULL DEFAULT 0,
    
    -- Montants (en centimes)
    total_commission_cents BIGINT NOT NULL DEFAULT 0,
    collected_commission_cents BIGINT NOT NULL DEFAULT 0,
    failed_commission_cents BIGINT NOT NULL DEFAULT 0,
    
    -- Statut global
    status TEXT NOT NULL DEFAULT 'running', -- running, completed, failed
    
    -- Détails d'erreur (si échec global)
    error_message TEXT,
    
    -- Contexte d'exécution
    triggered_by TEXT NOT NULL DEFAULT 'scheduler', -- scheduler, manual, admin
    executed_by TEXT, -- ID du user si manuel
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- TABLE : commission_batch_items (résultat par preuve)
-- ============================================================
CREATE TABLE IF NOT EXISTS commission_batch_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES commission_batches(id) ON DELETE CASCADE,
    
    -- Référence à la preuve COD
    cod_proof_id UUID NOT NULL,
    order_id UUID NOT NULL,
    shop_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    
    -- Résultat de la collecte
    status TEXT NOT NULL, -- success, failed, skipped
    commission_cents BIGINT NOT NULL,
    
    -- Détails
    error_message TEXT,
    wallet_balance_before BIGINT,
    wallet_balance_after BIGINT,
    account_frozen BOOLEAN DEFAULT false,
    
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- INDEX (optimisation des requêtes)
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_commission_batches_status 
    ON commission_batches(status);

CREATE INDEX IF NOT EXISTS idx_commission_batches_started_at 
    ON commission_batches(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_commission_batch_items_batch_id 
    ON commission_batch_items(batch_id);

CREATE INDEX IF NOT EXISTS idx_commission_batch_items_shop 
    ON commission_batch_items(shop_id);

CREATE INDEX IF NOT EXISTS idx_commission_batch_items_status 
    ON commission_batch_items(status);

-- ============================================================
-- VUE : statistiques quotidiennes (pour dashboard)
-- ============================================================
CREATE OR REPLACE VIEW v_commission_daily_stats AS
SELECT 
    DATE(started_at) as execution_date,
    COUNT(*) as total_batches,
    SUM(successful_collections) as total_successful,
    SUM(failed_collections) as total_failed,
    SUM(skipped_proofs) as total_skipped,
    SUM(collected_commission_cents) as total_collected_cents,
    SUM(failed_commission_cents) as total_failed_cents
FROM commission_batches
WHERE status = 'completed'
GROUP BY DATE(started_at)
ORDER BY execution_date DESC;

-- ============================================================
-- MESSAGE DE SUCCÈS
-- ============================================================
DO $$
BEGIN
    RAISE NOTICE '✅ Migration 014 appliquée avec succès';
    RAISE NOTICE '   - Table commission_batches créée';
    RAISE NOTICE '   - Table commission_batch_items créée';
    RAISE NOTICE '   - 5 index créés';
    RAISE NOTICE '   - Vue v_commission_daily_stats créée';
END $$;