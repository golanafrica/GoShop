-- ============================================================
-- Migration 033 : Ajout de la colonne user_id à la table customers
-- ============================================================
--
-- 🎯 Objectif :
--   Lier l'entité métier Customer à l'entité d'authentification User.
--   Cette colonne permet de résoudre le UserID à partir du CustomerID
--   pour envoyer des notifications WebSocket au bon destinataire.
--
-- 📋 Idempotence :
--   Cette migration peut être exécutée plusieurs fois sans erreur.
--   - IF NOT EXISTS pour la colonne
--   - IF NOT EXISTS pour l'index
--
-- 🔒 Rétrocompatibilité :
--   Les anciens enregistrements auront user_id = '' (chaîne vide).
--   Ils pourront être mis à jour manuellement ou via un script
--   de migration de données si nécessaire.
--
-- 📅 Date : 2026-07-20
-- 🔗 Impact : domain/entity/customer.go, infrastructure/notification/dispatcher.go
-- ============================================================

-- 1. Ajouter la colonne user_id (idempotent)
ALTER TABLE customers 
ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) NOT NULL DEFAULT '';

-- 2. Ajouter un commentaire descriptif sur la colonne
COMMENT ON COLUMN customers.user_id IS 
'Lien vers la table users.id pour résoudre les notifications WebSocket. Vide si le client est un invité (guest).';

-- 3. Créer un index pour accélérer les recherches par user_id (idempotent)
-- Critique pour les performances du NotificationDispatcher
CREATE INDEX IF NOT EXISTS idx_customers_user_id 
ON customers(user_id);

-- 4. (Optionnel) Créer un index partiel pour les clients liés à un user
-- Utile si on veut rapidement trouver tous les clients authentifiés
CREATE INDEX IF NOT EXISTS idx_customers_user_id_not_empty 
ON customers(user_id) 
WHERE user_id != '';

-- ============================================================
-- ✅ Migration terminée avec succès
-- ============================================================