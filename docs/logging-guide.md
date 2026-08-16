# 📋 Guide des Logs GoShop

## Formats de Logs

### Développement (`APP_ENV=development`)
- Console colorée avec timestamps
- Niveaux : TRACE | DEBUG | INFO | WARN | ERROR | FATAL | PANIC

### Production (`APP_ENV=production` ou `staging`)
- JSON structuré vers stdout
- Compatible avec tous les collecteurs de logs (ELK, Loki, CloudWatch)

## Champs Standards

| Champ | Description | Exemple |
|-------|-------------|---------|
| `level` | Niveau de log | `"info"` |
| `service` | Nom du service | `"goshop-api"` |
| `version` | Version de l'app | `"v4.5.0"` |
| `environment` | Environnement | `"production"` |
| `component` | Composant | `"payment_handler"` |
| `request_id` | ID unique de requête | `"550e8400-..."` |
| `user_id` | ID utilisateur | `"user-123"` |
| `shop_id` | ID boutique | `"shop-456"` |
| `time` | Timestamp RFC3339 | `"2026-08-15T12:30:00Z"` |
| `message` | Message du log | `"Payment initiated"` |

## Recherche dans les Logs

### Trouver toutes les erreurs
```bash
# Avec jq
go run cmd/api/main.go | jq 'select(.level=="error")'