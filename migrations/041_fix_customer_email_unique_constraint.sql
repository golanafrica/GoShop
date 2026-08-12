BEGIN;

-- 1. Supprimer l'ancienne contrainte unique globale sur l'email (idempotent)
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_email_key;

-- 2. Ajouter la nouvelle contrainte unique composite (seulement si elle n'existe pas)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE table_name = 'customers' AND constraint_name = 'customers_shop_email_unique'
    ) THEN
        ALTER TABLE customers ADD CONSTRAINT customers_shop_email_unique UNIQUE (shop_id, email);
    END IF;
END $$;

-- 3. Index pour optimiser les requêtes (idempotent)
CREATE INDEX IF NOT EXISTS idx_customers_shop_id ON customers(shop_id);

COMMIT;