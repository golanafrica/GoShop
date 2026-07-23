-- ============================================================
-- Migration 036 : Renforcement idempotence et audit webhooks
-- Objectif : Garantir qu'un webhook n'est jamais traité deux fois
-- ============================================================

-- 0. 🧹 NETTOYAGE PRÉALABLE AUTOMATIQUE
-- Supprime les anciens doublons en ne gardant que l'entrée la plus récente pour chaque (provider, external_id).
-- Cela garantit que la création de l'index unique à l'étape 3 ne plantera pas.
DELETE FROM payment_webhooks
WHERE id NOT IN (
    SELECT DISTINCT ON (provider, external_id) id
    FROM payment_webhooks
    WHERE external_id IS NOT NULL AND external_id != ''
    ORDER BY provider, external_id, received_at DESC
);

-- 1. Ajouter la colonne webhook_external_id à la table payments
ALTER TABLE payments
ADD COLUMN IF NOT EXISTS webhook_external_id VARCHAR(255);

COMMENT ON COLUMN payments.webhook_external_id IS
'ID unique du webhook (fourni par le provider) qui a traité ce paiement. Utilisé pour audit et idempotence.';

-- 2. Créer un index pour les recherches rapides par webhook_external_id
CREATE INDEX IF NOT EXISTS idx_payments_webhook_external_id
ON payments(webhook_external_id)
WHERE webhook_external_id IS NOT NULL;

-- 3. 🛡️ Index unique partiel pour l'idempotence
-- Maintenant que les doublons sont supprimés, cela passera sans erreur sur toutes les bases.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_webhooks_provider_external_id
ON payment_webhooks(provider, external_id)
WHERE external_id IS NOT NULL AND external_id != '';

COMMENT ON INDEX uq_payment_webhooks_provider_external_id IS
'Contrainte d''idempotence : chaque webhook (identifié par provider + external_id non vide) ne peut être enregistré qu''une seule fois.';

-- 4. Index composite pour accélérer la vérification d'idempotence
CREATE INDEX IF NOT EXISTS idx_payment_webhooks_provider_external_processed
ON payment_webhooks(provider, external_id, processed);

-- 5. Mise à jour des statistiques
ANALYZE payments;
ANALYZE payment_webhooks;

COMMENT ON INDEX idx_payments_webhook_external_id IS 'Accélère les recherches de paiements par ID de webhook externe';
COMMENT ON INDEX idx_payment_webhooks_provider_external_processed IS 'Accélère la vérification d''idempotence des webhooks';