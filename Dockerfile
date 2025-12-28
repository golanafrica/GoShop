# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
FROM golang:1.25.0-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./bin/api ./cmd/api

# ==================================================================
# STAGE 2 : RUNTIME
# ==================================================================
FROM alpine:latest

# Installer les dépendances runtime minimales
RUN apk --no-cache add ca-certificates tzdata

# Créer un utilisateur non-root
RUN adduser -D -s /bin/sh goshop

# Définir le répertoire de travail
WORKDIR /app

# Copier le binaire depuis le stage builder
COPY --from=builder /app/bin/api .

# Configurer les permissions
RUN chown -R goshop:goshop /app && \
    chmod 555 api

# Passer à l'utilisateur non-root
USER goshop

# Exposer le port
EXPOSE 8080

# Lancer directement l'application
# Docker Compose gère l'attente via healthcheck + depends_on
CMD ["./api"]