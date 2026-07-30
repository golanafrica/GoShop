BEGIN;

-- ============================================================
-- Migration 041 : Correction de l'unicité de l'email client
-- Objectif : Permettre à un même client (même email) d'exister 
-- dans plusieurs boutiques (multi-tenant), tout en interdisant 
-- les doublons au sein d'une même boutique.
-- ============================================================

-- 1. Supprimer l'ancienne contrainte unique globale sur l'email
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_email_key;

-- 2. Ajouter une nouvelle contrainte unique composite (shop_id + email)
-- Cela garantit qu'un email est unique PAR boutique, mais peut être réutilisé dans une autre boutique.
ALTER TABLE customers ADD CONSTRAINT customers_shop_email_unique UNIQUE (shop_id, email);

-- 3. Index pour optimiser les requêtes de recherche par boutique
CREATE INDEX IF NOT EXISTS idx_customers_shop_id ON customers(shop_id);

COMMIT;