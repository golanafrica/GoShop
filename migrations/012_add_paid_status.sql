-- Migration 012 : Ajouter le statut 'paid' pour les commandes payées par Mobile Money
-- Date: 2026-06-29
-- Description: Le statut 'paid' est nécessaire pour distinguer les commandes payées 
--              par Mobile Money (workflow automatique) des commandes cash (workflow manuel)

-- ============================================
-- 1. Ajouter 'paid' à la contrainte orders_status_check
-- ============================================

-- Supprimer l'ancienne contrainte
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;

-- Recréer avec 'paid' inclus
ALTER TABLE orders ADD CONSTRAINT orders_status_check 
    CHECK (status IN (
        'pending', 
        'pending_confirmation',  -- Cash : en attente confirmation marchand
        'confirmed',             -- Cash : marchand a accepté
        'rejected',              -- Cash : marchand a refusé
        'expired',               -- Cash : délai dépassé
        'out_for_delivery',      -- Cash : en cours de livraison
        'delivered',             -- Cash : livré + payé
        'cancelled',             -- Annulé par client ou système
        'paid'                   -- 🆕 Mobile Money : paiement réussi, en attente de livraison
    ));

-- ============================================
-- 2. Documentation
-- ============================================

COMMENT ON CONSTRAINT orders_status_check ON orders IS 
    'Statuts possibles : 
     - pending : commande créée, en attente de paiement
     - pending_confirmation : cash, en attente confirmation marchand
     - confirmed : cash, marchand a accepté
     - rejected : cash, marchand a refusé
     - expired : cash, délai dépassé
     - out_for_delivery : cash, en cours de livraison
     - delivered : cash, livré + payé
     - cancelled : annulé par client ou système
     - paid : mobile money, paiement réussi, en attente de livraison';