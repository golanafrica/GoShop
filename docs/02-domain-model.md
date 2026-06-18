# Modèle de données

## Entités principales (Domaines)

### Utilisateur (User)
- `id`, `email`, `password_hash`, `first_name`, `last_name`, `phone`, `role` (admin, marchand, client).
- `created_at`, `updated_at`.

### Boutique (Shop) - Multi-tenant
- `id`, `name`, `slug`, `custom_domain`, `owner_id` (User).
- `logo_url`, `theme` (JSONB).
- `plan` (free, pro, business).
- `db_schema` (préparation Option A, nullable).
- `is_active`, `created_at`, `updated_at`.

### Produit (Product)
- `id`, `shop_id` (UUID), `name`, `description`.
- `price` (int64, centimes FCFA), `stock`.
- `images` (TEXT[]), `category`.
- `created_at`, `updated_at`, `deleted_at`.

### Client (Customer)
- `id`, `shop_id` (UUID), `user_id` (UUID, optionnel).
- `first_name`, `last_name`, `phone`, `email`.
- `reliability_score` (enum: Excellente, Bonne, Passable, Mauvaise).
- `preferred_payment_method` (wave, cash, orange_money).
- `created_at`, `updated_at`.

### Commande (Order)
- `id`, `shop_id` (UUID), `customer_id` (UUID), `user_id` (UUID, si connecté).
- `total_amount` (int64), `status` (pending, confirmed, paid, delivered, cancelled).
- `payment_method` (cash, wave, orange_money, credit).
- `payment_plan_id` (UUID, nullable) → pour le crédit.
- `cash_delivery_config` (JSONB) : zones, réservation, monnaie.
- `created_at`, `updated_at`.

### Plan de paiement (PaymentPlan) - Crédit
- `id`, `shop_id` (UUID).
- `name`, `description`.
- `down_payment_pct` (int), `installments_count` (int), `interval_days` (int).
- `interest_rate_pct` (decimal), `min_order_amount`, `max_order_amount`.
- `requires_approval` (bool).
- `is_active`, `created_at`.

### Tranche (Installment)
- `id`, `order_id` (UUID), `shop_id` (UUID).
- `sequence` (int), `amount` (int64).
- `due_date` (date), `paid_at` (timestamptz, nullable).
- `payment_method` (wave, cash, orange_money), `payment_ref`.
- `status` (pending, paid, overdue, waived).

### Transaction (PaymentTransaction)
- `id`, `shop_id`, `order_id`, `installment_id` (nullable).
- `provider_code` (wave, orange_money, cash).
- `amount`, `provider_fee`, `platform_fee`, `net_amount`.
- `provider_ref`, `phone_number`.
- `status` (pending, processing, success, failed, refunded).
- `metadata` (JSONB).

## Relations clés

Shop 1──* Product
Shop 1──* Customer
Shop 1──* Order
Shop 1──* PaymentPlan
User 1──1 Shop (owner)

Order 1──* Installment
Order 1──0..1 PaymentPlan
Order 1──* PaymentTransaction
Customer 1──* Order

text

## Stratégie de migration (Option B → A)
*Voir [Multi-tenant Strategy](04-multi-tenant.md)*


