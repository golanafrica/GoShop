-- ============================================================
-- Migration 034 : Ajout du rôle 'user' à la contrainte users_role_check
-- ============================================================
--
-- 🎯 Objectif :
--   Autoriser le rôle 'user' pour les inscriptions publiques,
--   en plus des rôles d'administration et de marchand existants.
--
-- 🔒 Sécurité :
--   - Le rôle 'user' est le rôle le moins privilégié
--   - Aucun compte existant n'est modifié
--   - La contrainte reste stricte (pas de rôle invalide possible)
--
-- 📋 Idempotence :
--   Cette migration peut être exécutée plusieurs fois sans erreur.
--
-- 📅 Date : 2026-07-20
-- 🔗 Impact : domain/entity/user_entity/user.go, application/usecase/user_usecase/registerUsecase.go
-- ============================================================

-- 1. Supprimer l'ancienne contrainte (idempotent grâce à IF EXISTS)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

-- 2. Recréer la contrainte avec TOUS les rôles valides du système RBAC
--    (les 6 existants + le nouveau 'user')
ALTER TABLE users ADD CONSTRAINT users_role_check 
CHECK (role IN (
    'user',              -- ✅ NOUVEAU : Rôle par défaut pour inscriptions publiques
    'merchant',          -- Marchand (boutique)
    'admin',             -- Administrateur plateforme
    'super_admin',       -- Super administrateur
    'credit_analyst',    -- Analyste crédit
    'support_agent',     -- Agent support
    'moderator'          -- Modérateur
));

-- 3. (Optionnel) Ajouter un commentaire pour documenter la contrainte
COMMENT ON CONSTRAINT users_role_check ON users IS 
'Contrainte RBAC : autorise uniquement les rôles définis dans le système. Le rôle user est le moins privilégié.';