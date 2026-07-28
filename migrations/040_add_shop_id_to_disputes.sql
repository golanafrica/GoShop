BEGIN;

-- 1. Ajout de la colonne avec la clé étrangère
ALTER TABLE disputes 
ADD COLUMN shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;

-- 2. Mise à jour des litiges existants
UPDATE disputes d 
SET shop_id = o.shop_id 
FROM orders o 
WHERE d.order_id = o.id AND d.shop_id IS NULL;

-- 3. Application de la contrainte NOT NULL
ALTER TABLE disputes ALTER COLUMN shop_id SET NOT NULL;

-- 4. Création de l'index
CREATE INDEX IF NOT EXISTS idx_disputes_shop_id ON disputes(shop_id);

COMMIT;