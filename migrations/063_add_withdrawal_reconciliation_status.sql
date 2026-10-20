-- Migration 063: Ajout du statut reconciliation_required et mise à jour de la contrainte CHECK
BEGIN;

-- 1. Mettre à jour les retraits existants qui seraient bloqués (sécurité)
-- (Optionnel, mais recommandé si des données sont déjà dans cet état)
-- UPDATE withdrawals SET status = 'reconciliation_required' WHERE status = 'pending' AND ... (condition de blocage);

-- 2. Mettre à jour la contrainte CHECK pour inclure le nouveau statut
-- On suppose que la contrainte s'appelle 'withdrawals_status_check' (à vérifier dans ta DB)
ALTER TABLE withdrawals DROP CONSTRAINT IF EXISTS withdrawals_status_check;

ALTER TABLE withdrawals ADD CONSTRAINT withdrawals_status_check 
CHECK (status IN ('pending', 'processing', 'success', 'failed', 'compensation_pending', 'reconciliation_required'));

COMMIT;