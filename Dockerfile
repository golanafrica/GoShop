# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
# Utilisation de Go 1.26.9 pour correspondre exactement au go.mod
FROM golang:1.27.0-alpine AS builder

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
# Alpine 3.20 : version stable, sécurisée et existante
FROM alpine:3.21

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