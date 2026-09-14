# Multi-stage Dockerfile for AvtoFast Backend
# Optimized for high concurrency, minimal footprint (<25MB), and non-root security

# --- Stage 1: Build Binaries ---
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build essentials and certs
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Build statically linked, stripped production binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/seed ./cmd/seed

# --- Stage 2: Production Minimal Runtime ---
FROM alpine:3.20

# Create non-root unprivileged service user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Install runtime dependencies (TLS root certs and timezone data)
RUN apk --no-cache add ca-certificates tzdata curl

# Copy compiled binaries from builder
COPY --from=builder /app/bin/api /app/bin/api
COPY --from=builder /app/bin/migrate /app/bin/migrate
COPY --from=builder /app/bin/seed /app/bin/seed

# Copy database migrations and default config
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /app/config /app/config
COPY --from=builder /app/.env.example /app/.env.example

# Set ownership to unprivileged user
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

CMD ["/app/bin/api"]
