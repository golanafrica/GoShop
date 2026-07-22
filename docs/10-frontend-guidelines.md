

---

```markdown
# 🎨 Frontend Guidelines - Next.js (GoShop v4.5.0)

**Version** : v4.5.0  
**Dernière mise à jour** : 2026-07-21  
**Stack** : Next.js 15 (App Router) + TypeScript + Tailwind CSS + shadcn/ui + Turborepo

---

## 📋 Table des matières
1. [Structure du monorepo](#1-structure-du-monorepo)
2. [Technologies](#2-technologies)
3. [Architecture de communication avec le backend](#3-architecture-de-communication-avec-le-backend)
4. [Gestion de l'authentification](#4-gestion-de-lauthentification)
5. [Gestion du Multi-tenant](#5-gestion-du-multi-tenant)
6. [Notifications WebSocket (v4.5.0)](#6-notifications-websocket-v450)
7. [Pages à construire](#7-pages-à-construire)
8. [Intégration Paiements Mobile Money](#8-intégration-paiements-mobile-money)
9. [Gestion des erreurs](#9-gestion-des-erreurs)
10. [Bonnes pratiques de sécurité](#10-bonnes-pratiques-de-sécurité)
11. [Variables d'environnement](#11-variables-denvironnement)

---

## 1. Structure du monorepo

```text
golanafrica-frontend/
├── apps/
│   ├── dashboard/          # Dashboard marchand (Next.js)
│   ├── storefront/         # Storefront client public (Next.js)
│   ├── customer-portal/    # Espace client connecté (Next.js)
│   └── admin/              # Panel admin super_admin (Next.js)
│
├── packages/
│   ├── ui/                 # Composants partagés (shadcn/ui)
│   ├── api-client/         # Client HTTP vers Go backend (axios/fetch)
│   ├── types/              # Types TypeScript partagés (générés depuis Swagger)
│   ├── hooks/              # Hooks React partagés (auth, websocket, etc.)
│   └── utils/              # Utilitaires (formatage monnaie, dates, etc.)
│
├── turbo.json              # Configuration Turborepo
├── package.json            # Workspace root
└── pnpm-workspace.yaml     # Gestionnaire de paquets
```

---

## 2. Technologies

### Core
- **Next.js 15** (App Router) - Framework React avec SSR/SSG
- **TypeScript 5.x** - Typage strict
- **React 19** - Librairie UI
- **Turborepo** - Orchestration du monorepo

### UI & Style
- **Tailwind CSS 4** - Utility-first CSS
- **shadcn/ui** - Composants accessibles et personnalisables
- **Lucide React** - Icônes
- **Framer Motion** - Animations

### État & Data
- **TanStack Query (React Query)** - Gestion des données serveur
- **Zustand** - État global léger (auth, panier)
- **Zod** - Validation des schémas

### Communication
- **Axios** - Client HTTP avec intercepteurs
- **Native WebSocket API** - Notifications temps réel
- **Socket.io-client** (optionnel) - Alternative WebSocket

### Outils
- **ESLint + Prettier** - Qualité du code
- **Husky + lint-staged** - Pre-commit hooks
- **Vitest** - Tests unitaires
- **Playwright** - Tests E2E

---

## 3. Architecture de communication avec le backend

### 3.1. Client API centralisé (`packages/api-client`)

```typescript
// packages/api-client/src/client.ts
import axios, { AxiosInstance, AxiosError } from 'axios';
import { useAuthStore } from '@goshop/hooks';

class ApiClient {
  private client: AxiosInstance;
  private isRefreshing = false;
  private failedQueue: any[] = [];

  constructor() {
    this.client = axios.create({
      baseURL: process.env.NEXT_PUBLIC_API_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors() {
    // Request interceptor : injecte le JWT et le X-Shop-Slug
    this.client.interceptors.request.use((config) => {
      const token = useAuthStore.getState().accessToken;
      const shopSlug = useAuthStore.getState().currentShopSlug;

      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      if (shopSlug) {
        config.headers['X-Shop-Slug'] = shopSlug;
      }
      return config;
    });

    // Response interceptor : gestion automatique du refresh token
    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config;

        if (error.response?.status === 401 && !originalRequest._retry) {
          if (this.isRefreshing) {
            return new Promise((resolve, reject) => {
              this.failedQueue.push({ resolve, reject });
            }).then((token) => {
              originalRequest.headers.Authorization = `Bearer ${token}`;
              return this.client(originalRequest);
            });
          }

          originalRequest._retry = true;
          this.isRefreshing = true;

          try {
            const newToken = await this.refreshAccessToken();
            this.processQueue(null, newToken);
            originalRequest.headers.Authorization = `Bearer ${newToken}`;
            return this.client(originalRequest);
          } catch (refreshError) {
            this.processQueue(refreshError, null);
            useAuthStore.getState().logout();
            window.location.href = '/login';
            return Promise.reject(refreshError);
          } finally {
            this.isRefreshing = false;
          }
        }

        return Promise.reject(error);
      }
    );
  }

  private async refreshAccessToken(): Promise<string> {
    const refreshToken = useAuthStore.getState().refreshToken;
    const response = await axios.post(
      `${process.env.NEXT_PUBLIC_API_URL}/auth/refresh`,
      { refresh_token: refreshToken }
    );
    useAuthStore.getState().setTokens(response.data.access_token, response.data.refresh_token);
    return response.data.access_token;
  }

  private processQueue(error: any, token: string | null) {
    this.failedQueue.forEach((prom) => {
      if (error) prom.reject(error);
      else prom.resolve(token);
    });
    this.failedQueue = [];
  }

  public get axios() {
    return this.client;
  }
}

export const apiClient = new ApiClient().axios;
```

### 3.2. Types générés depuis Swagger

```typescript
// packages/types/src/index.ts
// Générés automatiquement depuis /swagger/doc.json du backend
export interface User {
  id: string;
  email: string;
  role: 'super_admin' | 'admin' | 'merchant' | 'user';
}

export interface Shop {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  is_active: boolean;
}

export interface Product {
  id: string;
  shop_id: string;
  name: string;
  description: string;
  price_cents: number; // ⚠️ Toujours en centimes !
  stock: number;
}

export interface Order {
  id: string;
  shop_id: string;
  customer_id: string;
  total_amount_cents: number;
  status: 'pending_confirmation' | 'confirmed' | 'out_for_delivery' | 'delivered' | 'cancelled';
  payment_method: 'cash_on_delivery' | 'wave' | 'orange_money' | 'moov_money' | 'yenga_pay' | 'credit';
}

// ... autres types
```

---

## 4. Gestion de l'authentification

### 4.1. Store Zustand pour l'auth

```typescript
// packages/hooks/src/useAuthStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: User | null;
  currentShopSlug: string | null;
  requires2FA: boolean;
  
  setTokens: (access: string, refresh: string) => void;
  setUser: (user: User) => void;
  setCurrentShop: (slug: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      currentShopSlug: null,
      requires2FA: false,

      setTokens: (accessToken, refreshToken) =>
        set({ accessToken, refreshToken }),

      setUser: (user) => set({ user }),

      setCurrentShop: (slug) => set({ currentShopSlug: slug }),

      logout: () =>
        set({
          accessToken: null,
          refreshToken: null,
          user: null,
          currentShopSlug: null,
        }),
    }),
    {
      name: 'goshop-auth-storage',
    }
  )
);
```

### 4.2. Flux de connexion avec 2FA (v4.4.0)

```typescript
// apps/dashboard/src/app/login/page.tsx
'use client';

import { useState } from 'react';
import { apiClient } from '@goshop/api-client';
import { useAuthStore } from '@goshop/hooks';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [requires2FA, setRequires2FA] = useState(false);
  const [tempToken, setTempToken] = useState('');

  const handleLogin = async () => {
    try {
      const response = await apiClient.post('/login', { email, password });
      
      if (response.data.requires_2fa) {
        // Étape 1 : 2FA requise
        setRequires2FA(true);
        setTempToken(response.data.temp_token);
      } else {
        // Étape 2 : Connexion directe
        useAuthStore.getState().setTokens(
          response.data.access_token,
          response.data.refresh_token
        );
        useAuthStore.getState().setUser(response.data.user);
        window.location.href = '/dashboard';
      }
    } catch (error) {
      console.error('Login failed', error);
    }
  };

  const handleVerify2FA = async () => {
    const response = await apiClient.post('/auth/2fa/verify', {
      temp_token: tempToken,
      code: totpCode,
    });
    
    useAuthStore.getState().setTokens(
      response.data.access_token,
      response.data.refresh_token
    );
    window.location.href = '/dashboard';
  };

  return (
    <div>
      {!requires2FA ? (
        <LoginForm onSubmit={handleLogin} />
      ) : (
        <TOTPForm onSubmit={handleVerify2FA} />
      )}
    </div>
  );
}
```

---

## 5. Gestion du Multi-tenant

### 5.1. Sélecteur de boutique

```typescript
// packages/hooks/src/useShopSelector.ts
import { useEffect } from 'react';
import { useAuthStore } from './useAuthStore';
import { apiClient } from '@goshop/api-client';

export function useShopSelector() {
  const { user, currentShopSlug, setCurrentShop } = useAuthStore();

  const fetchUserShops = async () => {
    const response = await apiClient.get('/api/shops');
    return response.data;
  };

  const selectShop = (slug: string) => {
    setCurrentShop(slug);
    localStorage.setItem('goshop-current-shop', slug);
  };

  useEffect(() => {
    const saved = localStorage.getItem('goshop-current-shop');
    if (saved) setCurrentShop(saved);
  }, []);

  return { fetchUserShops, selectShop, currentShopSlug };
}
```

### 5.2. Middleware Next.js pour protéger les routes

```typescript
// apps/dashboard/src/middleware.ts
import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

export function middleware(request: NextRequest) {
  const token = request.cookies.get('goshop-auth-storage');
  
  if (!token && request.nextUrl.pathname.startsWith('/dashboard')) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/dashboard/:path*', '/account/:path*'],
};
```

---

## 6. Notifications WebSocket (v4.5.0)

### 6.1. Hook WebSocket réutilisable

```typescript
// packages/hooks/src/useWebSocket.ts
import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuthStore } from './useAuthStore';

interface WebSocketMessage {
  type: string;
  title: string;
  message: string;
  data: Record<string, any>;
  timestamp: string;
}

export function useWebSocket() {
  const ws = useRef<WebSocket | null>(null);
  const [messages, setMessages] = useState<WebSocketMessage[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const { accessToken } = useAuthStore();
  const reconnectTimeout = useRef<NodeJS.Timeout>();

  const connect = useCallback(() => {
    if (!accessToken) return;

    const wsUrl = `${process.env.NEXT_PUBLIC_WS_URL}/ws/notifications`;
    ws.current = new WebSocket(wsUrl);

    ws.current.onopen = () => {
      // Envoyer le token dans un message d'authentification
      ws.current?.send(JSON.stringify({
        type: 'auth',
        token: accessToken,
      }));
      setIsConnected(true);
    };

    ws.current.onmessage = (event) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data);
        setMessages((prev) => [message, ...prev].slice(0, 50)); // Garder les 50 derniers
        
        // Afficher une notification toast
        showNotification(message);
      } catch (error) {
        console.error('WebSocket message parse error', error);
      }
    };

    ws.current.onclose = () => {
      setIsConnected(false);
      // Reconnexion automatique après 5s
      reconnectTimeout.current = setTimeout(connect, 5000);
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error', error);
    };
  }, [accessToken]);

  useEffect(() => {
    connect();
    return () => {
      ws.current?.close();
      if (reconnectTimeout.current) clearTimeout(reconnectTimeout.current);
    };
  }, [connect]);

  return { messages, isConnected };
}

function showNotification(message: WebSocketMessage) {
  // Utiliser sonner, react-hot-toast ou notification native
  console.log(`🔔 ${message.title}: ${message.message}`);
}
```

### 6.2. Utilisation dans le layout

```typescript
// apps/dashboard/src/app/layout.tsx
import { WebSocketProvider } from '@goshop/hooks';

export default function DashboardLayout({ children }) {
  return (
    <WebSocketProvider>
      <Toaster />
      {children}
    </WebSocketProvider>
  );
}
```

---

## 7. Pages à construire

### 7.1. Dashboard Marchand (`apps/dashboard`)

| Route | Description | Fonctionnalités clés |
|-------|-------------|----------------------|
| `/login` | Connexion marchand | JWT + 2FA |
| `/dashboard` | Vue d'ensemble | Stats, graphiques, notifications temps réel |
| `/dashboard/orders` | Liste commandes | Filtres, statuts COD, pagination |
| `/dashboard/orders/[id]` | Détail commande | Accepter/Refuser COD, preuve livraison |
| `/dashboard/products` | Liste produits | CRUD, stock, recherche full-text |
| `/dashboard/products/new` | Créer produit | Upload images, prix en centimes |
| `/dashboard/customers` | Liste clients | KYC status, score fiabilité |
| `/dashboard/customers/[id]` | Détail client | Historique commandes, KYC review |
| `/dashboard/credit` | Gestion crédit | Plans, demandes, échéances |
| `/dashboard/tontine` | Groupes tontine | Création, suivi cycles, vouchers |
| `/dashboard/wallet` | Portefeuille | Solde, transactions, demande retrait |
| `/dashboard/collaborators` | Collaborateurs | Invitations, rôles |
| `/dashboard/settings` | Paramètres boutique | Paiements, domaine, apparence |
| `/dashboard/kyc` | KYC marchand | Upload documents, statut |
| `/dashboard/2fa` | Sécurité 2FA | Activation, codes récupération |
| `/dashboard/api-keys` | Clés API | Création, révocation, stats |

### 7.2. Storefront Client (`apps/storefront`)

| Route | Description | Fonctionnalités clés |
|-------|-------------|----------------------|
| `/` | Accueil | Liste des boutiques |
| `/shop/[slug]` | Page boutique | Hero, catégories |
| `/shop/[slug]/products` | Catalogue | Filtres, tri, recherche |
| `/shop/[slug]/products/[id]` | Détail produit | Galerie, options paiement (cash/crédit/tontine) |
| `/shop/[slug]/cart` | Panier | Modification quantités |
| `/shop/[slug]/checkout` | Paiement | Choix provider (Wave/Orange/Moov/Yenga/COD/Crédit) |
| `/shop/[slug]/tontine/join` | Rejoindre tontine | Code invitation, KYC |

### 7.3. Espace Client (`apps/customer-portal`)

| Route | Description | Fonctionnalités clés |
|-------|-------------|----------------------|
| `/account` | Dashboard client | Commandes en cours, notifications |
| `/account/orders` | Historique commandes | Statuts, factures |
| `/account/orders/[id]` | Détail commande | Suivi COD, paiement crédit |
| `/account/credit` | Mes crédits | Échéances, paiements |
| `/account/tontine` | Mes tontines | Cycles en cours, vouchers |
| `/account/kyc` | Mon KYC | Upload documents, statut |
| `/account/profile` | Profil | Modifier infos, 2FA |
| `/account/sessions` | Sessions actives | Révocation |

### 7.4. Panel Admin (`apps/admin`)

| Route | Description | Fonctionnalités clés |
|-------|-------------|----------------------|
| `/admin` | Dashboard admin | Stats globales, métriques |
| `/admin/shops` | Gestion boutiques | Suspendre/activer, audit |
| `/admin/merchant-kyc` | KYC marchands | Validation en masse |
| `/admin/collaborators` | Collaborateurs plateforme | Gestion rôles |
| `/admin/scheduler` | Schedulers | Déclencher manuellement, voir batches |
| `/admin/commissions` | Commissions | Stats, réconciliation |
| `/admin/2fa` | Sécurité | Audit 2FA |
| `/admin/sessions` | Sessions | Révocation globale |

---

## 8. Intégration Paiements Mobile Money

### 8.1. Flux de paiement Wave/Orange/Moov/Yenga

```typescript
// apps/storefront/src/lib/payment.ts
export async function initiatePayment(orderId: string, provider: string, phoneNumber: string) {
  const response = await apiClient.post(`/api/orders/${orderId}/pay`, {
    provider, // 'wave' | 'orange_money' | 'moov_money' | 'yenga_pay'
    phone_number: phoneNumber,
    description: `Paiement commande #${orderId}`,
  });

  if (response.data.ussd_code) {
    // Afficher le code USSD à composer
    showUSSDModal(response.data.ussd_code, response.data.amount_cents);
  }

  // Écouter les webhooks via WebSocket pour confirmation
  return response.data;
}
```

### 8.2. Gestion du Cash on Delivery (COD)

```typescript
// apps/dashboard/src/app/dashboard/orders/[id]/page.tsx
'use client';

export default function OrderDetailPage({ params }) {
  const { order } = useOrder(params.id);

  const handleAccept = async () => {
    await apiClient.post(`/api/orders/${order.id}/accept`);
    toast.success('Commande acceptée');
  };

  const handleReject = async (reason: string) => {
    await apiClient.post(`/api/orders/${order.id}/reject`, { reason });
    toast.success('Commande refusée');
  };

  const handleDeliver = async (proofUrl: string, amountReceived: number) => {
    await apiClient.post(`/api/orders/${order.id}/deliver`, {
      proof_url: proofUrl,
      amount_received_cents: amountReceived * 100,
    });
    toast.success('Livraison confirmée');
  };

  return (
    <OrderActions
      status={order.status}
      onAccept={handleAccept}
      onReject={handleReject}
      onDeliver={handleDeliver}
    />
  );
}
```

---

## 9. Gestion des erreurs

### 9.1. Format d'erreur standardisé

Le backend retourne toujours ce format :
```json
{
  "code": "VALIDATION_FAILED",
  "message": "Le champ 'email' est requis",
  "status": 400
}
```

### 9.2. Handler d'erreur global

```typescript
// packages/utils/src/errorHandler.ts
export function handleApiError(error: any): string {
  if (error.response?.data?.code) {
    const code = error.response.data.code;
    
    // Mapping code → message utilisateur friendly
    const errorMessages: Record<string, string> = {
      'INVALID_PAYLOAD': 'Données invalides. Vérifiez votre saisie.',
      'VALIDATION_FAILED': error.response.data.message,
      'UNAUTHORIZED': 'Session expirée. Veuillez vous reconnecter.',
      'FORBIDDEN': 'Vous n\'avez pas les droits pour cette action.',
      'SHOP_ACCESS_DENIED': 'Vous n\'avez pas accès à cette boutique.',
      'NOT_FOUND': 'Ressource introuvable.',
      'TOO_MANY_ATTEMPTS': 'Trop de tentatives. Réessayez dans quelques minutes.',
      'CUSTOMER_NOT_FOUND': 'Client introuvable.',
      'PRODUCT_OUT_OF_STOCK': 'Produit en rupture de stock.',
      'ORDER_INSUFFICIENT_STOCK': 'Stock insuffisant pour cette commande.',
    };

    return errorMessages[code] || error.response.data.message || 'Une erreur est survenue';
  }

  if (error.code === 'ECONNABORTED') {
    return 'Délai d\'attente dépassé. Vérifiez votre connexion.';
  }

  return 'Erreur de connexion au serveur';
}
```

---

## 10. Bonnes pratiques de sécurité

### 🚨 Règles impératives

1. **NE JAMAIS stocker le JWT dans localStorage** (vulnérable au XSS) :
   - ✅ Utiliser des **cookies HttpOnly** pour le refresh token
   - ✅ Stocker l'access token en mémoire (Zustand sans persist)

2. **NE JAMAIS logger les tokens ou mots de passe** :
   ```typescript
   // ❌ MAUVAIS
   console.log('Token:', token);
   
   // ✅ BON
   console.log('User authenticated', { userId: user.id });
   ```

3. **Toujours valider les données côté client** avec Zod :
   ```typescript
   const ProductSchema = z.object({
     name: z.string().min(2).max(255),
     price_cents: z.number().int().positive(),
     stock: z.number().int().min(0),
   });
   ```

4. **Sanitizer les entrées utilisateur** avant affichage (prévention XSS)

5. **Utiliser CSP (Content Security Policy)** dans `next.config.js` :
   ```javascript
   const securityHeaders = [
     {
       key: 'Content-Security-Policy',
       value: "default-src 'self'; script-src 'self' 'unsafe-inline';",
     },
   ];
   ```

6. **Inclure le header `X-Shop-Slug`** sur toutes les requêtes API protégées (obligatoire pour le multi-tenant)

7. **Déconnecter automatiquement** après inactivité (ex: 30 minutes)

---

## 11. Variables d'environnement

### `.env.local` (Dashboard/Storefront)

```bash
# Backend API
NEXT_PUBLIC_API_URL=http://localhost:8081
NEXT_PUBLIC_WS_URL=ws://localhost:8081

# Auth
NEXT_PUBLIC_JWT_EXPIRY=900 # 15 minutes en secondes

# Features flags
NEXT_PUBLIC_ENABLE_2FA=true
NEXT_PUBLIC_ENABLE_TONTINE=true
NEXT_PUBLIC_ENABLE_CREDIT=true

# Analytics
NEXT_PUBLIC_GA_ID=G-XXXXXXXXXX
```

### `.env.production`

```bash
NEXT_PUBLIC_API_URL=https://api.golanafrica.com
NEXT_PUBLIC_WS_URL=wss://api.golanafrica.com
```

---

## 📚 Références complémentaires

- [API Reference](03-api-reference.md) - Liste complète des endpoints
- [Multi-tenant Strategy](04-multi-tenant.md) - Comprendre l'isolation
- [Domain Model](02-domain-model.md) - Entités métier
- [Testing Guide](08-testing-guide.md) - Tests E2E avec Playwright
- [Glossary](09-glossary.md) - Vocabulaire technique

---

**Dernière mise à jour** : 2026-07-21
```

