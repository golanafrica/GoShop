-- migrations/043_fix_tontine_min_participants_default.sql
-- Correction du défaut min_participants à 3 pour correspondre à la règle métier réaliste (3 cycles = 3 membres)

ALTER TABLE product_tontine_settings 
ALTER COLUMN min_participants SET DEFAULT 3;

-- Mise à jour des enregistrements existants qui avaient la valeur par défaut de 4 
-- mais qui devraient logiquement être à 3 pour les petites tontines
UPDATE product_tontine_settings 
SET min_participants = 3 
WHERE min_participants = 4;