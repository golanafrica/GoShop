
---

## 📄 Fichier 3 : `docs/KYC.md` (NOUVEAU)

```markdown
# 🔐 Système KYC (Know Your Customer)

**Version** : v2.9.0  
**Date** : 2026-06-29  
**Statut** : ✅ Implémenté et testé

---

## 📋 Table des matières

1. [Vue d'ensemble](#1-vue-densemble)
2. [Workflow KYC](#2-workflow-kyc)
3. [Niveaux KYC](#3-niveaux-kyc)
4. [Types de documents](#4-types-de-documents)
5. [Architecture technique](#5-architecture-technique)
6. [Modèle de données](#6-modèle-de-données)
7. [API Endpoints](#7-api-endpoints)
8. [Sécurité et validation](#8-sécurité-et-validation)
9. [Intégration avec la Tontine](#9-intégration-avec-la-tontine)
10. [Tests E2E](#10-tests-e2e)
11. [Roadmap](#11-roadmap)

---

## 1. Vue d'ensemble

### 🎯 Qu'est-ce que le KYC ?

Le **KYC (Know Your Customer)** est un processus de vérification d'identité obligatoire pour les clients souhaitant participer au système de tontine. Il garantit que seuls des utilisateurs identifiés peuvent créer ou rejoindre des groupes de tontine.

### 💡 Pourquoi le KYC est nécessaire ?

| Raison | Explication |
|--------|-------------|
| **Conformité légale** | Respect des réglementations BCEAO |
| **Sécurité financière** | Prévention de la fraude |
| **Confiance sociale** | Les participants se connaissent |
| **Traçabilité** | Audit trail complet |
| **Protection marchand** | Le marchand sait à qui il livre |

### 🎯 Cas d'usage


Client veut rejoindre une tontine pour une moto
→ Upload CNI
→ Marchand vérifie l'identité
→ Client validé → peut rejoindre la tontine
→ Client participe aux cotisations
→ Client reçoit le voucher à son tour


---

## 2. Workflow KYC

### 📤 Côté client

┌─────────────────────────────────────────────────────────────┐
│ 1. Client arrive sur produit avec tontine │
│ → Voit "Créer une tontine" ou "Payer cash" │
│ → Clique → ❌ "Votre identité doit être vérifiée" │
└─────────────────────────────────────────────────────────────┘
│
▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Client upload document │
│ → CNI, passeport ou autre │
│ → Photo claire, max 5 Mo │
│ → Statut : pending │
└─────────────────────────────────────────────────────────────┘
│
▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Marchand reçoit notification │
│ → Voit les documents dans son dashboard │
│ → Vérifie l'identité (carte ID, téléphone, appel) │
│ → Clique "Valider KYC" ou "Rejeter" │
└─────────────────────────────────────────────────────────────┘
│
┌─────────────┴─────────────┐
▼ ▼
┌──────────────────┐ ┌──────────────────┐
│ ✅ verified │ │ ❌ rejected │
│ Peut participer │ │ Doit re-soumettre│
└──────────────────┘ └──────────────────┘


### 🏪 Côté marchand


Marchand (déjà verified via shop_id)
→ Dashboard → "KYC en attente"
→ Liste des clients avec documents
→ Vérifie chaque document
→ Approuve ou rejette avec raison
Si rejeté :
→ Client reçoit notification
→ Peut re-soumettre un nouveau document
→ Statut repasse à pending


---

## 3. Niveaux KYC

### 📊 États possibles

| Niveau | Description | Capabilities |
|--------|-------------|--------------|
| `none` | Compte créé, aucun document | ❌ Ne peut PAS participer à une tontine |
| `pending` | Documents uploadés, en attente | ❌ En attente de validation |
| `verified` | Identité vérifiée par le marchand | ✅ Peut créer/rejoindre une tontine |
| `rejected` | Documents rejetés | ❌ Doit re-soumettre |

### 🔄 Transitions d'état


none ──(upload)──► pending ──(approve)──► verified
│
└──(reject)──► rejected ──(re-upload)──► pending


### 💻 Implémentation dans le code

```go
// domain/entity/customer.go
type KYCLevel string

const (
    KYCLevelNone     KYCLevel = "none"
    KYCLevelPending  KYCLevel = "pending"
    KYCLevelVerified KYCLevel = "verified"
    KYCLevelRejected KYCLevel = "rejected"
)

// Vérification avant participation tontine
func (c *Customer) CanParticipateInTontine() error {
    if c.KYCLevel != KYCLevelVerified {
        return errors.New("votre identité doit être vérifiée")
    }
    return nil
}

4. Types de documents
📋 Documents acceptés

Type
Code
Description
CNI
cni
Carte Nationale d'Identité
Passeport
passport
Passeport biométrique
Autre
other
Permis de conduire, carte résident, etc.
📏 Contraintes de validation
Contrainte
Valeur
Raison
Taille max
5 Mo
Performance + stockage
Types MIME
image/jpeg, image/png, application/pdf
Formats standards
Max documents
3 par client
Éviter abus
Un par type
1 CNI + 1 passeport max
Pas de doublons
💻 Validation dans le code


// application/usecase/customer_usecase/upload_kyc.go
func (r *UploadKYCRequest) Validate() error {
    if r.FileSizeBytes > 5*1024*1024 {
        return fmt.Errorf("file size exceeds maximum (5 MB)")
    }
    
    allowedMimeTypes := map[string]bool{
        "image/jpeg":      true,
        "image/jpg":       true,
        "image/png":       true,
        "application/pdf": true,
    }
    if !allowedMimeTypes[r.MimeType] {
        return fmt.Errorf("mime type not allowed: %s", r.MimeType)
    }
    return nil
}


5. Architecture technique
🏗️ Architecture en couches

┌─────────────────────────────────────────────────────────────┐
│                    INTERFACES (HTTP)                          │
│  KYCHandler (UploadKYC, GetKYCStatus, ReviewKYC, ListPending)│
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   APPLICATION (USE CASES)                     │
│  UploadKYCDocumentUsecase  │  ReviewKYCUsecase               │
│  GetKYCStatusUsecase       │  ListPendingKYCUsecase          │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                        DOMAIN (ENTITIES)                      │
│  CustomerKYCDocument  │  Customer (avec KYCLevel)            │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE                             │
│  CustomerKYCRepository (Postgres)                            │
│  CustomerRepository (avec champs KYC)                        │
│  LocalStorage (fichiers KYC)                                 │
└─────────────────────────────────────────────────────────────┘

📦 Packages

domain/
├── entity/
│   ├── customer_kyc.go      ← Entité KYC + enums
│   └── customer.go          ← Champs KYCLevel ajoutés
└── repository/
    └── customer_kyc_repository.go  ← Interface

application/usecase/customer_usecase/
├── upload_kyc.go            ← Upload + validation
└── review_kyc.go            ← Validation/rejet marchand

infrastructure/postgres/customer/
└── kyc_repository.go        ← Implémentation Postgres

interfaces/handler/customer_handler/
└── kyc_handler.go           ← 4 endpoints HTTP

6. Modèle de données
🗄️ Table customer_kyc_documents

CREATE TABLE customer_kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    
    document_type VARCHAR(20) NOT NULL,  -- 'cni', 'passport', 'other'
    file_path VARCHAR(500) NOT NULL,
    file_size_bytes BIGINT NOT NULL CHECK (file_size_bytes > 0),
    mime_type VARCHAR(100) NOT NULL,
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by UUID,                    -- user_id du marchand
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT kyc_documents_type_check
        CHECK (document_type IN ('cni', 'passport', 'other')),
    CONSTRAINT kyc_documents_status_check
        CHECK (status IN ('pending', 'approved', 'rejected'))
);

-- Index de performance
CREATE INDEX idx_kyc_documents_customer ON customer_kyc_documents(customer_id);
CREATE INDEX idx_kyc_documents_shop ON customer_kyc_documents(shop_id);
CREATE INDEX idx_kyc_documents_status ON customer_kyc_documents(status);

🗄️ Modifications sur customers

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS kyc_level VARCHAR(20) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS kyc_validated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS kyc_validated_by UUID;

ALTER TABLE customers
    ADD CONSTRAINT customers_kyc_level_check
    CHECK (kyc_level IN ('none', 'pending', 'verified', 'rejected'));

🔗 Relations

customers (1) ──< (N) customer_kyc_documents
                      │
                      └── shop_id (multi-tenant)

7. API Endpoints
🔐 Endpoints Client
Upload document KYC

POST /api/customers/kyc/upload
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "customer_id": "uuid-customer",
  "document_type": "cni",
  "file_path": "/uploads/kyc/{customer_id}/cni.jpg",
  "file_size_bytes": 102400,
  "mime_type": "image/jpeg"
}

Réponse 201 :

{
  "id": "uuid-document",
  "customer_id": "uuid-customer",
  "shop_id": "uuid-shop",
  "document_type": "cni",
  "file_path": "/uploads/kyc/uuid-customer/cni.jpg",
  "file_size_bytes": 102400,
  "mime_type": "image/jpeg",
  "status": "pending",
  "created_at": "2026-06-29T10:00:00Z",
  "updated_at": "2026-06-29T10:00:00Z"
}

Voir statut KYC

GET /api/customers/{customer_id}/kyc/status
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Réponse 200 :

{
  "customer_id": "uuid-customer",
  "kyc_level": "verified",
  "documents": [
    {
      "id": "uuid-document",
      "document_type": "cni",
      "status": "approved",
      "reviewed_at": "2026-06-29T10:05:00Z"
    }
  ]
}

🏪 Endpoints Marchand
Liste des KYC en attente

GET /api/merchant/kyc/pending
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}

Réponse 200 :

[
  {
    "customer": {
      "id": "uuid-customer",
      "first_name": "Jean",
      "last_name": "Dupont",
      "email": "jean@example.com"
    },
    "documents": [
      {
        "id": "uuid-document",
        "document_type": "cni",
        "file_path": "/uploads/kyc/uuid-customer/cni.jpg",
        "status": "pending"
      }
    ]
  }
]

Valider/Rejeter KYC

POST /api/merchant/kyc/{customer_id}/review
Authorization: Bearer {token}
X-Shop-Slug: {shop_slug}
Content-Type: application/json

{
  "action": "approve"  // ou "reject"
}

Pour rejet, ajouter la raison :

{
  "action": "reject",
  "rejection_reason": "Photo floue, merci de renvoyer une photo claire"
}

Réponse 200 :

{
  "customer": {
    "id": "uuid-customer",
    "kyc_level": "verified"
  },
  "documents": [...],
  "new_kyc_level": "verified"
}

8. Sécurité et validation
🔐 Règles de sécurité
Règle
Implémentation
Multi-tenant
Tous les documents filtrent par shop_id
Validation taille
Max 5 Mo
Validation MIME
JPG, PNG, PDF uniquement
Limite documents
Max 3 par client
Un par type
Pas de doublon CNI/CNI
Audit trail
reviewed_by, reviewed_at, rejection_reason
Accès restreint
Seul le marchand du shop peut voir
✅ Validations effectuées


// 1. Validation de la requête
if err := req.Validate(); err != nil {
    return nil, fmt.Errorf("validation error: %w", err)
}

// 2. Vérification du nombre de documents
count, _ := kycDocRepo.CountByCustomer(ctx, req.CustomerID)
if count >= 3 {
    return nil, fmt.Errorf("maximum number of documents reached")
}

// 3. Vérification des doublons
pendingDocs, _ := kycDocRepo.FindPendingByCustomer(ctx, req.CustomerID)
for _, doc := range pendingDocs {
    if doc.DocumentType == req.DocumentType {
        return nil, fmt.Errorf("document already pending for this type")
    }
}

// 4. Transaction atomique
tx, _ := txManager.BeginTx(ctx)
defer tx.Rollback()
// ... opérations ...
tx.Commit()


🔒 Protection des documents
Stockage local : Pas de cloud public pour MVP
Accès restreint : Marchand du shop uniquement
Pas de partage : Documents non partagés avec tiers
Suppression optionnelle : Après validation si demandé
9. Intégration avec la Tontine
🔗 KYC obligatoire pour la tontine
go


// Dans CreateTontineGroupUsecase.Execute()
customer, err := uc.customerRepo.FindByCustomerID(ctx, req.CreatorCustomerID)
if err != nil {
    return nil, fmt.Errorf("customer not found")
}
if err := customer.CanParticipateInTontine(); err != nil {
    return nil, fmt.Errorf("customer cannot participate: %w", err)
}

📋 Workflow complet

1. Client veut créer un groupe tontine
   → Vérification KYC level
   → Si `none` ou `rejected` → ❌ Erreur
   → Si `pending` → ❌ "En attente de validation"
   → Si `verified` → ✅ Création autorisée

2. Client veut rejoindre un groupe
   → Même vérification KYC
   → Si non vérifié → ❌ Rejeté

3. Client participe aux cotisations
   → KYC déjà vérifié (vérifié à la création/rejointure)

🎯 Exemple concret

Client Jean veut rejoindre une tontine pour une moto
  │
  ├─► Upload CNI → statut `pending`
  ├─► Marchand vérifie → statut `verified`
  ├─► Rejoint groupe avec code d'invitation
  ├─► Paie 8 cotisations de 62 500 F
  └─► Reçoit voucher à son tour → va chercher la moto


10. Tests E2E
✅ Tests implémentés
Test
Description
Durée
TestTontineWorkflowE2E
Workflow complet avec KYC
~9s
TestTontineValidationRules
Validation taille/MIME
~9s
📋 Scénarios testés
TestTontineWorkflowE2E
✅ Création client + upload KYC
✅ Validation KYC par marchand
✅ Vérification statut verified
✅ Client vérifié peut rejoindre tontine
✅ Client non vérifié est rejeté (HTTP 400)
✅ Liste des KYC en attente
TestTontineValidationRules
✅ Upload > 5 Mo rejeté (HTTP 400)
✅ MIME type invalide rejeté (HTTP 400)
✅ Types MIME autorisés acceptés
🚀 Lancer les tests


# Tests KYC uniquement (via tontine)
go test ./tests/e2e/ -run TestTontine -v

# Tous les tests E2E
go test ./tests/e2e/ -v

11. Roadmap
✅ Phase 1 : MVP (v2.9.0 - Terminé)
Upload documents (CNI, passeport, autre)
Validation par marchand
Rejet avec raison
Statuts KYC (none, pending, verified, rejected)
Intégration avec tontine
Tests E2E complets
🎯 Phase 2 : Améliorations
OCR automatique (extraction données CNI)
Vérification biométrique (selfie + CNI)
Notifications SMS/email au client
Dashboard KYC avancé (stats, filtres)
Historique complet des validations
Export des documents (PDF)
🌟 Phase 3 : Automatisation
Intégration service d'identité national
Vérification automatique (IA)
Détection de fraude (deepfake, falsification)
KYC niveau 2 (vérification renforcée)
Conformité AML (Anti-Money Laundering)
📚 Références
Réglementation BCEAO
Système de Tontine
Architecture GoShop
Système de paiement
🤝 Contribution
Voir CONTRIBUTING.md pour les détails.
Dernière mise à jour : 2026-06-29


