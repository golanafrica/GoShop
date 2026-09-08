-- Migration 051 : Création du service DeliveryZone partagé
-- Date: 2026-09-08
-- Description: Table delivery_zones pour gérer les délais par zone géographique
--              Service partagé entre : Vente Classique, Tontine, COD, Paiement en Tranches

-- ============================================
-- 1. Création de la table delivery_zones
-- ============================================

CREATE TABLE IF NOT EXISTS delivery_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Identification
    zone_code VARCHAR(20) UNIQUE NOT NULL,
    zone_name VARCHAR(100) NOT NULL,
    country VARCHAR(50) NOT NULL,
    region VARCHAR(100),
    zone_type VARCHAR(20) NOT NULL CHECK (zone_type IN ('urban', 'peri_urban', 'rural', 'remote', 'international')),
    
    -- Délais configurables (en jours)
    delivery_delay_days INT NOT NULL DEFAULT 7 CHECK (delivery_delay_days >= 1 AND delivery_delay_days <= 30),
    return_delay_days INT NOT NULL DEFAULT 14 CHECK (return_delay_days >= 3 AND return_delay_days <= 60),
    warranty_response_days INT NOT NULL DEFAULT 7 CHECK (warranty_response_days >= 1 AND warranty_response_days <= 30),
    cod_confirmation_delay_days INT NOT NULL DEFAULT 7 CHECK (cod_confirmation_delay_days >= 1 AND cod_confirmation_delay_days <= 30),
    installment_release_delay_days INT NOT NULL DEFAULT 7 CHECK (installment_release_delay_days >= 3 AND installment_release_delay_days <= 30),
    
    -- Métadonnées
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    priority INT DEFAULT 0,
    
    -- Audit (VARCHAR(36) pour compatibilité avec users.id qui est VARCHAR(36))
    created_by VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    updated_by VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index pour performance
CREATE INDEX IF NOT EXISTS idx_delivery_zones_country ON delivery_zones(country);
CREATE INDEX IF NOT EXISTS idx_delivery_zones_zone_type ON delivery_zones(zone_type);
CREATE INDEX IF NOT EXISTS idx_delivery_zones_is_active ON delivery_zones(is_active);
CREATE INDEX IF NOT EXISTS idx_delivery_zones_priority ON delivery_zones(priority DESC);

COMMENT ON TABLE delivery_zones IS 'Zones de livraison avec délais configurables pour tous les systèmes de vente';
COMMENT ON COLUMN delivery_zones.zone_code IS 'Code unique (ex: BF-OUAGA-URB)';
COMMENT ON COLUMN delivery_zones.delivery_delay_days IS 'Délai standard de livraison';
COMMENT ON COLUMN delivery_zones.return_delay_days IS 'Délai autorisé pour retour produit';
COMMENT ON COLUMN delivery_zones.warranty_response_days IS 'Délai de prise en charge SAV';
COMMENT ON COLUMN delivery_zones.cod_confirmation_delay_days IS 'Délai confirmation Cash on Delivery';
COMMENT ON COLUMN delivery_zones.installment_release_delay_days IS 'Délai sécurité avant auto-release (paiement en tranches)';

-- ============================================
-- 2. Seed : Zones Burkina Faso (max 10 jours)
-- ============================================

INSERT INTO delivery_zones (zone_code, zone_name, country, region, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, description, priority) VALUES
('BF-OUAGA-URB',   'Ouagadougou Urbain',        'Burkina Faso', 'Centre',         'urban',      3, 7,  5, 5, 5,  'Patte d''Oie, 2000, Koulouba, Cissin, Gounghin', 100),
('BF-OUAGA-PERI',  'Ouagadougou Périphérie',    'Burkina Faso', 'Centre',         'peri_urban', 5, 10, 7, 7, 7,  'Saaba, Karpala, Tanghin Dassouri, Komki-Ipala', 90),
('BF-BOBO-URB',    'Bobo-Dioulasso Urbain',     'Burkina Faso', 'Hauts-Bassins',  'urban',      3, 7,  5, 5, 5,  'Centre-ville, Do, Dafra, Kelkou', 95),
('BF-BOBO-PERI',   'Bobo-Dioulasso Périphérie', 'Burkina Faso', 'Hauts-Bassins',  'peri_urban', 5, 10, 7, 7, 7,  'Banlieues, villages proches', 85),
('BF-KUDOUGOU',    'Koudougou',                 'Burkina Faso', 'Centre-Ouest',   'urban',      5, 10, 7, 7, 7,  '3ème ville du Burkina', 80),
('BF-OUAHIGOUYA',  'Ouahigouya',                'Burkina Faso', 'Nord',           'urban',      7, 14, 10, 10, 10, 'Capitale du Nord', 75),
('BF-KAYA',        'Kaya',                      'Burkina Faso', 'Centre-Nord',    'urban',      7, 14, 10, 10, 10, 'Centre administratif', 70),
('BF-FADA',        'Fada N''Gourma',            'Burkina Faso', 'Est',            'urban',      7, 14, 10, 10, 10, 'Capitale de l''Est', 70),
('BF-BANFORA',     'Banfora',                   'Burkina Faso', 'Cascades',       'urban',      7, 14, 10, 10, 10, 'Capitale des Cascades', 70),
('BF-DEDUOGOU',    'Dédougou',                  'Burkina Faso', 'Boucle du Mouhoun', 'urban',   7, 14, 10, 10, 10, 'Capitale de la Boucle du Mouhoun', 70),
('BF-NORD-RURAL',  'Nord Rural',                'Burkina Faso', 'Nord-Sahel',     'rural',      10, 21, 14, 14, 10, 'Dori, Djibo, Gorom-Gorom, zones rurales du Nord', 50),
('BF-EST-RURAL',   'Est Rural',                 'Burkina Faso', 'Est',            'rural',      10, 21, 14, 14, 10, 'Kantchari, Diagourou, zones rurales de l''Est', 50),
('BF-OUEST-RURAL', 'Ouest Rural',               'Burkina Faso', 'Ouest',          'rural',      10, 21, 14, 14, 10, 'Nouna, Dô, zones rurales de l''Ouest', 50),
('BF-SUD-RURAL',   'Sud Rural',                 'Burkina Faso', 'Cascades',       'rural',      10, 21, 14, 14, 10, 'Gaoua, Diébougou, zones rurales du Sud', 50),
('BF-CENTRE-RURAL','Centre Rural',              'Burkina Faso', 'Centre',         'rural',      7,  14, 10, 10, 10, 'Villages autour de Ouagadougou', 60);

-- ============================================
-- 3. Seed : Zones Côte d'Ivoire
-- ============================================

INSERT INTO delivery_zones (zone_code, zone_name, country, region, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, description, priority) VALUES
('CI-ABJ-URB',   'Abidjan Urbain',       'Côte d''Ivoire', 'Lagunes',        'urban',      3, 7,  5, 5, 5,  'Cocody, Plateau, Marcory, Treichville, Riviera', 100),
('CI-ABJ-PERI',  'Abidjan Périphérie',   'Côte d''Ivoire', 'Lagunes',        'peri_urban', 5, 10, 7, 7, 7,  'Yopougon, Abobo, Anyama, Adjamé', 90),
('CI-BDK',       'Bouaké',               'Côte d''Ivoire', 'Gbêkê',          'urban',      5, 10, 7, 7, 7,  '2ème ville de Côte d''Ivoire', 85),
('CI-YAM',       'Yamoussoukro',         'Côte d''Ivoire', 'Bélier',         'urban',      5, 10, 7, 7, 7,  'Capitale politique', 80),
('CI-KOR',       'Korhogo',              'Côte d''Ivoire', 'Poro',           'urban',      7, 14, 10, 10, 10, 'Capitale du Nord', 75),
('CI-MAN',       'Man',                  'Côte d''Ivoire', 'Tonkpi',         'urban',      7, 14, 10, 10, 10, 'Capitale de l''Ouest', 75),
('CI-DAL',       'Daloa',                'Côte d''Ivoire', 'Haut-Sassandra', 'urban',      7, 14, 10, 10, 10, 'Capitale du Centre-Ouest', 75),
('CI-SAN',       'San-Pédro',            'Côte d''Ivoire', 'San-Pédro',      'urban',      7, 14, 10, 10, 10, 'Port industriel', 75),
('CI-INTERIEUR', 'Intérieur CI',         'Côte d''Ivoire', 'Multiple',       'rural',      10, 21, 14, 14, 10, 'Zones rurales et villes secondaires', 50);

-- ============================================
-- 4. Seed : Zones Sénégal
-- ============================================

INSERT INTO delivery_zones (zone_code, zone_name, country, region, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, description, priority) VALUES
('SN-DAKAR-URB',  'Dakar Urbain',        'Sénégal',       'Dakar',          'urban',      3, 7,  5, 5, 5,  'Plateau, Almadies, Médina, Fann', 100),
('SN-DAKAR-PERI', 'Dakar Périphérie',    'Sénégal',       'Dakar',          'peri_urban', 5, 10, 7, 7, 7,  'Pikine, Guédiawaye, Rufisque', 90),
('SN-THIES',      'Thiès',               'Sénégal',       'Thiès',          'urban',      5, 10, 7, 7, 7,  'Ville industrielle', 85),
('SN-SAINT-LOUIS','Saint-Louis',         'Sénégal',       'Saint-Louis',    'urban',      7, 14, 10, 10, 10, 'Ancienne capitale', 80),
('SN-KAOLACK',    'Kaolack',             'Sénégal',       'Kaolack',        'urban',      7, 14, 10, 10, 10, 'Capitale du Bassin arachidier', 75),
('SN-INTERIEUR',  'Intérieur Sénégal',   'Sénégal',       'Multiple',       'rural',      10, 21, 14, 14, 10, 'Zones rurales', 50);

-- ============================================
-- 5. Seed : Autres pays CEDEAO
-- ============================================

INSERT INTO delivery_zones (zone_code, zone_name, country, region, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, description, priority) VALUES
('ML-BKO',        'Bamako',              'Mali',          'Bamako',         'urban',      5, 10, 7, 7, 7,  'Capitale malienne', 85),
('ML-INTERIEUR',  'Intérieur Mali',      'Mali',          'Multiple',       'remote',     14, 30, 21, 21, 14, 'Tombouctou, Gao, Mopti, zones reculées', 40),
('NE-NIAM',       'Niamey',              'Niger',         'Niamey',         'urban',      5, 10, 7, 7, 7,  'Capitale nigérienne', 85),
('NE-INTERIEUR',  'Intérieur Niger',     'Niger',         'Multiple',       'remote',     14, 30, 21, 21, 14, 'Agadez, Zinder, Maradi, zones reculées', 40),
('TG-LME',        'Lomé',                'Togo',          'Maritime',       'urban',      3, 7,  5, 5, 5,  'Capitale togolaise', 90),
('TG-INTERIEUR',  'Intérieur Togo',      'Togo',          'Multiple',       'rural',      10, 21, 14, 14, 10, 'Kara, Sokodé, Atakpamé', 50),
('BJ-COT',        'Cotonou',             'Bénin',         'Littoral',       'urban',      3, 7,  5, 5, 5,  'Capitale économique', 90),
('BJ-INTERIEUR',  'Intérieur Bénin',     'Bénin',         'Multiple',       'rural',      10, 21, 14, 14, 10, 'Porto-Novo, Parakou, Abomey', 50),
('GH-ACC',        'Accra',               'Ghana',         'Grand Accra',    'urban',      3, 7,  5, 5, 5,  'Capitale ghanéenne', 90),
('GH-KUM',        'Kumasi',              'Ghana',         'Ashanti',        'urban',      5, 10, 7, 7, 7,  '2ème ville du Ghana', 85),
('GH-INTERIEUR',  'Intérieur Ghana',     'Ghana',         'Multiple',       'rural',      10, 21, 14, 14, 10, 'Tamale, Takoradi, zones rurales', 50);

-- ============================================
-- 6. Seed : Livraisons inter-pays
-- ============================================

INSERT INTO delivery_zones (zone_code, zone_name, country, region, zone_type, delivery_delay_days, return_delay_days, warranty_response_days, cod_confirmation_delay_days, installment_release_delay_days, description, priority) VALUES
('INT-CEDEAO-1', 'CEDEAO Zone 1',       'International', 'Afrique de l''Ouest', 'international', 14, 30, 21, 21, 14, 'Burkina → Côte d''Ivoire, Mali, Niger, Togo, Bénin', 30),
('INT-CEDEAO-2', 'CEDEAO Zone 2',       'International', 'Afrique de l''Ouest', 'international', 21, 45, 30, 30, 21, 'Ghana, Sénégal, Nigeria, etc.', 20),
('INT-AFRIQ-1',  'Afrique Centrale',    'International', 'Afrique Centrale',    'international', 21, 45, 30, 30, 21, 'Cameroun, Gabon, Congo', 20),
('INT-AFRIQ-2',  'Afrique de l''Est',   'International', 'Afrique de l''Est',   'international', 30, 60, 30, 30, 30, 'Kenya, Tanzanie, Ouganda', 10),
('INT-EUROPE',   'Europe',              'International', 'Europe',              'international', 30, 60, 30, 30, 30, 'France, Belgique, etc.', 10),
('INT-AMERIQUE', 'Amériques',           'International', 'Amériques',           'international', 30, 60, 30, 30, 30, 'USA, Canada, etc.', 10);

-- ============================================
-- 7. Fonction auto-update updated_at
-- ============================================

CREATE OR REPLACE FUNCTION update_delivery_zones_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_delivery_zones_updated_at
    BEFORE UPDATE ON delivery_zones
    FOR EACH ROW
    EXECUTE FUNCTION update_delivery_zones_updated_at();

-- ============================================
-- 8. Résumé
-- ============================================

DO $$
DECLARE
    zone_count INT;
BEGIN
    SELECT COUNT(*) INTO zone_count FROM delivery_zones;
    RAISE NOTICE '✅ Migration 051 terminée avec succès';
    RAISE NOTICE '   - Table delivery_zones créée';
    RAISE NOTICE '   - % zones pré-configurées', zone_count;
    RAISE NOTICE '   - Index créés pour performance';
    RAISE NOTICE '   - Trigger auto-update activé';
END $$;