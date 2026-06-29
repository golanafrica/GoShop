# Système de paiement par Tontine

## Vue d'ensemble

La tontine est un système d'épargne collective profondément ancré dans la culture ouest-africaine. Elle permet à un groupe de personnes de cotiser régulièrement et de retirer des fonds pour effectuer des achats.

Dans GoShop, chaque boutique peut activer une ou plusieurs tontines pour ses clients. Les clients cotisent, accumulent un solde, et peuvent utiliser ce solde pour payer leurs commandes.

---

## Pourquoi intégrer la tontine ?

| Bénéfice | Description |
|----------|-------------|
| **Fidélisation** | Les clients cotisent régulièrement → ils reviennent acheter |
| **Ventes** | Les clients ont de l'épargne disponible → ils achètent plus |
| **Avantage concurrentiel** | Aucun concurrent local n'offre cette fonctionnalité |
| **Culturel** | Répond à une pratique financière répandue en Afrique de l'Ouest |
| **Confiance** | Le marchand connaît ses clients (membres de la tontine) |

---

## Catégories de tontines

Pour répondre aux besoins spécifiques des clients, chaque boutique peut créer des **tontines thématiques** dédiées à des catégories de produits.

| Catégorie | Icône | Exemples de produits | Cotisation typique |
|-----------|-------|---------------------|-------------------|
| **Moto** | 🏍️ | Motos, casques, pièces détachées | 25 000 - 50 000 FCFA/mois |
| **Voiture** | 🚗 | Voitures, entretien, assurance | 50 000 - 100 000 FCFA/mois |
| **Ciment** | 🏗️ | Ciment, fer à béton, matériaux | 10 000 - 20 000 FCFA/mois |
| **Électroménager** | 📺 | Réfrigérateurs, téléviseurs, climatiseurs | 15 000 - 30 000 FCFA/mois |
| **Téléphonie** | 📱 | Smartphones, accessoires | 10 000 - 20 000 FCFA/mois |
| **Mode** | 👗 | Vêtements, chaussures, bijoux | 5 000 - 15 000 FCFA/mois |
| **Alimentation** | 🍲 | Produits alimentaires | 5 000 - 10 000 FCFA/mois |
| **Éducation** | 📚 | Fournitures scolaires, ordinateurs | 10 000 - 20 000 FCFA/mois |
| **Santé** | 💊 | Médicaments, consultations | 10 000 - 25 000 FCFA/mois |
| **Générale** | 💰 | Tous les produits | 10 000 - 30 000 FCFA/mois |

---

## Modèle de données

### Tables principales

```sql
-- ============================================
-- 1. Tontine (gérée par le marchand)
-- ============================================
CREATE TABLE tontines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Catégorie thématique
    category VARCHAR(50) NOT NULL DEFAULT 'general',
    category_icon VARCHAR(50),
    category_color VARCHAR(20),
    
    -- Objectif
    savings_goal BIGINT, -- Objectif total en FCFA
    target_product_id UUID REFERENCES products(id), -- Produit cible
    
    -- Règles
    contribution_amount BIGINT NOT NULL,
    contribution_frequency VARCHAR(20) NOT NULL, -- weekly, biweekly, monthly
    contribution_day INTEGER,
    max_members INTEGER DEFAULT 20,
    min_members INTEGER DEFAULT 5,
    min_savings_to_withdraw BIGINT DEFAULT 0,
    max_withdraw_percentage INTEGER DEFAULT 80,
    withdrawal_fee_pct DECIMAL(5,2) DEFAULT 0.00,
    
    -- Gestion
    status VARCHAR(20) DEFAULT 'active', -- active, paused, closed
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- 2. Membres de la tontine
-- ============================================
CREATE TABLE tontine_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    
    role VARCHAR(20) DEFAULT 'member', -- member, treasurer, admin
    balance BIGINT DEFAULT 0,
    total_contributions BIGINT DEFAULT 0,
    total_withdrawals BIGINT DEFAULT 0,
    
    status VARCHAR(20) DEFAULT 'active', -- active, inactive, blacklisted
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    last_contribution_at TIMESTAMPTZ,
    
    UNIQUE(tontine_id, customer_id)
);

-- ============================================
-- 3. Objectifs personnalisés par membre
-- ============================================
CREATE TABLE tontine_member_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES tontine_members(id) ON DELETE CASCADE,
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    
    target_amount BIGINT NOT NULL,
    progress_percentage DECIMAL(5,2) DEFAULT 0.00,
    target_product_id UUID REFERENCES products(id),
    notes TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- 4. Cotisations
-- ============================================
CREATE TABLE tontine_contributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES tontine_members(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id),
    
    amount BIGINT NOT NULL,
    payment_method VARCHAR(50) NOT NULL, -- cash, wave, orange_money
    payment_ref VARCHAR(255),
    
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    
    status VARCHAR(20) DEFAULT 'completed', -- pending, completed, failed
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- 5. Retraits (utilisés pour payer les commandes)
-- ============================================
CREATE TABLE tontine_withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES tontine_members(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES customers(id),
    order_id UUID NOT NULL REFERENCES orders(id),
    
    amount BIGINT NOT NULL,
    fee BIGINT DEFAULT 0,
    net_amount BIGINT NOT NULL,
    
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected, completed
    approved_by UUID REFERENCES users(id),
    notes TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- ============================================
-- 6. Produits éligibles
-- ============================================
CREATE TABLE tontine_product_eligibility (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    eligible BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(tontine_id, product_id)
);

-- ============================================
-- 7. Annonces de la tontine
-- ============================================
CREATE TABLE tontine_announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tontine_id UUID NOT NULL REFERENCES tontines(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    
    type VARCHAR(50) DEFAULT 'info', -- info, success, alert, promotion
    is_pinned BOOLEAN DEFAULT false,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

-- ============================================
-- 8. Paramètres tontine par boutique
-- ============================================
CREATE TABLE shop_tontine_settings (
    shop_id UUID PRIMARY KEY REFERENCES shops(id),
    
    enabled BOOLEAN DEFAULT false,
    default_contribution_amount BIGINT DEFAULT 10000,
    default_contribution_frequency VARCHAR(20) DEFAULT 'monthly',
    default_max_members INTEGER DEFAULT 20,
    default_min_members INTEGER DEFAULT 5,
    default_max_withdraw_percentage INTEGER DEFAULT 80,
    default_withdrawal_fee_pct DECIMAL(5,2) DEFAULT 0.00,
    
    withdrawal_requires_approval BOOLEAN DEFAULT true,
    allow_negative_balance BOOLEAN DEFAULT false,
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- INDEX
-- ============================================
CREATE INDEX idx_tontines_shop ON tontines(shop_id);
CREATE INDEX idx_tontines_category ON tontines(category);
CREATE INDEX idx_tontines_shop_category ON tontines(shop_id, category);
CREATE INDEX idx_tontine_members_customer ON tontine_members(customer_id);
CREATE INDEX idx_tontine_contributions_member ON tontine_contributions(member_id);
CREATE INDEX idx_tontine_withdrawals_order ON tontine_withdrawals(order_id);
CREATE INDEX idx_tontine_eligibility_product ON tontine_product_eligibility(product_id);
CREATE INDEX idx_tontine_member_goals_member ON tontine_member_goals(member_id);

Plan d'implémentation
Étape	Priorité	Description
1	🔴 Haute	Migrations SQL (8 tables)
2	🔴 Haute	Domaines (domain/tontine/)
3	🔴 Haute	Repositories (domain/repository/tontine_*.go)
4	🔴 Haute	Use Case CreateTontine
5	🔴 Haute	Use Case JoinTontine
6	🔴 Haute	Use Case Contribute
7	🔴 Haute	Use Case WithdrawFromTontine
8	🟡 Moyenne	Use Case ListTontinesByCategory
9	🟡 Moyenne	Use Case ApproveWithdrawal
10	🟢 Basse	Handlers (marchand)
11	🟢 Basse	Handlers (client)
12	🟢 Basse	Dashboard marchand (frontend)
13	🟢 Basse	Interface client (frontend)
FAQ
Q: Un client peut-il rejoindre plusieurs tontines ?
R: Oui, un client peut être membre de plusieurs tontines dans la même boutique (ex: tontine Moto + tontine Ciment).

Q: Les produits éligibles sont-ils obligatoires ?
R: Non, si aucun produit n'est spécifié, tous les produits de la boutique sont éligibles.

Q: Le client peut-il changer d'objectif en cours de route ?
R: Oui, il peut modifier son objectif personnel à tout moment depuis son espace client.

Q: Que se passe-t-il si le client atteint son objectif avant la fin ?
R: Il peut utiliser son solde immédiatement pour acheter le produit cible.

Q: Le marchand peut-il modifier les règles en cours de route ?
R: Oui, mais les changements s'appliquent uniquement aux nouvelles cotisations.

Q: Que devient le solde si la tontine est fermée ?
R: Les membres peuvent retirer leur solde avant la fermeture. Les soldes non retirés sont remboursés.

text
---

## 📄 Document 2 : `/docs/12-tontine-api-reference.md`

```markdown
# API Reference - Tontine

## Base URL
- **Développement** : `http://localhost:8080`
- **Production** : `https://api.golanafrica.com`

## Authentification
- **JWT** : Access token (15min), Refresh token (7j)
- **Headers** : `Authorization: Bearer <access_token>`

---

## Routes Marchand (Dashboard)

### 1. Paramètres de la tontine

#### GET `/api/dashboard/tontine/settings`
Récupérer les paramètres de la tontine pour la boutique.

**Réponse** :
```json
{
    "enabled": true,
    "default_contribution_amount": 10000,
    "default_contribution_frequency": "monthly",
    "default_max_members": 20,
    "default_min_members": 5,
    "default_max_withdraw_percentage": 80,
    "default_withdrawal_fee_pct": 0.00,
    "withdrawal_requires_approval": true,
    "allow_negative_balance": false,
    "updated_at": "2026-06-17T10:00:00Z"
}
PUT /api/dashboard/tontine/settings
Mettre à jour les paramètres de la tontine.

Requête :

json
{
    "enabled": true,
    "default_contribution_amount": 15000,
    "default_contribution_frequency": "monthly",
    "default_max_members": 25,
    "default_min_members": 5,
    "default_max_withdraw_percentage": 85,
    "default_withdrawal_fee_pct": 0.50,
    "withdrawal_requires_approval": false,
    "allow_negative_balance": false
}
Réponse :

json
{
    "status": "updated"
}
2. Gestion des tontines
GET /api/dashboard/tontine
Liste des tontines de la boutique.

Query params :

category (optionnel) : filtre par catégorie

status (optionnel) : active, paused, closed

Réponse :

json
[
    {
        "id": "uuid",
        "name": "Tontine Moto",
        "category": "moto",
        "category_icon": "🏍️",
        "savings_goal": 500000,
        "target_product": {
            "id": "uuid",
            "name": "TVS Apache 150"
        },
        "contribution_amount": 25000,
        "contribution_frequency": "monthly",
        "max_members": 10,
        "min_members": 3,
        "current_members": 5,
        "total_collected": 375000,
        "progress_percentage": 75,
        "status": "active",
        "created_at": "2026-06-01T10:00:00Z"
    }
]
POST /api/dashboard/tontine
Créer une nouvelle tontine.

Requête :

json
{
    "name": "Tontine Moto",
    "description": "Pour acheter une moto TVS Apache",
    "category": "moto",
    "category_icon": "🏍️",
    "category_color": "#FF6B00",
    "savings_goal": 500000,
    "target_product_id": "product-uuid",
    "contribution_amount": 25000,
    "contribution_frequency": "monthly",
    "contribution_day": 5,
    "max_members": 10,
    "min_members": 3,
    "min_savings_to_withdraw": 50000,
    "max_withdraw_percentage": 80,
    "withdrawal_fee_pct": 0.00,
    "eligible_product_ids": ["product-uuid-1", "product-uuid-2"]
}
Réponse :

json
{
    "id": "uuid",
    "name": "Tontine Moto",
    "category": "moto",
    "savings_goal": 500000,
    "status": "active",
    "created_at": "2026-06-17T10:00:00Z"
}
GET /api/dashboard/tontine/{id}
Détail d'une tontine.

Réponse :

json
{
    "id": "uuid",
    "name": "Tontine Moto",
    "description": "Pour acheter une moto TVS Apache",
    "category": "moto",
    "category_icon": "🏍️",
    "savings_goal": 500000,
    "target_product": {
        "id": "uuid",
        "name": "TVS Apache 150",
        "price": 500000
    },
    "contribution_amount": 25000,
    "contribution_frequency": "monthly",
    "contribution_day": 5,
    "max_members": 10,
    "min_members": 3,
    "current_members": 5,
    "total_collected": 375000,
    "progress_percentage": 75,
    "status": "active",
    "started_at": "2026-06-01T10:00:00Z",
    "eligible_products": [
        {
            "id": "uuid",
            "name": "TVS Apache 150",
            "price": 500000
        },
        {
            "id": "uuid",
            "name": "TVS Star HLX",
            "price": 450000
        }
    ],
    "members": [
        {
            "customer_id": "uuid",
            "name": "Fatouma Coulibaly",
            "balance": 200000,
            "progress": 80,
            "joined_at": "2026-06-01T10:00:00Z"
        }
    ],
    "announcements": [
        {
            "id": "uuid",
            "title": "Nouvelle tontine !",
            "content": "Rejoignez la tontine moto",
            "type": "info",
            "is_pinned": true,
            "created_at": "2026-06-01T10:00:00Z"
        }
    ]
}
PUT /api/dashboard/tontine/{id}
Modifier une tontine.

Requête : (même structure que POST, champs optionnels)

Réponse :

json
{
    "status": "updated"
}
POST /api/dashboard/tontine/{id}/toggle
Activer/désactiver une tontine.

Requête :

json
{
    "status": "paused" // ou "active", "closed"
}
Réponse :

json
{
    "status": "paused"
}
DELETE /api/dashboard/tontine/{id}
Supprimer une tontine (uniquement si solde = 0).

Réponse :

json
{
    "status": "deleted"
}
3. Gestion des membres
GET /api/dashboard/tontine/{id}/members
Liste des membres d'une tontine.

Réponse :

json
[
    {
        "id": "uuid",
        "customer_id": "uuid",
        "name": "Fatouma Coulibaly",
        "phone": "+226 70 00 00 00",
        "balance": 200000,
        "total_contributions": 250000,
        "total_withdrawals": 50000,
        "progress": 80,
        "role": "member",
        "status": "active",
        "joined_at": "2026-06-01T10:00:00Z"
    }
]
POST /api/dashboard/tontine/{id}/members
Ajouter un membre manuellement (cash).

Requête :

json
{
    "customer_id": "uuid",
    "initial_balance": 50000,
    "notes": "Cotisation cash enregistrée"
}
Réponse :

json
{
    "member_id": "uuid",
    "balance": 50000,
    "joined_at": "2026-06-17T10:00:00Z"
}
POST /api/dashboard/tontine/members/{id}/blacklist
Mettre un membre en liste noire.

Réponse :

json
{
    "status": "blacklisted"
}
4. Gestion des retraits
GET /api/dashboard/tontine/withdrawals/pending
Retraits en attente d'approbation.

Réponse :

json
[
    {
        "id": "uuid",
        "customer_name": "Fatouma Coulibaly",
        "amount": 45000,
        "net_amount": 45000,
        "order_id": "uuid",
        "order_total": 120000,
        "remaining_to_pay": 75000,
        "created_at": "2026-06-17T10:00:00Z"
    }
]
POST /api/dashboard/tontine/withdrawals/{id}/approve
Approuver un retrait.

Réponse :

json
{
    "status": "approved",
    "approved_at": "2026-06-17T10:00:00Z"
}
POST /api/dashboard/tontine/withdrawals/{id}/reject
Refuser un retrait.

Requête :

json
{
    "reason": "Solde insuffisant"
}
Réponse :

json
{
    "status": "rejected"
}
5. Gestion des produits éligibles
POST /api/dashboard/tontine/{id}/eligibility
Ajouter des produits éligibles.

Requête :

json
{
    "product_ids": ["uuid-1", "uuid-2"]
}
Réponse :

json
{
    "added": 2
}
DELETE /api/dashboard/tontine/{id}/eligibility/{productId}
Retirer un produit éligible.

Réponse :

json
{
    "status": "removed"
}
6. Annonces
POST /api/dashboard/tontine/{id}/announcements
Créer une annonce.

Requête :

json
{
    "title": "Nouvelle promotion !",
    "content": "Cotisez maintenant et bénéficiez de 5% de réduction",
    "type": "promotion",
    "is_pinned": true,
    "expires_at": "2026-07-01T00:00:00Z"
}
Réponse :

json
{
    "id": "uuid",
    "title": "Nouvelle promotion !",
    "created_at": "2026-06-17T10:00:00Z"
}
DELETE /api/dashboard/tontine/announcements/{id}
Supprimer une annonce.

Réponse :

json
{
    "status": "deleted"
}
Routes Client
1. Découverte des tontines
GET /api/client/tontine
Liste des tontines disponibles pour le client.

Query params :

category (optionnel) : filtre par catégorie

Réponse :

json
[
    {
        "id": "uuid",
        "name": "Tontine Moto",
        "category": "moto",
        "category_icon": "🏍️",
        "description": "Pour acheter une moto TVS Apache",
        "savings_goal": 500000,
        "contribution_amount": 25000,
        "contribution_frequency": "monthly",
        "max_members": 10,
        "current_members": 5,
        "total_collected": 375000,
        "progress_percentage": 75,
        "has_joined": false,
        "created_at": "2026-06-01T10:00:00Z"
    }
]
GET /api/client/tontine/{id}
Détail d'une tontine.

Réponse : (identique à la version marchand, mais sans les données sensibles)

2. Adhésion
POST /api/client/tontine/{id}/join
Rejoindre une tontine.

Réponse :

json
{
    "member_id": "uuid",
    "balance": 0,
    "joined_at": "2026-06-17T10:00:00Z"
}
POST /api/client/tontine/{id}/leave
Quitter une tontine (si solde = 0).

Réponse :

json
{
    "status": "left"
}
3. Cotisations
POST /api/client/tontine/{id}/contribute
Cotiser à la tontine.

Requête :

json
{
    "amount": 25000,
    "payment_method": "wave", // cash, wave, orange_money
    "payment_ref": "WV-2847361"
}
Réponse :

json
{
    "contribution_id": "uuid",
    "new_balance": 25000,
    "total_contributions": 25000,
    "progress_percentage": 5,
    "created_at": "2026-06-17T10:00:00Z"
}
GET /api/client/tontine/{id}/contributions
Historique des cotisations.

Réponse :

json
[
    {
        "id": "uuid",
        "amount": 25000,
        "payment_method": "wave",
        "period_start": "2026-06-01",
        "period_end": "2026-07-01",
        "status": "completed",
        "created_at": "2026-06-01T10:00:00Z"
    }
]
4. Retraits (pour payer)
POST /api/client/tontine/{id}/withdraw
Demander un retrait pour payer une commande.

Requête :

json
{
    "order_id": "uuid",
    "amount": 45000,
    "notes": "Achat Samsung A54"
}
Réponse (si approbation requise) :

json
{
    "withdrawal_id": "uuid",
    "status": "pending",
    "balance_after": 25000,
    "fee": 0,
    "net_amount": 45000,
    "requires_approval": true,
    "created_at": "2026-06-17T10:00:00Z"
}
Réponse (si approbation non requise) :

json
{
    "withdrawal_id": "uuid",
    "status": "completed",
    "balance_after": 25000,
    "fee": 0,
    "net_amount": 45000,
    "requires_approval": false,
    "completed_at": "2026-06-17T10:00:00Z"
}
GET /api/client/tontine/withdrawals
Historique des retraits.

Réponse :

json
[
    {
        "id": "uuid",
        "amount": 45000,
        "net_amount": 45000,
        "status": "completed",
        "order_id": "uuid",
        "order_total": 120000,
        "created_at": "2026-06-17T10:00:00Z",
        "completed_at": "2026-06-17T10:30:00Z"
    }
]
5. Objectifs personnels
GET /api/client/tontine/{id}/goal
Récupérer l'objectif personnel.

Réponse :

json
{
    "target_amount": 500000,
    "progress_percentage": 40,
    "target_product": {
        "id": "uuid",
        "name": "TVS Apache 150",
        "price": 500000
    },
    "notes": "Je veux cette moto pour septembre",
    "updated_at": "2026-06-17T10:00:00Z"
}
POST /api/client/tontine/{id}/goal
Définir ou modifier l'objectif personnel.

Requête :

json
{
    "target_amount": 500000,
    "target_product_id": "uuid",
    "notes": "Je veux cette moto pour septembre"
}
Réponse :

json
{
    "target_amount": 500000,
    "progress_percentage": 40,
    "updated_at": "2026-06-17T10:00:00Z"
}
6. Tableau de bord client
GET /api/client/tontine/dashboard
Vue d'ensemble des tontines du client.

Réponse :

json
{
    "total_tontines": 2,
    "total_balance": 320000,
    "total_contributions": 350000,
    "total_withdrawals": 30000,
    "tontines": [
        {
            "id": "uuid",
            "name": "Tontine Moto",
            "category": "moto",
            "balance": 200000,
            "progress": 80,
            "next_contribution": {
                "amount": 25000,
                "due_date": "2026-07-05"
            }
        },
        {
            "id": "uuid",
            "name": "Tontine Ciment",
            "category": "ciment",
            "balance": 120000,
            "progress": 40,
            "next_contribution": {
                "amount": 15000,
                "due_date": "2026-07-10"
            }
        }
    ]
}
text

---

## 📄 Document 3 : `/docs/13-tontine-frontend.md`

```markdown
# Frontend - Tontine

## Structure des pages Next.js
/apps/
├── dashboard/ # Dashboard marchand
│ └── app/
│ └── (dashboard)/
│ └── tontine/
│ ├── page.tsx # Liste des tontines
│ ├── create/
│ │ └── page.tsx # Création d'une tontine
│ ├── [id]/
│ │ ├── page.tsx # Détail d'une tontine
│ │ ├── edit/
│ │ │ └── page.tsx # Édition
│ │ └── members/
│ │ └── page.tsx # Gestion des membres
│ └── withdrawals/
│ └── pending/
│ └── page.tsx # Retraits en attente
│
└── storefront/ # Storefront client
└── app/
└── shop/
└── [slug]/
└── tontine/
├── page.tsx # Liste des tontines
├── [id]/
│ ├── page.tsx # Détail
│ └── checkout/
│ └── page.tsx # Payer avec tontine
└── profile/
└── page.tsx # Mes tontines

text

---

## Composants clés

### 1. Dashboard marchand - Liste des tontines

```tsx
// apps/dashboard/app/(dashboard)/tontine/page.tsx
'use client';

import { useState, useEffect } from 'react';
import { useTontine } from '@/hooks/useTontine';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import Link from 'next/link';

export default function TontineListPage() {
  const { tontines, isLoading, fetchTontines } = useTontine();
  const [category, setCategory] = useState('all');

  useEffect(() => {
    fetchTontines(category);
  }, [category]);

  if (isLoading) return <div>Chargement...</div>;

  return (
    <div className="container mx-auto p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">💰 Gestion des Tontines</h1>
        <Link href="/tontine/create">
          <Button>➕ Créer une tontine</Button>
        </Link>
      </div>

      <Tabs defaultValue="all" onValueChange={setCategory}>
        <TabsList>
          <TabsTrigger value="all">Toutes</TabsTrigger>
          <TabsTrigger value="moto">🏍️ Moto</TabsTrigger>
          <TabsTrigger value="voiture">🚗 Voiture</TabsTrigger>
          <TabsTrigger value="ciment">🏗️ Ciment</TabsTrigger>
          <TabsTrigger value="telephonie">📱 Téléphonie</TabsTrigger>
          <TabsTrigger value="general">💰 Générale</TabsTrigger>
        </TabsList>

        <TabsContent value="all">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-4">
            {tontines.map((tontine) => (
              <TontineCard key={tontine.id} tontine={tontine} />
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function TontineCard({ tontine }: { tontine: any }) {
  return (
    <Link href={`/tontine/${tontine.id}`}>
      <Card className="hover:shadow-lg transition-shadow cursor-pointer">
        <CardHeader>
          <div className="flex justify-between items-start">
            <div>
              <span className="text-2xl mr-2">{tontine.category_icon}</span>
              <CardTitle>{tontine.name}</CardTitle>
            </div>
            <StatusBadge status={tontine.status} />
          </div>
          <p className="text-sm text-gray-500">{tontine.description}</p>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <span>Objectif</span>
              <span className="font-semibold">
                {tontine.total_collected.toLocaleString()} / {tontine.savings_goal.toLocaleString()} FCFA
              </span>
            </div>
            <Progress value={tontine.progress_percentage} className="h-2" />
            
            <div className="flex justify-between text-sm mt-4">
              <span>👥 {tontine.current_members}/{tontine.max_members} membres</span>
              <span>💰 {tontine.contribution_amount.toLocaleString()} FCFA/mois</span>
            </div>

            <Button variant="outline" size="sm" className="w-full mt-2">
              Voir les détails
            </Button>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
2. Client - Payer avec la tontine
tsx
// apps/storefront/app/shop/[slug]/tontine/[id]/checkout/page.tsx
'use client';

import { useState, useEffect } from 'react';
import { useTontine } from '@/hooks/useTontine';
import { useCart } from '@/hooks/useCart';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Slider } from '@/components/ui/slider';
import { Alert, AlertDescription } from '@/components/ui/alert';

export default function TontineCheckoutPage({ params }: { params: { slug: string; id: string } }) {
  const { tontine, balance, maxWithdraw, isLoading, withdraw } = useTontine(params.id);
  const { cart, total } = useCart();
  const [amount, setAmount] = useState(0);
  const [remaining, setRemaining] = useState(0);

  useEffect(() => {
    if (tontine && balance) {
      const max = Math.min(balance, total);
      setAmount(Math.min(max, maxWithdraw));
      setRemaining(total - amount);
    }
  }, [tontine, balance, total]);

  const handleWithdraw = async () => {
    const result = await withdraw({
      order_id: orderId,
      amount: amount,
      notes: `Achat ${cart.length} articles`
    });

    if (result.requires_approval) {
      router.push(`/orders/${orderId}/pending`);
    } else {
      router.push(`/orders/${orderId}/success`);
    }
  };

  if (isLoading) return <div>Chargement...</div>;

  return (
    <div className="max-w-2xl mx-auto p-6">
      <h1 className="text-2xl font-bold mb-6">Payer avec la tontine</h1>

      <Card className="mb-6">
        <CardContent className="p-6">
          <div className="flex justify-between items-center mb-4">
            <div>
              <span className="text-2xl mr-2">{tontine.category_icon}</span>
              <span className="font-semibold">{tontine.name}</span>
            </div>
            <span className="text-sm text-gray-500">
              Solde : {balance.toLocaleString()} FCFA
            </span>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">
                Montant à utiliser : {amount.toLocaleString()} FCFA
              </label>
              <Slider
                value={[amount]}
                min={0}
                max={Math.min(balance, total)}
                step={1000}
                onValueChange={(value) => setAmount(value[0])}
                className="w-full"
              />
              <div className="flex justify-between text-xs text-gray-500 mt-1">
                <span>0 FCFA</span>
                <span>{Math.min(balance, total).toLocaleString()} FCFA</span>
              </div>
            </div>

            {tontine.withdrawal_fee_pct > 0 && (
              <div className="text-sm text-gray-600">
                Frais de retrait : {tontine.withdrawal_fee_pct}% (
                {(amount * tontine.withdrawal_fee_pct / 100).toLocaleString()} FCFA)
              </div>
            )}

            {remaining > 0 && (
              <Alert>
                <AlertDescription>
                  Reste à payer : <strong>{remaining.toLocaleString()} FCFA</strong>
                  <br />
                  <span className="text-sm">
                    Vous pourrez payer le reste avec Wave ou Cash à la livraison.
                  </span>
                </AlertDescription>
              </Alert>
            )}

            {remaining === 0 && (
              <Alert className="bg-green-50 border-green-200">
                <AlertDescription className="text-green-700">
                  ✅ Vous avez suffisamment sur votre tontine pour payer cette commande !
                </AlertDescription>
              </Alert>
            )}
          </div>
        </CardContent>
      </Card>

      <div className="flex gap-4">
        <Button variant="outline" className="flex-1" onClick={() => router.back()}>
          Retour
        </Button>
        <Button 
          className="flex-1 bg-green-600 hover:bg-green-700"
          onClick={handleWithdraw}
          disabled={amount === 0}
        >
          {amount === 0 ? 'Choisissez un montant' : 'Confirmer le paiement'}
        </Button>
      </div>
    </div>
  );
}
3. Dashboard marchand - Approbation des retraits
tsx
// apps/dashboard/app/(dashboard)/tontine/withdrawals/pending/page.tsx
'use client';

import { useState, useEffect } from 'react';
import { useTontine } from '@/hooks/useTontine';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { CheckCircle, XCircle, Clock } from 'lucide-react';

export default function PendingWithdrawalsPage() {
  const { pendingWithdrawals, approveWithdrawal, rejectWithdrawal } = useTontine();
  const [selected, setSelected] = useState<any>(null);

  useEffect(() => {
    fetchPendingWithdrawals();
  }, []);

  const handleApprove = async (id: string) => {
    await approveWithdrawal(id);
    fetchPendingWithdrawals();
  };

  const handleReject = async (id: string) => {
    await rejectWithdrawal(id);
    fetchPendingWithdrawals();
  };

  return (
    <div className="container mx-auto p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">📋 Retraits en attente</h1>
        <Badge variant="outline" className="text-sm">
          {pendingWithdrawals.length} en attente
        </Badge>
      </div>

      {pendingWithdrawals.length === 0 ? (
        <Card>
          <CardContent className="p-12 text-center">
            <Clock className="w-12 h-12 mx-auto text-gray-400 mb-4" />
            <p className="text-gray-500">Aucun retrait en attente d'approbation</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {pendingWithdrawals.map((withdrawal) => (
            <Card key={withdrawal.id} className="hover:shadow-md transition-shadow">
              <CardContent className="p-6">
                <div className="flex justify-between items-start">
                  <div>
                    <div className="flex items-center gap-3">
                      <h3 className="font-semibold text-lg">{withdrawal.customer_name}</h3>
                      <Badge variant="outline" className="text-yellow-600 border-yellow-200 bg-yellow-50">
                        <Clock className="w-3 h-3 mr-1" />
                        En attente
                      </Badge>
                    </div>
                    <p className="text-sm text-gray-500 mt-1">
                      Commande #{withdrawal.order_id.slice(0, 8)}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-xl font-bold text-green-600">
                      {withdrawal.amount.toLocaleString()} FCFA
                    </p>
                    <p className="text-sm text-gray-500">
                      Solde restant : {withdrawal.balance_after.toLocaleString()} FCFA
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4 mt-4 text-sm">
                  <div>
                    <span className="text-gray-500">Produit</span>
                    <p className="font-medium">{withdrawal.product_name}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">Total commande</span>
                    <p className="font-medium">{withdrawal.order_total.toLocaleString()} FCFA</p>
                  </div>
                  <div>
                    <span className="text-gray-500">Reste à payer</span>
                    <p className="font-medium">{withdrawal.remaining_to_pay.toLocaleString()} FCFA</p>
                  </div>
                </div>

                {withdrawal.notes && (
                  <p className="text-sm text-gray-500 mt-2">
                    📝 {withdrawal.notes}
                  </p>
                )}

                <div className="flex gap-3 mt-4 pt-4 border-t">
                  <Button 
                    onClick={() => handleApprove(withdrawal.id)}
                    className="flex-1 bg-green-600 hover:bg-green-700"
                  >
                    <CheckCircle className="w-4 h-4 mr-2" />
                    Approuver
                  </Button>
                  <Button 
                    variant="destructive"
                    onClick={() => setSelected(withdrawal)}
                    className="flex-1"
                  >
                    <XCircle className="w-4 h-4 mr-2" />
                    Refuser
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <AlertDialog open={!!selected} onOpenChange={() => setSelected(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Refuser le retrait</AlertDialogTitle>
            <AlertDialogDescription>
              Êtes-vous sûr de vouloir refuser ce retrait de {selected?.amount.toLocaleString()} FCFA ?
              <br />
              <span className="text-sm text-gray-500">
                Le client sera notifié et son solde restera disponible.
              </span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Annuler</AlertDialogCancel>
            <AlertDialogAction 
              className="bg-red-600 hover:bg-red-700"
              onClick={() => {
                if (selected) handleReject(selected.id);
                setSelected(null);
              }}
            >
              Confirmer le refus
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
Hooks pour les tontines
tsx
// packages/hooks/useTontine.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/api-client';

export function useTontine(tontineId?: string) {
  const queryClient = useQueryClient();

  // Liste des tontines (marchand)
  const useTontines = (category?: string) => {
    return useQuery({
      queryKey: ['tontines', category],
      queryFn: () => api.tontine.list({ category }),
    });
  };

  // Détail d'une tontine
  const useTontineDetail = () => {
    return useQuery({
      queryKey: ['tontine', tontineId],
      queryFn: () => api.tontine.get(tontineId!),
      enabled: !!tontineId,
    });
  };

  // Créer une tontine
  const useCreateTontine = () => {
    return useMutation({
      mutationFn: (data: any) => api.tontine.create(data),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['tontines'] });
      },
    });
  };

  // Rejoindre une tontine (client)
  const useJoinTontine = () => {
    return useMutation({
      mutationFn: (id: string) => api.tontine.join(id),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['tontine', tontineId] });
      },
    });
  };

  // Cotiser (client)
  const useContribute = () => {
    return useMutation({
      mutationFn: (data: { amount: number; payment_method: string }) =>
        api.tontine.contribute(tontineId!, data),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['tontine', tontineId] });
        queryClient.invalidateQueries({ queryKey: ['tontine-balance'] });
      },
    });
  };

  // Retirer (client)
  const useWithdraw = () => {
    return useMutation({
      mutationFn: (data: { order_id: string; amount: number; notes?: string }) =>
        api.tontine.withdraw(tontineId!, data),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['tontine', tontineId] });
        queryClient.invalidateQueries({ queryKey: ['tontine-balance'] });
      },
    });
  };

  // Approuver un retrait (marchand)
  const useApproveWithdrawal = () => {
    return useMutation({
      mutationFn: (withdrawalId: string) =>
        api.tontine.approveWithdrawal(withdrawalId),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['pending-withdrawals'] });
      },
    });
  };

  return {
    useTontines,
    useTontineDetail,
    useCreateTontine,
    useJoinTontine,
    useContribute,
    useWithdraw,
    useApproveWithdrawal,
  };
}
Client API
ts
// packages/api-client/src/tontine.ts
import { apiClient } from './client';

export const tontineApi = {
  // Marchand
  getSettings: () => apiClient.get('/api/dashboard/tontine/settings'),
  updateSettings: (data: any) => apiClient.put('/api/dashboard/tontine/settings', data),
  
  list: (params?: { category?: string }) =>
    apiClient.get('/api/dashboard/tontine', { params }),
  
  create: (data: any) => apiClient.post('/api/dashboard/tontine', data),
  get: (id: string) => apiClient.get(`/api/dashboard/tontine/${id}`),
  update: (id: string, data: any) => apiClient.put(`/api/dashboard/tontine/${id}`, data),
  toggle: (id: string, status: string) =>
    apiClient.post(`/api/dashboard/tontine/${id}/toggle`, { status }),
  delete: (id: string) => apiClient.delete(`/api/dashboard/tontine/${id}`),
  
  // Membres
  getMembers: (id: string) => apiClient.get(`/api/dashboard/tontine/${id}/members`),
  addMember: (id: string, data: any) =>
    apiClient.post(`/api/dashboard/tontine/${id}/members`, data),
  blacklistMember: (memberId: string) =>
    apiClient.post(`/api/dashboard/tontine/members/${memberId}/blacklist`),
  
  // Retraits
  getPendingWithdrawals: () => apiClient.get('/api/dashboard/tontine/withdrawals/pending'),
  approveWithdrawal: (id: string) =>
    apiClient.post(`/api/dashboard/tontine/withdrawals/${id}/approve`),
  rejectWithdrawal: (id: string, data?: { reason: string }) =>
    apiClient.post(`/api/dashboard/tontine/withdrawals/${id}/reject`, data),
  
  // Éligibilité
  addEligibleProducts: (id: string, productIds: string[]) =>
    apiClient.post(`/api/dashboard/tontine/${id}/eligibility`, { product_ids: productIds }),
  removeEligibleProduct: (id: string, productId: string) =>
    apiClient.delete(`/api/dashboard/tontine/${id}/eligibility/${productId}`),
  
  // Annonces
  createAnnouncement: (id: string, data: any) =>
    apiClient.post(`/api/dashboard/tontine/${id}/announcements`, data),
  deleteAnnouncement: (id: string) =>
    apiClient.delete(`/api/dashboard/tontine/announcements/${id}`),
  
  // Client
  listAvailable: (params?: { category?: string }) =>
    apiClient.get('/api/client/tontine', { params }),
  getDetail: (id: string) => apiClient.get(`/api/client/tontine/${id}`),
  join: (id: string) => apiClient.post(`/api/client/tontine/${id}/join`),
  leave: (id: string) => apiClient.post(`/api/client/tontine/${id}/leave`),
  contribute: (id: string, data: { amount: number; payment_method: string; payment_ref?: string }) =>
    apiClient.post(`/api/client/tontine/${id}/contribute`, data),
  getContributions: (id: string) =>
    apiClient.get(`/api/client/tontine/${id}/contributions`),
  withdraw: (id: string, data: { order_id: string; amount: number; notes?: string }) =>
    apiClient.post(`/api/client/tontine/${id}/withdraw`, data),
  getWithdrawals: () => apiClient.get('/api/client/tontine/withdrawals'),
  getGoal: (id: string) => apiClient.get(`/api/client/tontine/${id}/goal`),
  setGoal: (id: string, data: { target_amount: number; target_product_id?: string; notes?: string }) =>
    apiClient.post(`/api/client/tontine/${id}/goal`, data),
  getDashboard: () => apiClient.get('/api/client/tontine/dashboard'),
};
text

---

## 📄 Document 4 : Diagramme supplémentaire `/docs/diagrams/tontine-categories.puml`

```plantuml
@startuml
title Catégories de Tontines

package "Tontine Moto" {
    [Motos]
    [Pièces détachées]
    [Accessoires]
    note right: Client épargne pour\nacheter une moto
}

package "Tontine Voiture" {
    [Voitures]
    [Entretien]
    [Assurance]
    note right: Client épargne pour\nacheter une voiture
}

package "Tontine Ciment" {
    [Ciment]
    [Matériaux]
    [Construction]
    note right: Client épargne pour\nconstruire sa maison
}

package "Tontine Électroménager" {
    [Réfrigérateurs]
    [Téléviseurs]
    [Climatiseurs]
    note right: Client épargne pour\néquiper sa maison
}

package "Tontine Téléphonie" {
    [Téléphones]
    [Accessoires]
    [Forfaits]
    note right: Client épargne pour\nacheter un smartphone
}

package "Tontine Mode" {
    [Vêtements]
    [Chaussures]
    [Bijoux]
    note right: Client épargne pour\nsa garde-robe
}

package "Tontine Générale" {
    [Tous les produits]
    note right: Tontine polyvalente\npour tout type d'achat
}

@enduml
✅ Récapitulatif des documents à créer
Fichier	Description	Statut
/docs/11-tontine-system.md	Documentation complète du système	✅ Prêt
/docs/12-tontine-api-reference.md	Routes API détaillées	✅ Prêt
/docs/13-tontine-frontend.md	Interface utilisateur Next.js	✅ Prêt
/docs/diagrams/tontine-categories.puml	Diagramme des catégories	✅ Prêt
