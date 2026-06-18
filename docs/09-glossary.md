arkdown
# Glossaire

| Terme | Définition |
|-------|------------|
| **Boutique (Shop)** | Espace en ligne d'un marchand, avec ses produits, commandes, paramètres. |
| **Client** | Personne qui achète dans une boutique. Peut être connecté (compte) ou anonyme. |
| **Marchand** | Propriétaire d'une boutique, gère produits, commandes, paiements. |
| **Comptant** | Paiement immédiat en ligne (Wave, Orange Money) ou à la livraison (cash). |
| **Crédit / Tranches** | Paiement en plusieurs fois : acompte + échéances. |
| **Acompte** | Premier paiement effectué à la commande (ex: 30% du total). |
| **Échéance** | Date à laquelle une tranche doit être payée. |
| **Score de fiabilité** | Indicateur de la ponctualité des paiements d'un client (Excellent → Mauvaise). |
| **Plan de paiement** | Template défini par le marchand (ex: "3 fois sans frais"). |
| **Provider** | Service de paiement externe (Wave, Orange Money, MTN MoMo). |
| **Webhook** | Appel HTTP effectué par le provider pour confirmer un paiement. |
| **Multi-tenant** | Architecture où plusieurs boutiques (tenants) partagent la même application. |
| **Option B** | Multi-tenant avec une colonne `shop_id` dans chaque table. |
| **Option A** | Multi-tenant avec un schéma PostgreSQL par boutique. |
| **Clean Architecture** | Architecture avec séparation des couches (Domain, Application, Infrastructure). |
| **DDD (Domain-Driven Design)** | Conception centrée sur le domaine métier. |
| **Use Case** | Une action métier spécifique (ex: "Passer une commande"). |
| **DTO (Data Transfer Object)** | Objet pour transférer des données entre couches (ex: JSON). |
| **JWT (JSON Web Token)** | Token d'authentification (access + refresh). |
| **Rate Limiting** | Limitation du nombre de requêtes par client (sécurité). |
| **Cron Job** | Tâche automatisée exécutée périodiquement (ex: expiration des commandes). |