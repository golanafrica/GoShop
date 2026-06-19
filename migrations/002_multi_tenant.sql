-- migrations/002_multi_tenant.sql

-- ============================================
-- PHASE 1 : Multi-tenant + Shop
-- ============================================

-- 1. Table shops (boutiques)
-- ⚠️ owner_id est VARCHAR(36) pour être compatible avec users.id
CREATE TABLE IF NOT EXISTS shops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    custom_domain VARCHAR(255) UNIQUE,
    owner_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    logo_url TEXT,
    theme JSONB DEFAULT '{}'::jsonb,
    plan VARCHAR(50) DEFAULT 'free', -- free, pro, business
    db_schema VARCHAR(100) UNIQUE, -- Pour migration Option A future
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shops_slug ON shops(slug);
CREATE INDEX IF NOT EXISTS idx_shops_custom_domain ON shops(custom_domain);
CREATE INDEX IF NOT EXISTS idx_shops_owner ON shops(owner_id);

-- 2. Ajouter shop_id aux tables existantes
ALTER TABLE products ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shop_id UUID REFERENCES shops(id) ON DELETE CASCADE;

-- 3. Index pour le filtrage multi-tenant (critique pour la performance)
CREATE INDEX IF NOT EXISTS idx_products_shop ON products(shop_id);
CREATE INDEX IF NOT EXISTS idx_customers_shop ON customers(shop_id);
CREATE INDEX IF NOT EXISTS idx_orders_shop ON orders(shop_id);

-- 4. Table shop_payment_settings (paramètres paiement par boutique)
CREATE TABLE IF NOT EXISTS shop_payment_settings (
    shop_id UUID PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
    
    -- Cash à la livraison
    cash_enabled BOOLEAN DEFAULT true,
    cash_reservation_hours INTEGER DEFAULT 24,
    cash_min_amount BIGINT DEFAULT 10000,
    cash_delivery_zones JSONB DEFAULT '[]'::jsonb,
    cash_requires_approval BOOLEAN DEFAULT true,
    
    -- Wave
    wave_enabled BOOLEAN DEFAULT false,
    wave_api_key TEXT,
    
    -- Orange Money
    orange_money_enabled BOOLEAN DEFAULT false,
    orange_merchant_code TEXT,
    orange_money_api_key TEXT,
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 5. Fonction pour mettre à jour updated_at automatiquement
CREATE OR REPLACE FUNCTION update_shops_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- PostgreSQL ne supporte pas CREATE TRIGGER IF NOT EXISTS
-- On doit donc DROP avant CREATE
DROP TRIGGER IF EXISTS shops_updated_at ON shops;
CREATE TRIGGER shops_updated_at
    BEFORE UPDATE ON shops
    FOR EACH ROW
    EXECUTE FUNCTION update_shops_updated_at();

DROP TRIGGER IF EXISTS shop_payment_settings_updated_at ON shop_payment_settings;
CREATE TRIGGER shop_payment_settings_updated_at
    BEFORE UPDATE ON shop_payment_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_shops_updated_at();

-- 6. Données de test (shop de démonstration)
DO $$
DECLARE
    demo_user_id VARCHAR(36);
    demo_shop_id UUID := 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
BEGIN
    -- Récupérer un utilisateur existant
    SELECT id INTO demo_user_id FROM users LIMIT 1;
    
    IF demo_user_id IS NOT NULL THEN
        -- Créer un shop de démo
        INSERT INTO shops (id, name, slug, owner_id, plan, is_active)
        VALUES (demo_shop_id, 'Boutique Démo', 'demo', demo_user_id, 'free', true)
        ON CONFLICT (id) DO NOTHING;
        
        -- Créer les settings par défaut
        INSERT INTO shop_payment_settings (shop_id)
        VALUES (demo_shop_id)
        ON CONFLICT (shop_id) DO NOTHING;
        
        -- Associer les données existantes au shop de démo
        UPDATE products SET shop_id = demo_shop_id WHERE shop_id IS NULL;
        UPDATE customers SET shop_id = demo_shop_id WHERE shop_id IS NULL;
        UPDATE orders SET shop_id = demo_shop_id WHERE shop_id IS NULL;
    END IF;
END $$;