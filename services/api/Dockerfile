# ── api: Go REST + WebSocket server ─ multi-stage ───────────────────────────

# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache module downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 → fully static binary; -ldflags trims debug info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /api .

# ── Stage 2: minimal runtime ─────────────────────────────────────────────────
FROM alpine:3.20

# CA certs + timezone data needed at runtime for TLS / time.LoadLocation
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /api .

USER appuser

EXPOSE 8080

ENTRYPOINT ["./api"]
