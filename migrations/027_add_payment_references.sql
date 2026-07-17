-- migrations/027_add_payment_references.sql
-- Description: Rendre les paiements polymorphes pour supporter les échéances de crédit et autres références

-- 1. Rendre order_id nullable (car un paiement peut désormais concerner autre chose qu'une commande)
ALTER TABLE payments ALTER COLUMN order_id DROP NOT NULL;

-- 2. Ajouter des colonnes de référence génériques
ALTER TABLE payments 
  ADD COLUMN IF NOT EXISTS reference_type VARCHAR(50), -- ex: 'order', 'credit_installment', 'tontine_cycle'
  ADD COLUMN IF NOT EXISTS reference_id UUID;          -- ex: l'ID de l'échéance ou de la commande

-- 3. Ajouter un index pour les recherches rapides par référence
CREATE INDEX IF NOT EXISTS idx_payments_reference ON payments(reference_type, reference_id);

-- 4. Mettre à jour les paiements existants pour qu'ils restent cohérents
UPDATE payments SET reference_type = 'order', reference_id = order_id WHERE reference_type IS NULL;

DO $$
BEGIN
  RAISE NOTICE '✅ Migration 027 appliquée : Table payments désormais polymorphe';
END $$;