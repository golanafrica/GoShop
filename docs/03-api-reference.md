# API Reference

## Base URL
- **Développement** : `http://localhost:8080`
- **Production** : `https://api.golanafrica.com`

## Authentification
- **JWT** : Access token (15min), Refresh token (7j).
- **Headers** : `Authorization: Bearer <access_token>`

## Endpoints publics
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| POST | `/register` | Créer un compte utilisateur |
| POST | `/login` | Connexion (email/password) |
| POST | `/auth/refresh` | Rafraîchir le JWT |
| GET | `/health/live` | Liveness probe |
| GET | `/health/ready` | Readiness probe |
| GET | `/swagger/index.html` | Swagger UI |

## Routes protégées (/api) - Dashboard marchand
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/dashboard` | Vue d'ensemble (métriques) |
| GET | `/api/dashboard/orders` | Liste commandes |
| POST | `/api/dashboard/orders` | Créer commande |
| POST | `/api/dashboard/orders/{id}/accept` | **Accepter commande cash** |
| POST | `/api/dashboard/orders/{id}/reject` | **Refuser commande cash** |
| POST | `/api/dashboard/orders/{id}/deliver` | **Marquer comme livrée/payée** |
| GET | `/api/dashboard/products` | Liste produits (CRUD complet) |
| GET | `/api/dashboard/customers` | Liste clients |
| GET | `/api/dashboard/installments` | Tranches à venir |
| GET | `/api/dashboard/payment-settings` | Paramètres paiement |
| PUT | `/api/dashboard/payment-settings` | Mettre à jour paramètres |
| POST | `/api/dashboard/payment-methods/{code}/enable` | Activer moyen de paiement |
| POST | `/api/dashboard/payment-plans` | Créer plan de crédit |

## Routes publiques (/public) - Storefront client
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/public/shops/{slug}` | Infos boutique |
| GET | `/public/shops/{slug}/products` | Catalogue produits |
| POST | `/public/shops/{slug}/orders` | Créer commande (cash/credit) |
| GET | `/public/shops/{slug}/checkout-options` | Modes de paiement disponibles |

## Webhooks (paiements)
| Méthode | Endpoint | Description |
|---------|----------|-------------|
| POST | `/webhooks/wave` | Confirmation Wave |
| POST | `/webhooks/orange_money` | Confirmation Orange Money |
| POST | `/webhooks/mtn_momo` | Confirmation MTN MoMo |