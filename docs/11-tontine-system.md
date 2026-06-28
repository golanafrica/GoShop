# 🏦 Système de Tontine GoShop

**Version** : v1.0.0  
**Date** : 2026-06-28  
**Statut** : 🚧 En conception — Prêt à implémenter

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Modèle économique](#2-modèle-économique)
3. [Acteurs et rôles](#3-acteurs-et-rôles)
4. [Workflow KYC](#4-workflow-kyc)
5. [Workflow Tontine](#5-workflow-tontine)
6. [Architecture technique](#6-architecture-technique)
7. [Modèle de données](#7-modèle-de-données)
8. [API Endpoints](#8-api-endpoints)
9. [Intégration YengaPay](#9-intégration-yengapay)
10. [Sécurité et conformité](#10-sécurité-et-conformité)
11. [Limites et décisions](#11-limites-et-décisions)
12. [Roadmap](#12-roadmap)

---

## 1. Vue d'ensemble

### 🎯 Qu'est-ce que la Tontine GoShop ?

La **Tontine GoShop** est un système de **tontine de biens physiques** qui permet à un groupe de personnes (famille, amis, collègues, commerçants) de cotiser régulièrement pour acquérir un bien de valeur (moto, congélateur, voiture, etc.) à tour de rôle.

### 💡 Différence avec Taaraogo

| Projet | Nature | Tontine |
|--------|--------|---------|
| **GoShop** 🛒 | E-commerce SaaS | Tontine de **biens physiques** (voucher pour un produit) |
| **Taaraogo** 💰 | Fintech super-app | Tontine **financière** (redistribution d'argent) |

### 🎯 Cas d'usage typique

**Exemple** : 8 collègues veulent chacun une moto à 500 000 FCFA


Semaine 1 : 8 × 62 500 F cotisés → Client A reçoit sa moto
Semaine 2 : 8 × 62 500 F cotisés → Client B reçoit sa moto
...
Semaine 8 : 8 × 62 500 F cotisés → Client H reçoit sa moto
Total collecté : 4 000 000 FCFA
8 motos livrées au total
Commission GoShop : 100 000 FCFA (2.50% de 4M)


---

## 2. Modèle économique

### 💰 Flux d'argent


Client A paie 62 500 ──► [YengaPay] ──► [GoShop] ──► Marchand
Client B paie 62 500 ──► [YengaPay] ──► [GoShop] ──► Marchand
Client C paie 62 500 ──► [YengaPay] ──► [GoShop] ──► Marchand
...
À chaque cycle terminé :
→ Voucher généré pour le bénéficiaire
→ Fonds libérés au marchand
→ Client va chercher son bien chez le marchand


### 📊 Répartition des montants

| Acteur | Montant | Commentaire |
|--------|---------|-------------|
| **Client** | 62 500 F / cycle | Paiement via YengaPay |
| **Marchand** | 60 938 F / cycle | Reçoit après commission |
| **GoShop** | 1 562 F / cycle | Commission 2.50% (configurable 0-15%) |

### 🎯 Modèle de commission

**Option A retenue** : Commission prélevée sur **chaque cotisation**

**Avantages** :
- ✅ Trésorerie lissée pour GoShop (revenus réguliers)
- ✅ Acceptation psychologique pour le marchand (petites retenues vs grosse amputation)
- ✅ Cohérent avec le modèle COD existant

**Calcul** :
```go
goshop_part := amount_cents * tontine_commission_rate / 10000
// Exemple : 6250000 * 250 / 10000 = 156250 centimes = 1 562 F

🎲 Modèle de risque
Niveau
Gestion
GoShop
0 risque — prend juste la commission
Marchand
0 risque — reçoit l'argent à chaque cycle
Participants
Confiance sociale (famille/amis/collègues)
Pas de caution, pas de garantie technique — le cercle social gère les défauts.
3. Acteurs et rôles
👥 Les 3 acteurs
Acteur
Rôle
KYC
Qui valide ?
Marchand
Reçoit l'argent, livre le produit
✅ Auto via shop_id
Système
Client créateur
Organise la tontine, invite
✅ Manuel
Marchand
Client participant
Paie les cotisations
✅ Manuel
Marchand
🔐 Niveaux KYC
Niveau
Capabilities
none
Compte basique, ne peut PAS participer à une tontine
pending
Documents uploadés, en attente de validation
verified
Peut créer/rejoindre une tontine
rejected
Documents rejetés, doit re-soumettre
📋 Matrice des permissions
Action
Marchand
Client créateur
Client participant
Créer une boutique
✅ Auto
❌ N/A
❌ N/A
Activer tontine sur produit
✅ Auto
❌
❌
Créer un groupe tontine
✅ Auto
✅ verified
❌
Rejoindre un groupe
❌ N/A
✅ verified
✅ verified
Payer cotisation
❌ N/A
✅ verified
✅ verified
Recevoir voucher
❌ N/A
✅ verified
✅ verified
Valider voucher
✅ Auto
❌
❌
Valider KYC client
✅ Auto
❌
❌
4. Workflow KYC
📤 Côté client


1. Client arrive sur produit avec tontine
   → Voit "Créer une tontine" ou "Payer cash à 450 000 F"
   → Clique → ❌ "Votre identité doit être vérifiée"
   → Télécharge CNI ou passeport (photo)
   → Statut : `pending`

2. Marchand reçoit notification
   → Voit les documents dans son dashboard
   → Vérifie l'identité (carte ID, téléphone, appel si besoin)
   → Clique "Valider KYC" ou "Rejeter"
   → Client passe à `verified` ou `rejected`

3. Client vérifié
   → Peut créer une tontine
   → Peut rejoindre une tontine


🏪 Côté marchand

1. Marchand (déjà verified via shop_id)
   → Dashboard → Produit → "Activer la tontine"
   → Configure paramètres (type cercle, min/max participants)
   → Crée groupe
   → Partage code d'invitation

2. Client veut rejoindre
   → Doit être verified
   → Marchand valide KYC si nécessaire
   → Client rejoint avec code


1. Marchand (déjà verified via shop_id)
   → Dashboard → Produit → "Activer la tontine"
   → Configure paramètres (type cercle, min/max participants)
   → Crée groupe
   → Partage code d'invitation

2. Client veut rejoindre
   → Doit être verified
   → Marchand valide KYC si nécessaire
   → Client rejoint avec code


📁 Stockage des documents

/uploads/
  /kyc/
    /{customer_id}/
      cni_20260628_143022.jpg
      passport_20260628_143025.jpg



Contraintes :
Taille max : 5 Mo par document
Types acceptés : JPG, PNG, PDF
Max 3 documents par client
5. Workflow Tontine
🔄 Machine à états


     ┌─────────────────┐
     │ PENDING_MEMBERS │  ← En attente de participants
     └────────┬────────┘
              │ groupe complet
              ▼
     ┌─────────────────┐
     │     ACTIVE      │  ← Cycles en cours
     └────────┬────────┘
              │ tous cycles terminés
              ▼
     ┌─────────────────┐
     │    COMPLETED    │  ← Terminé
     └─────────────────┘


📅 Cycle de paiement

Cycle N démarre
    │
    ├─► Tous les participants reçoivent notification
    │
    ├─► Chaque participant paie sa cotisation via YengaPay
    │   └─► Webhook YengaPay → GoShop enregistre paiement
    │
    ├─► Tous ont payé ?
    │   ├─► OUI → Générer voucher pour bénéficiaire du cycle
    │   │         → Libérer fonds au marchand
    │   │         → Passer au cycle N+1
    │   │
    │   └─► NON → Attendre (pas de relance automatique MVP)
    │
    └─► Dernier cycle ?
        └─► OUI → Statut COMPLETED


🎟️ Voucher de livraison
Caractéristiques :
Code unique de 12 caractères (ex: A3F9KL2M9X4P)
Validité : 6 mois
Utilisation : mono-boutique uniquement (sécurité)
Format : QR code + code alphanumérique
Sécurité :


// Dans RedeemVoucherUsecase
if voucher.ShopID != current_merchant.ShopID {
    return errors.New("ce bon de livraison appartient à une autre boutique")
}


6. Architecture technique
🏗️ Vue d'ensemble

┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES (HTTP)                        │
│  merchant_handler.go  │  client_handler.go  │  kyc_handler  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   APPLICATION (USE CASES)                    │
│  create_group  │  join_group  │  pay_cycle  │  redeem_voucher│
│  validate_kyc  │  complete_cycle  │  process_webhook         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                        DOMAIN (ENTITIES)                     │
│  TontineGroup  │  TontineParticipant  │  TontinePayment      │
│  TontineVoucher  │  CustomerKYCDocument                     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE                             │
│  Postgres  │  YengaPay Provider  │  Storage (fichiers KYC)  │
└─────────────────────────────────────────────────────────────┘


📦 Packages créés


domain/
├── entity/
│   ├── tontine.go                    ← Entités tontine
│   └── customer_kyc.go               ← Entité KYC
└── repository/
    ├── tontine_repository.go         ← Interfaces tontine
    └── customer_kyc_repository.go    ← Interface KYC

application/usecase/
├── tontine_usecase/
│   ├── create_group.go
│   ├── join_group.go
│   ├── pay_cycle.go
│   ├── complete_cycle.go
│   ├── redeem_voucher.go
│   └── process_webhook.go
└── customer_usecase/
    ├── upload_kyc.go
    └── review_kyc.go

infrastructure/
├── postgres/tontine/                 ← Implémentations Postgres
├── postgres/customer/                ← KYC repository
└── storage/local_storage.go          ← Stockage fichiers KYC

interfaces/handler/
├── tontine_handler/
│   ├── merchant_handler.go
│   └── client_handler.go
└── customer_handler/
    └── kyc_handler.go


7. Modèle de données
🗄️ Tables principales
product_tontine_settings


CREATE TABLE product_tontine_settings (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,

    is_tontine_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    allow_commercial_circle BOOLEAN NOT NULL DEFAULT TRUE,
    allow_corporate_circle BOOLEAN NOT NULL DEFAULT TRUE,
    allow_family_circle BOOLEAN NOT NULL DEFAULT TRUE,

    min_participants INT NOT NULL DEFAULT 4 CHECK (min_participants >= 2),
    max_participants INT NOT NULL DEFAULT 12 CHECK (max_participants <= 50),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


tontine_groups

CREATE TABLE tontine_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    creator_customer_id UUID REFERENCES customers(id),  -- NULL si créé par marchand
    creator_type VARCHAR(20) NOT NULL,  -- 'merchant' ou 'customer'

    circle_type VARCHAR(20) NOT NULL,  -- 'COMMERCIAL', 'CORPORATE', 'FAMILY'
    amount_per_cycle_cents BIGINT NOT NULL CHECK (amount_per_cycle_cents > 0),

    total_cycles INT NOT NULL CHECK (total_cycles >= 2),
    current_cycle INT NOT NULL DEFAULT 1,

    invite_code VARCHAR(10) UNIQUE NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING_MEMBERS',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT tontine_groups_circle_check
        CHECK (circle_type IN ('COMMERCIAL', 'CORPORATE', 'FAMILY')),
    CONSTRAINT tontine_groups_status_check
        CHECK (status IN ('PENDING_MEMBERS', 'ACTIVE', 'COMPLETED')),
    CONSTRAINT tontine_groups_creator_check
        CHECK (creator_type IN ('merchant', 'customer'))
);


tontine_participants

CREATE TABLE tontine_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id),

    payout_position INT NOT NULL CHECK (payout_position >= 1),
    status VARCHAR(20) NOT NULL DEFAULT 'active',

    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(group_id, customer_id),
    UNIQUE(group_id, payout_position),

    CONSTRAINT tontine_participants_status_check
        CHECK (status IN ('active', 'suspended', 'excluded'))
);


tontine_payments


CREATE TABLE tontine_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES tontine_participants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),

    cycle_number INT NOT NULL CHECK (cycle_number >= 1),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    commission_cents BIGINT NOT NULL,  -- Commission GoShop

    yengapay_reference VARCHAR(255),  -- Format: TONTINE:{groupID}:{cycle}:{participantID}
    yengapay_transaction_id VARCHAR(255),
    payment_provider VARCHAR(50) NOT NULL DEFAULT 'yenga_pay',

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    due_date TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(group_id, customer_id, cycle_number),

    CONSTRAINT tontine_payments_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED'))
);


tontine_vouchers

CREATE TABLE tontine_vouchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES tontine_groups(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES tontine_participants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    product_id UUID NOT NULL REFERENCES products(id),
    shop_id UUID NOT NULL REFERENCES shops(id),  -- 🆡 Sécurité mono-boutique

    voucher_code VARCHAR(20) UNIQUE NOT NULL,
    cycle_number INT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'generated',
    expires_at TIMESTAMPTZ NOT NULL,  -- NOW() + 6 mois
    redeemed_at TIMESTAMPTZ,
    redeemed_by UUID,  -- user_id du marchand

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(group_id, participant_id, cycle_number),

    CONSTRAINT tontine_vouchers_status_check
        CHECK (status IN ('generated', 'redeemed', 'expired', 'cancelled'))
);


customer_kyc_documents

CREATE TABLE customer_kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,

    document_type VARCHAR(20) NOT NULL,  -- 'cni', 'passport', 'other'
    file_path VARCHAR(500) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kyc_documents_type_check
        CHECK (document_type IN ('cni', 'passport', 'other')),
    CONSTRAINT kyc_documents_status_check
        CHECK (status IN ('pending', 'approved', 'rejected'))
);


Modifications sur customers et shop_payment_settings

-- Ajout KYC sur customers
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS kyc_level VARCHAR(20) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS kyc_validated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS kyc_validated_by UUID;

ALTER TABLE customers
    ADD CONSTRAINT customers_kyc_level_check
    CHECK (kyc_level IN ('none', 'pending', 'verified', 'rejected'));

-- Ajout config tontine sur shop_payment_settings
ALTER TABLE shop_payment_settings
    ADD COLUMN IF NOT EXISTS tontine_enabled BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS tontine_commission_rate INTEGER DEFAULT 250
        CHECK (tontine_commission_rate >= 0 AND tontine_commission_rate <= 1500);


8. API Endpoints
🔐 Endpoints KYC
Méthode
Endpoint
Acteur
Description
POST
/api/customers/kyc/upload
Client
Upload CNI/passeport
GET
/api/customers/kyc/status
Client
Voir statut KYC
GET
/api/merchant/kyc/pending
Marchand
Liste KYC en attente
GET
/api/merchant/kyc/{customer_id}/documents
Marchand
Voir documents
POST
/api/merchant/kyc/{customer_id}/approve
Marchand
Valider KYC
POST
/api/merchant/kyc/{customer_id}/reject
Marchand
Rejeter KYC
🏪 Endpoints Tontine (Marchand)
Méthode
Endpoint
Description
PUT
/api/products/{id}/tontine-settings
Activer/configurer tontine
GET
/api/products/{id}/tontine-settings
Lire config tontine
GET
/api/tontine/groups
Lister groupes de la boutique
GET
/api/tontine/groups/{id}
Détail d'un groupe
GET
/api/tontine/vouchers
Lister vouchers émis
POST
/api/tontine/vouchers/{code}/redeem
Valider un voucher
👤 Endpoints Tontine (Client)
Méthode
Endpoint
Description
GET
/api/client/tontine/products
Produits avec tontine dispo
POST
/api/client/tontine/groups
Créer un groupe
POST
/api/client/tontine/groups/join
Rejoindre par code
GET
/api/client/tontine/groups/{id}
Détail du groupe
GET
/api/client/tontine/groups/{id}/payments
Historique paiements
POST
/api/client/tontine/groups/{id}/pay
Initier paiement cycle
GET
/api/client/tontine/vouchers
Mes vouchers
Exemple de requête
Créer un groupe (client)


POST /api/client/tontine/groups
Authorization: Bearer {token}
X-Shop-Slug: boutique-moto-bobo
Content-Type: application/json

{
  "product_id": "bc7459fa-4368-4d51-a860-1bc19f9917ec",
  "circle_type": "FAMILY",
  "total_cycles": 8
}


Réponse 201 :


{
  "id": "33fb96c3-6819-411d-9be5-d2f196977127",
  "product_id": "bc7459fa-4368-4d51-a860-1bc19f9917ec",
  "shop_id": "ec4ff426-db05-421a-8b97-19c8470de0fb",
  "creator_customer_id": "834183ee-0f75-4178-8ff1-db8687d40a4b",
  "creator_type": "customer",
  "circle_type": "FAMILY",
  "amount_per_cycle_cents": 6250000,
  "total_cycles": 8,
  "current_cycle": 1,
  "invite_code": "A3F9KL2M",
  "status": "PENDING_MEMBERS",
  "created_at": "2026-06-28T14:30:22Z"
}

Rejoindre un groupe


POST /api/client/tontine/groups/join
Authorization: Bearer {token}
X-Shop-Slug: boutique-moto-bobo
Content-Type: application/json

{
  "invite_code": "A3F9KL2M"
}

Payer une cotisation
http

POST /api/client/tontine/groups/33fb96c3-6819-411d-9be5-d2f196977127/pay
Authorization: Bearer {token}
X-Shop-Slug: boutique-moto-bobo
Content-Type: application/json

{
  "operator": "orange_money",
  "phone_number": "+22670123456"
}


Réponse 201 :

{
  "payment_id": "50124a01-f42c-4aee-9beb-e8d27e5cd554",
  "amount_cents": 6250000,
  "commission_cents": 156250,
  "status": "PROCESSING",
  "ussd_code": "*144*4*6*62500#",
  "message": "Composez le code USSD pour initier le paiement"
}

Valider un voucher (marchand)

POST /api/tontine/vouchers/A3F9KL2M9X4P/redeem
Authorization: Bearer {token}
X-Shop-Slug: boutique-moto-bobo

Réponse 200 :


{
  "voucher_code": "A3F9KL2M9X4P",
  "customer_id": "834183ee-0f75-4178-8ff1-db8687d40a4b",
  "product_id": "bc7459fa-4368-4d51-a860-1bc19f9917ec",
  "cycle_number": 1,
  "status": "redeemed",
  "redeemed_at": "2026-07-15T10:30:00Z"
}


9. Intégration YengaPay
🔗 Format de référence
YengaPay ne supporte pas les métadonnées dans le webhook. On utilise le champ reference :


// Format : "TONTINE:{groupID_short}:{cycleNumber}:{participantID_short}"
reference := fmt.Sprintf("TONTINE:%s:%d:%s",
    groupID[:8],
    cycleNumber,
    participantID[:8])
// Exemple : "TONTINE:33fb96c3:1:834183ee"


🔄 Webhook handler

// Dans ProcessWebhookUsecase.Execute()
if strings.HasPrefix(payload.Reference, "TONTINE:") {
    parts := strings.Split(payload.Reference, ":")
    groupIDPrefix := parts[1]
    cycleNumber, _ := strconv.Atoi(parts[2])
    participantIDPrefix := parts[3]

    // 1. Calculer la commission
    commissionCents := payload.PaymentAmount * tontineCommissionRate / 100
    merchantAmount := payload.PaymentAmount - commissionCents

    // 2. Enregistrer le paiement tontine
    // 3. Vérifier si tous les participants ont payé
    // 4. Si oui → générer voucher + passer au cycle suivant
}


💸 Répartition des fonds

Client paie 62 500 F
    │
    ├─► YengaPay reçoit 62 500 F
    │
    ├─► Webhook envoyé à GoShop
    │
    ├─► GoShop calcule :
    │   ├─ Commission : 62 500 × 2.50% = 1 562 F
    │   └─ Marchand : 62 500 - 1 562 = 60 938 F
    │
    └─► Transfert vers marchand : 60 938 F

    10. Sécurité et conformité
🔐 Règles de sécurité
Règle
Implémentation
Multi-tenant
Toutes les routes passent par TenantResolver
Voucher mono-boutique
Check shop_id dans redeem_voucher
KYC obligatoire
Check kyc_level avant création/rejoint
Codes uniques
crypto/rand pour invite_code et voucher_code
Montants int64
Jamais de float, toujours en centimes
Audit trail
Logs zerolog sur toutes les actions
⚖️ Conformité BCEAO
Point d'attention : Collecter de l'argent pour livraison future peut nécessiter une autorisation BCEAO.
Recommandations :
✅ Consulter un conseil juridique avant lancement public
✅ Limiter le MVP à des cercles fermés (famille/amis)
✅ Pas de publicité publique avant validation juridique
✅ Documenter les flux financiers pour audit
🔒 Protection des documents KYC
Stockage local sécurisé (pas de cloud public pour MVP)
Accès restreint au marchand de la boutique
Suppression après validation (optionnel)
Pas de partage avec des tiers
11. Limites et décisions
✅ Décisions actées
Sujet
Décision
Raison
Types montants
BIGINT / int64 centimes
Précision financière
Types IDs
UUID partout
Cohérence avec le reste
Accès groupe
invite_code 8-10 chars
Cercle fermé
Position bénéficiaire
Ordre d'arrivée
Simple, transparent
Mode distribution
ROTATING uniquement au MVP
LOCKED_SAVINGS reporté
Livraison
Voucher numérique
Flexible, traçable
Validité voucher
6 mois
Standard marché
Utilisation voucher
Mono-boutique
Sécurité comptable
Commission
Par cotisation
Trésorerie lissée
Taux commission
0-15% configurable
Flexibilité marchand
Création groupe
Marchand + client
Flexibilité
KYC
Obligatoire pour tous
Sécurité
Paiement
Via YengaPay
6 opérateurs BF déjà intégrés
❌ Hors scope MVP
Fonctionnalité
Raison
Mode LOCKED_SAVINGS
Modèle économique flou
Gestion défauts de paiement
Confiance sociale au MVP
Notifications SMS
No-op au MVP, SMS plus tard
Relances automatiques
Pas de job scheduler
Assurance défaut
Complexe, reporté
Multi-devises
XOF uniquement au BF
API publique
Réservé aux clients/marchands
12. Roadmap
🚀 Phase 1 : MVP (2 semaines)
Semaine
Jours
Livrables
S1
J1-J2
Migration 010 + 011 (tontine + KYC)
J3
Entities + Repositories
J4-J5
Usecases (create, join, pay, webhook)
S2
J6-J7
Usecases (redeem, validate KYC)
J8
Handlers HTTP + routes
J9
Tests E2E complets
J10
Documentation + tag v2.9.0
🎯 Phase 2 : Améliorations (post-MVP)
Notifications SMS (Africa's Talking ou YengaPay SMS)
Dashboard marchand avancé (stats, graphiques)
Mode LOCKED_SAVINGS (avec prix négocié)
Liste d'attente pour groupes complets
Statistiques publiques (anonymisées)
🌟 Phase 3 : Scale
Application mobile React Native
Intégration Wave
Marketplace publique de tontines
API publique pour partenaires
📚 Références
Documentation YengaPay
Réglementation BCEAO
Architecture GoShop
Système de paiement
🤝 Contribution
Voir CONTRIBUTING.md pour les détails.
Prochaine étape : Implémentation de la Phase 1 (migration + entities + usecases).
Dernière mise à jour : 2026-06-28




