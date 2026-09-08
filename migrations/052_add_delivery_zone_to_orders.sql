-- Migration 052 : Ajout de delivery_zone_id aux tables existantes
-- Date: 2026-09-07
-- Description: Lie les commandes, tontines, COD et plans de tranches aux zones de livraison

-- ============================================
-- 1. Ajout aux commandes (orders)
-- ============================================

ALTER TABLE orders 
ADD COLUMN IF NOT EXISTS delivery_zone_id UUID REFERENCES delivery_zones(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_orders_delivery_zone ON orders(delivery_zone_id);

COMMENT ON COLUMN orders.delivery_zone_id IS 'Zone de livraison pour calculer les délais';

-- ============================================
-- 2. Ajout aux groupes de tontine (tontine_groups)
-- ============================================

ALTER TABLE tontine_groups 
ADD COLUMN IF NOT EXISTS delivery_zone_id UUID REFERENCES delivery_zones(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tontine_groups_delivery_zone ON tontine_groups(delivery_zone_id);

COMMENT ON COLUMN tontine_groups.delivery_zone_id IS 'Zone de livraison pour les lots de tontine';

-- ============================================
-- 3. Ajout aux preuves COD (cod_proofs)
-- ============================================

ALTER TABLE cod_proofs 
ADD COLUMN IF NOT EXISTS delivery_zone_id UUID REFERENCES delivery_zones(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_cod_proofs_delivery_zone ON cod_proofs(delivery_zone_id);

COMMENT ON COLUMN cod_proofs.delivery_zone_id IS 'Zone de livraison pour calculer le délai de confirmation COD';

-- ============================================
-- 4. Ajout aux plans de tranches (installment_plans)
-- ============================================

ALTER TABLE installment_plans 
ADD COLUMN IF NOT EXISTS delivery_zone_id UUID REFERENCES delivery_zones(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_installment_plans_delivery_zone ON installment_plans(delivery_zone_id);

COMMENT ON COLUMN installment_plans.delivery_zone_id IS 'Zone de livraison pour calculer le délai de sécurité avant auto-release';

-- ============================================
-- 5. Résumé
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '✅ Migration 052 terminée avec succès';
    RAISE NOTICE '   - delivery_zone_id ajouté à orders';
    RAISE NOTICE '   - delivery_zone_id ajouté à tontine_groups';
    RAISE NOTICE '   - delivery_zone_id ajouté à cod_proofs';
    RAISE NOTICE '   - delivery_zone_id ajouté à installment_plans';
    RAISE NOTICE '   - Index créés pour performance';
END $$;