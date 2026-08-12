-- migrations/042_add_tontine_provider_intent_id.sql
-- Ajoute le champ pour stocker l'ID d'intention de paiement YengaPay (nécessaire pour le CheckStatus / Sync)

ALTER TABLE tontine_payments
  ADD COLUMN IF NOT EXISTS provider_intent_id TEXT;

CREATE INDEX IF NOT EXISTS idx_tontine_payments_provider_intent
  ON tontine_payments (provider_intent_id)
  WHERE provider_intent_id IS NOT NULL;