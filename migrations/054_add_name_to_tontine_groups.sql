-- ============================================
-- Migration 054 : Add Name to Tontine Groups
-- ============================================
-- Description : Ajoute un nom personnalisé aux groupes de tontine 
--               pour une meilleure identification (ex: "Famille Diallo", "Bureau Compta").

-- 1. Ajout de la colonne name
ALTER TABLE tontine_groups 
ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT 'Groupe Tontine';

-- 2. Ajout d'un commentaire explicatif
COMMENT ON COLUMN tontine_groups.name IS 'Nom personnalisé du groupe de tontine (ex: "Famille Diallo", "Cercle Bureau")';

-- 3. Confirmation de l'exécution
DO $$
BEGIN
    RAISE NOTICE '✅ Migration 054 terminée avec succès : colonne "name" ajoutée à tontine_groups';
END $$;