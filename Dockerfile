# ==================================================================
# STAGE 1 : BUILD
# ==================================================================
# CVE stdlib (net/http, http2, crypto/tls) : correctifs dès Go 1.26.9 / 1.27.2
# Tag sans digest : Dependabot pourra ré-épingler le SHA ensuite.
FROM golang:1.27.3-alpine AS builder

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
FROM alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715

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