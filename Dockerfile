# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
# CVE stdlib (net/http, http2, crypto/tls) : correctifs dès Go 1.26.9 / 1.27.2
# Tag sans digest : Dependabot pourra ré-épingler le SHA ensuite.
FROM golang:1.26.9-alpine AS builder

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
FROM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

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