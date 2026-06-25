-- Migration 005 : Ajouter Yenga Pay à la contrainte CHECK des providers
-- Date: 2026-06-24

-- Supprimer l'ancienne contrainte
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_provider_check;

-- Recréer la contrainte avec yenga_pay inclus
ALTER TABLE payments ADD CONSTRAINT payments_provider_check 
    CHECK (provider IN (
        'orange_money', 
        'moov_money', 
        'wave', 
        'yenga_pay',
        'cash', 
        'mock'
    ));

-- Vérifier que la contrainte a été appliquée
SELECT conname, pg_get_constraintdef(oid) 
FROM pg_constraint 
WHERE conname = 'payments_provider_check';