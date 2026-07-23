-- ============================================================
-- Migration 037 : Correction de l'index unique pour l'idempotence des webhooks
-- Objectif : Remplacer l'ancien index (partiel ou contrainte) par un index unique standard
--            pour que la clause "ON CONFLICT (provider, external_id)" de PostgreSQL fonctionne.
-- ============================================================

-- 1. Supprimer la contrainte UNIQUE si elle existe (cas de goshop_test)
ALTER TABLE payment_webhooks 
DROP CONSTRAINT IF EXISTS uq_payment_webhooks_provider_external_id;

-- 2. Supprimer l'index s'il existe encore (cas de goshop_db, ou si la contrainte n'existait pas)
DROP INDEX IF EXISTS uq_payment_webhooks_provider_external_id;

-- 3. Créer un index unique standard sur (provider, external_id)
-- Cela permet à PostgreSQL de faire correspondre exactement la clause ON CONFLICT du code Go
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_webhooks_provider_external_id
ON payment_webhooks(provider, external_id);

COMMENT ON INDEX uq_payment_webhooks_provider_external_id IS
'Index unique standard pour garantir l''idempotence des webhooks via ON CONFLICT (provider, external_id)';

-- 4. Mise à jour des statistiques pour le planificateur de requêtes
ANALYZE payment_webhooks;

DO $$
BEGIN
    RAISE NOTICE 'Migration 037 appliquee avec succes : Index unique standard pour l''idempotence des webhooks cree.';
END $$;