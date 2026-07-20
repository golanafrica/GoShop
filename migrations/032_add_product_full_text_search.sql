-- ============================================================
-- Migration 032 : Ajout de la recherche plein texte (FTS) sur les produits
-- Objectif : Remplacer les ILIKE lents par un index GIN sur tsvector
-- ============================================================

-- 1. Ajouter la colonne search_vector de type tsvector (si elle n'existe pas)
ALTER TABLE products 
ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- 2. Initialiser la colonne pour les produits existants
-- On donne un poids 'A' (le plus élevé) au nom, et 'B' à la description.
-- La configuration 'french' active la racinisation (stemming) adaptée au Burkina Faso.
UPDATE products 
SET search_vector = 
    setweight(to_tsvector('french', coalesce(name, '')), 'A') ||
    setweight(to_tsvector('french', coalesce(description, '')), 'B')
WHERE search_vector IS NULL;

-- 3. Créer l'index GIN pour des recherches ultra-rapides
CREATE INDEX IF NOT EXISTS idx_products_search_vector 
ON products USING GIN (search_vector);

-- 4. Créer une fonction trigger pour maintenir le search_vector à jour automatiquement
-- Ainsi, l'application Go n'a pas à gérer cette logique lors des INSERT/UPDATE.
CREATE OR REPLACE FUNCTION products_search_vector_trigger() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('french', coalesce(NEW.name, '')), 'A') ||
        setweight(to_tsvector('french', coalesce(NEW.description, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 5. Attacher le trigger à la table products (en supprimant l'ancien s'il existe pour l'idempotence)
DROP TRIGGER IF EXISTS tsvectorupdate ON products;
CREATE TRIGGER tsvectorupdate 
BEFORE INSERT OR UPDATE OF name, description ON products
FOR EACH ROW EXECUTE FUNCTION products_search_vector_trigger();

-- ============================================================
-- Rollback (pour référence en cas de besoin)
-- DROP TRIGGER IF EXISTS tsvectorupdate ON products;
-- DROP FUNCTION IF EXISTS products_search_vector_trigger();
-- DROP INDEX IF EXISTS idx_products_search_vector;
-- ALTER TABLE products DROP COLUMN IF EXISTS search_vector;
-- ============================================================