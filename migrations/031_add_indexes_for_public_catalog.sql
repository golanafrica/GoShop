-- Migration 031 : Index pour optimiser le catalogue public et les jointures produits/boutiques

-- 1. Index sur la jointure produits -> boutiques
CREATE INDEX IF NOT EXISTS idx_products_shop_id ON products(shop_id);

-- 2. Index composite pour le filtre public (stock > 0) et le tri (created_at DESC)
CREATE INDEX IF NOT EXISTS idx_products_stock_created_at ON products(stock, created_at DESC);

-- 3. Index sur le statut d'activité des boutiques (pour le WHERE s.is_active = true)
CREATE INDEX IF NOT EXISTS idx_shops_is_active ON shops(is_active);

-- Analyse des tables pour mettre à jour les statistiques du planificateur de requêtes
ANALYZE products;
ANALYZE shops;

COMMENT ON INDEX idx_products_shop_id IS 'Optimise la jointure entre produits et boutiques pour le catalogue public';
COMMENT ON INDEX idx_products_stock_created_at IS 'Optimise le filtrage des produits en stock et le tri par date pour le catalogue public';