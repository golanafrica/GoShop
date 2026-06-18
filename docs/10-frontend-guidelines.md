markdown
# Frontend Guidelines - Next.js

## Structure du monorepo
golanafrica-frontend/
├── apps/
│ ├── dashboard/ # Dashboard marchand
│ └── storefront/ # Storefront client
├── packages/
│ ├── ui/ # Composants partagés
│ ├── api-client/ # Client HTTP vers Go backend
│ └── types/ # Types TypeScript partagés

text

## Technologies
- Next.js 15 (App Router)
- TypeScript
- Tailwind CSS
- shadcn/ui (composants)
- Turborepo (monorepo)

## Pages à construire

### Dashboard marchand
- `/dashboard` - Vue d'ensemble
- `/dashboard/orders` - Commandes
- `/dashboard/products` - Produits (CRUD)
- `/dashboard/customers` - Clients
- `/dashboard/installments` - Tranches
- `/dashboard/settings` - Paramètres (paiements, domaine, apparence)

### Storefront client
- `/shop/[slug]` - Page boutique
- `/shop/[slug]/products` - Catalogue
- `/shop/[slug]/products/[id]` - Détail produit
- `/shop/[slug]/cart` - Panier
- `/shop/[slug]/checkout` - Paiement

### Espace client
- `/account` - Mes commandes
- `/account/orders/[id]` - Détail commande
- `/account/profile` - Profil

## Connexion avec le backend Go
- API URL : `NEXT_PUBLIC_API_URL`
- Authentification : JWT (access + refresh)
- Client API : `packages/api-client`

