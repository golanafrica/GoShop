-- migrations/029_drop_payments_order_id_fkey.sql
-- Description: Supprimer la contrainte de clé étrangère sur order_id dans la table payments
-- pour permettre les paiements polymorphes (ex: credit_down_payment, credit_installment)
-- qui ne sont pas liés à une commande classique.

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_order_id_fkey;

DO $$
BEGIN
  RAISE NOTICE '✅ Migration 029 appliquée : Contrainte payments_order_id_fkey supprimée';
END $$;