-- Migration 063: Ajout du statut reconciliation_required et extension de la taille de la colonne status
BEGIN;

-- 1. Étendre la taille de la colonne status à VARCHAR(50) 
-- 'compensation_pending' fait 20 caractères, 'reconciliation_required' en fait 23.
ALTER TABLE withdrawals ALTER COLUMN status TYPE VARCHAR(50);

-- 2. Mettre à jour la contrainte CHECK pour inclure le nouveau statut
ALTER TABLE withdrawals DROP CONSTRAINT IF EXISTS withdrawals_status_check;

ALTER TABLE withdrawals ADD CONSTRAINT withdrawals_status_check 
CHECK (status IN ('pending', 'processing', 'success', 'failed', 'cancelled', 'compensation_pending', 'reconciliation_required'));

COMMIT;