# Architecture de GoShop

## Principes
- **Clean Architecture / DDD / Hexagonale**
- **Indépendance des frameworks** : Le `domain` ne dépend de rien.
- **Testabilité** : Les interfaces permettent de mocker facilement.

## Structure actuelle (basée sur votre arborescence)
├── cmd/api/ # Point d'entrée de l'application
├── internal/app/ # Initialisation, DI container (Wire)
├── domain/ # ❤️ Cœur métier
│ ├── entity/ # Entités (User, Product, Order, Customer)
│ ├── repository/ # Interfaces des repositories (pour inversion de contrôle)
│ ├── auth_entity/ # Entités liées à l'auth (RefreshToken)
│ └── valueObject/ # Objets valeur (ex: Email, Money)
├── application/ # Use cases
│ ├── usecase/ # Implémentations des use cases (ex: CreateOrderUseCase)
│ ├── dto/ # Data Transfer Objects (entrée/sortie)
│ └── mapper/ # Mappeurs entre entités et DTOs
├── interfaces/ # Adaptateurs (entrée)
│ ├── handler/ # HTTP handlers (Chi)
│ ├── middl/ # Middlewares (auth, tenant, logging)
│ └── utils/ # Helpers (response, error handling)
├── infrastructure/ # Adaptateurs (sortie)
│ └── postgres/ # Implémentations des repositories (SQL)
├── migrations/ # Fichiers de migration SQL (versionnés)
├── config/ # Configuration (env, logging)
├── tests/ # Tests (unitaires, intégration, E2E, charge)
├── mocks/ # Mocks générés (pour les tests)
└── k8s/ # Manifests Kubernetes

text

## Flux de dépendances
interfaces/handler → application/usecase → domain/repository (interface)
↑
infrastructure/postgres (implémentation)

text
