BEGIN;

-- 1. Ajout de la colonne (seulement si elle n'existe pas)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'disputes' AND column_name = 'shop_id'
    ) THEN
        ALTER TABLE disputes ADD COLUMN shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;
    END IF;
END $$;

-- 2. Mise à jour des litiges existants (idempotent par nature)
UPDATE disputes d 
SET shop_id = o.shop_id 
FROM orders o 
WHERE d.order_id = o.id AND d.shop_id IS NULL;

-- 3. Application de la contrainte NOT NULL (seulement si pas déjà fait)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'disputes' AND column_name = 'shop_id' AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE disputes ALTER COLUMN shop_id SET NOT NULL;
    END IF;
END $$;

-- 4. Création de l'index (idempotent)
CREATE INDEX IF NOT EXISTS idx_disputes_shop_id ON disputes(shop_id);

COMMIT;