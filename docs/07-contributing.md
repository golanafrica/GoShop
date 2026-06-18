

```markdown
# Contribuer à GoShop

## Comment contribuer ?

1. **Fork** le projet
2. **Créer une branche** `feature/ma-fonctionnalite`
3. **Écrire des tests** pour votre code
4. **Commiter** avec des messages clairs
5. **Ouvrir une Pull Request** vers `develop`

## Convention de code (Go)
- `gofmt` et `go vet`
- Documenter les fonctions publiques
- Utiliser `zerolog` pour les logs structurés

## Style des commits
- `feat:` Nouvelle fonctionnalité
- `fix:` Correction de bug
- `docs:` Documentation
- `test:` Ajout de tests
- `chore:` Maintenance

## Tests requis
- **Unitaires** : `go test ./... -v`
- **Intégration** : `go test -tags=integration ./... -v`
- **E2E** : `go test -tags=e2e ./tests/e2e/... -v`

## Environnement de développement
- Go 1.25+
- Docker & Docker Compose
- (Optionnel) Minikube

## Relecture de code
- Respect de l'architecture (Domain → Application → Interfaces → Infrastructure)
- Pas de fuite de `shop_id`
- Toutes les nouvelles routes documentées