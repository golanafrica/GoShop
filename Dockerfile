# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
# 🛡️ Image épinglée par TAG + DIGEST SHA-256 (immuable, vérifiable,
#    ET suivie par Dependabot — le tag reste devant le @ pour que
#    l'outil sache quelle lignée de versions surveiller)
# Source : https://hub.docker.com/layers/library/golang/1.25.5-alpine
# Digest vérifié le : 13 août 2026
# ⚠️ MAINTENANCE : Dependabot ouvre une PR automatique quand le digest change
FROM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS builder
# Installer les dépendances de build
RUN apk add --no-cache git ca-certificates tzdata
# Définir le répertoire de travail
WORKDIR /app
# Copier les fichiers de dépendances
COPY go.mod go.sum ./
# Télécharger les dépendances
RUN go mod download
# Copier tout le code source
COPY . .
# Compiler l'application en mode statique (sans CGO)
# 🛡️ CGO_ENABLED=0 : binaire statique, pas de dépendances libc dynamiques
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./bin/api ./cmd/api

# ==================================================================
# STAGE 2 : RUNTIME
# ==================================================================
# 🛡️ Image épinglée par TAG + DIGEST SHA-256 (immuable, vérifiable,
#    ET suivie par Dependabot)
# Alpine 3.22.0 (dernière version stable en août 2026)
# Source : https://hub.docker.com/layers/library/alpine/3.22.0
# Digest vérifié le : 13 août 2026
# ⚠️ MAINTENANCE : Dependabot ouvre une PR automatique quand le digest change
FROM alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715
# Installer les dépendances runtime minimales
RUN apk --no-cache add ca-certificates tzdata
# Créer un utilisateur non-root avec UID explicite
# 🛡️ UID 10001 : déterministe pour K8s runAsUser, évite les conflits
# 🛡️ -D : pas de mot de passe (compte service uniquement)
RUN adduser -D -u 10001 -s /bin/sh goshop
# Définir le répertoire de travail
WORKDIR /app
# Copier le binaire depuis le stage builder avec ownership correct
# 🛡️ Fusion COPY + chown en un seul layer (optimisation taille image)
COPY --from=builder --chown=goshop:goshop /app/bin/api .
# Configurer les permissions du binaire
# 🛡️ 555 = lecture+exécution uniquement (pas d'écriture)
RUN chmod 555 api
# 🆕 Créer le dossier uploads AVANT de passer en non-root, avec les bons droits
# 🛡️ Sans ça, le binaire (UID 10001) n'a pas le droit de créer/écrire dans ./uploads
RUN mkdir -p /app/uploads && chown -R goshop:goshop /app/uploads
# Passer à l'utilisateur non-root
USER goshop
# Exposer le port
EXPOSE 8080
# Lancer directement l'application
# Docker Compose gère l'attente via healthcheck + depends_on
CMD ["./api"]