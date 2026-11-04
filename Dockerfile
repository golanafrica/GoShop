# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
# Utilisation d'une version Go stable et largement supportée (1.23)
# pour une compatibilité optimale avec golangci-lint, gosec et govulncheck.
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Binaire statique (pas de dépendances libc dynamiques)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./bin/api ./cmd/api

# ==================================================================
# STAGE 2 : RUNTIME
# ==================================================================
# Alpine 3.20 : version stable et sécurisée
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

# UID 10001 déterministe (K8s runAsUser)
RUN adduser -D -u 10001 -s /bin/sh goshop

WORKDIR /app

COPY --from=builder --chown=goshop:goshop /app/bin/api .

RUN chmod 555 api

RUN mkdir -p /app/uploads && chown -R goshop:goshop /app/uploads

USER goshop

EXPOSE 8080

CMD ["./api"]