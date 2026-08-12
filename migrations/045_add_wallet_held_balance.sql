-- ============================================================
-- Migration 045 : held_cents sur merchant_wallets
-- Phase 2 — Fondation Wallet (Held vs Available)
-- ============================================================
-- balance_cents  = solde total crédité (ledger)
-- held_cents     = montant gelé (tontine cycle, litige, etc.)
-- available      = balance_cents - held_cents  (calculé en app)
--
-- Invariant app : held_cents >= 0
-- PAS de CHECK held <= balance en DB : balance_cents peut être
-- négatif (commission debit AllowNegative / freeze).
-- ============================================================

ALTER TABLE merchant_wallets
  ADD COLUMN IF NOT EXISTS held_cents BIGINT NOT NULL DEFAULT 0;

-- Contrainte non-négative sur held uniquement
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'merchant_wallets_held_cents_non_negative'
  ) THEN
    ALTER TABLE merchant_wallets
      ADD CONSTRAINT merchant_wallets_held_cents_non_negative
      CHECK (held_cents >= 0);
  END IF;
END $$;

-- Si une ancienne version de 045 a créé le CHECK trop strict, le retirer
ALTER TABLE merchant_wallets
  DROP CONSTRAINT IF EXISTS merchant_wallets_held_lte_balance;

COMMENT ON COLUMN merchant_wallets.held_cents IS
  'Montant gelé (tontine, litige…). Available = balance_cents - held_cents (app). Pas de CHECK held<=balance : balance peut être négatif.';

DO $$
BEGIN
  RAISE NOTICE 'Migration 045 appliquée : merchant_wallets.held_cents (DEFAULT 0), CHECK held >= 0 uniquement';
END $$;