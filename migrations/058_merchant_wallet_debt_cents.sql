-- Dette marchand (clawback post-retrait / solde insuffisant) + types ledger
-- debt_cents >= 0 : montant encore dû à la plateforme
-- available_withdraw ≈ max(0, balance_cents - held_cents - debt_cents)

BEGIN;

ALTER TABLE merchant_wallets
  ADD COLUMN IF NOT EXISTS debt_cents bigint NOT NULL DEFAULT 0;

ALTER TABLE merchant_wallets
  DROP CONSTRAINT IF EXISTS merchant_wallets_debt_cents_non_negative;

ALTER TABLE merchant_wallets
  ADD CONSTRAINT merchant_wallets_debt_cents_non_negative
  CHECK (debt_cents >= 0);

COMMENT ON COLUMN merchant_wallets.debt_cents IS
  'Montant dû plateforme (clawback partiel). Réduit automatiquement sur prochains crédits (CreditWithDebtSweep).';

-- Types ledger pour audit (si contrainte type_check existe)
ALTER TABLE wallet_transactions DROP CONSTRAINT IF EXISTS wallet_transactions_type_check;
ALTER TABLE wallet_transactions ADD CONSTRAINT wallet_transactions_type_check
  CHECK (transaction_type::text = ANY (ARRAY[
    'sale_credit'::character varying,
    'sale_cod'::character varying,
    'sale_tontine'::character varying,
    'sale_credit_plan'::character varying,
    'commission_debit'::character varying,
    'payout'::character varying,
    'deposit'::character varying,
    'refund'::character varying,
    'freeze_penalty'::character varying,
    'unfreeze_deposit'::character varying,
    'clawback'::character varying,
    'debt_add'::character varying,
    'debt_sweep'::character varying
  ]::text[]));

COMMIT;