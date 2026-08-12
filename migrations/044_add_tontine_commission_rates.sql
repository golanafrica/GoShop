-- migrations/044_add_tontine_commission_rates.sql
-- S'assure que les taux de commission tontine sont correctement définis pour toutes les boutiques

INSERT INTO commission_rates (
    shop_id,
    transaction_type,
    rate_bps,
    min_commission_cents,
    max_commission_cents,
    is_active,
    created_by
)
SELECT 
    s.id AS shop_id,
    t.transaction_type,
    t.rate_bps,
    0 AS min_commission_cents,
    5000000 AS max_commission_cents,
    true AS is_active,
    'system' AS created_by
FROM shops s
CROSS JOIN (
    VALUES 
        ('tontine_commercial', 200),
        ('tontine_corporate', 150),
        ('tontine_family', 150)
) AS t(transaction_type, rate_bps)
ON CONFLICT (shop_id, transaction_type) 
DO UPDATE SET 
    rate_bps = EXCLUDED.rate_bps,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

DO $$
BEGIN
    RAISE NOTICE 'Migration 044 appliquée avec succès :';
    RAISE NOTICE '  - tontine_commercial : 2.0%% (200 bps)';
    RAISE NOTICE '  - tontine_corporate  : 1.5%% (150 bps)';
    RAISE NOTICE '  - tontine_family     : 1.5%% (150 bps)';
END $$;