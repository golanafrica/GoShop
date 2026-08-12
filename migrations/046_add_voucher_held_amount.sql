-- ============================================================
-- Migration 046 : held_amount_cents sur tontine_vouchers
-- Phase 5 — libération exacte du held au redeem
-- ============================================================

ALTER TABLE tontine_vouchers
  ADD COLUMN IF NOT EXISTS held_amount_cents BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tontine_vouchers.held_amount_cents IS
  'Montant net gelé sur merchant_wallets.held_cents au moment de la génération du voucher. Libéré au redeem.';

DO $$
BEGIN
  RAISE NOTICE 'Migration 046 appliquée : tontine_vouchers.held_amount_cents';
END $$;