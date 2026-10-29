-- 068_webhook_claim_id.sql
-- PR 3 : Ajout de claim_id pour renforcer la propriété du bail et empêcher les finalisations obsolètes.

BEGIN;

ALTER TABLE payment_webhooks
  ADD COLUMN IF NOT EXISTS claim_id UUID;

COMMENT ON COLUMN payment_webhooks.claim_id IS 'Identifiant unique du claim actuel. Doit correspondre pour finaliser (processed/failed).';

COMMIT;