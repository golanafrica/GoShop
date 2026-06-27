-- Migration 009 : Cash à la livraison
-- Date: 2026-06-28
-- Description: Ajout du support pour le paiement à la livraison
--              avec workflow de confirmation marchand

-- ============================================
-- 1. Enrichissement de la table orders
-- ============================================

-- Méthode de paiement (défaut: mobile_money pour commandes existantes)
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_method VARCHAR(50) 
    NOT NULL DEFAULT 'mobile_money';

-- Timestamps du workflow cash
ALTER TABLE orders ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS rejected_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;

-- Informations de livraison
ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_notes TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS amount_received_cents BIGINT;

-- Expiration de la réservation stock (pour cash)
ALTER TABLE orders ADD COLUMN IF NOT EXISTS reserved_until TIMESTAMPTZ;

-- Modification du CHECK sur status pour inclure les nouveaux statuts
-- D'abord supprimer l'ancienne contrainte si elle existe
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;

-- Vérifier et nettoyer les statuts invalides avant d'ajouter la contrainte
-- Convertir les statuts invalides en 'cancelled'
UPDATE orders 
SET status = 'cancelled', cancelled_at = NOW()
WHERE status NOT IN ('pending', 'pending_confirmation', 'confirmed', 
                     'rejected', 'expired', 'out_for_delivery', 
                     'delivered', 'cancelled');

-- Maintenant ajouter la contrainte CHECK
ALTER TABLE orders ADD CONSTRAINT orders_status_check 
    CHECK (status IN (
        'pending', 
        'pending_confirmation',  -- 🆕 Cash : en attente confirmation marchand
        'confirmed',             -- 🆕 Cash : marchand a accepté
        'rejected',              -- 🆕 Cash : marchand a refusé
        'expired',               -- 🆕 Cash : délai dépassé
        'out_for_delivery',      -- 🆕 Cash : en cours de livraison
        'delivered',             -- 🆕 Cash : livré + payé
        'cancelled'              -- Annulé par client ou système
    ));

-- Contrainte CHECK sur payment_method
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check 
    CHECK (payment_method IN ('mobile_money', 'cash_on_delivery'));

-- Index pour les requêtes de workflow cash
CREATE INDEX IF NOT EXISTS idx_orders_status_payment_method 
    ON orders(status, payment_method);
CREATE INDEX IF NOT EXISTS idx_orders_reserved_until 
    ON orders(reserved_until) 
    WHERE status = 'pending_confirmation';

-- ============================================
-- 2. Commission cash dans shop_payment_settings
-- ============================================

-- Taux de commission GoShop sur les paiements cash (en basis points)
-- Exemple : 250 = 2.50%, 100 = 1.00%, 0 = pas de commission
ALTER TABLE shop_payment_settings ADD COLUMN IF NOT EXISTS cash_commission_rate INTEGER 
    NOT NULL DEFAULT 250
    CHECK (cash_commission_rate >= 0 AND cash_commission_rate <= 10000);

-- Cash à la livraison activé/désactivé par boutique
ALTER TABLE shop_payment_settings ADD COLUMN IF NOT EXISTS cash_on_delivery_enabled BOOLEAN 
    NOT NULL DEFAULT false;

COMMENT ON COLUMN orders.payment_method IS 'Méthode de paiement : mobile_money ou cash_on_delivery';
COMMENT ON COLUMN orders.reserved_until IS 'Timestamp d''expiration de la réservation stock (pour cash)';
COMMENT ON COLUMN orders.amount_received_cents IS 'Montant effectivement reçu en cash (centimes)';
COMMENT ON COLUMN shop_payment_settings.cash_commission_rate IS 'Commission GoShop en basis points (250 = 2.50%)';
COMMENT ON COLUMN shop_payment_settings.cash_on_delivery_enabled IS 'Activation du paiement à la livraison pour cette boutique';