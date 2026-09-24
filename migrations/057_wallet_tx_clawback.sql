-- 057 : autoriser transaction_type = 'clawback' (litige customer_wins post-release)
ALTER TABLE wallet_transactions
  DROP CONSTRAINT IF EXISTS wallet_transactions_type_check;

ALTER TABLE wallet_transactions
  ADD CONSTRAINT wallet_transactions_type_check
  CHECK (
    transaction_type::text = ANY (
      ARRAY[
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
        'clawback'::character varying
      ]::text[]
    )
  );