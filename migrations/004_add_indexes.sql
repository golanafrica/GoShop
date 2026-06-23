-- migrations/004_add_indexes.sql
-- ============================================
-- PHASE 2 : Optimisation des performances
-- ============================================

-- Index sur email pour accélérer les recherches d'utilisateurs
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Index fonctionnel pour recherches case-insensitive
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users(LOWER(email));

-- Index sur les timestamps pour les tris et filtres
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);