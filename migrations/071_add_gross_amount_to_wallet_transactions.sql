-- migration 071: add gross_amount_cents to wallet_transactions
-- Cette colonne stocke le montant brut demandé avant tout debt sweep.
-- Elle est nullable pour accommoder les transactions historiques (legacy).
ALTER TABLE wallet_transactions 
ADD COLUMN IF NOT EXISTS gross_amount_cents BIGINT;