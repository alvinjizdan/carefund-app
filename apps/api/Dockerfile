# ==========================================
# CareFund Backend Multi-Target Dockerfile
# Multi-stage, minimal runtime, non-root user
# ==========================================

# ------------------------------------------
# Stage 1: Build Stage (Golang Compiler)
# ------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Install build-time certificates
RUN apk add --no-cache ca-certificates tzdata

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source tree
COPY . .

# Compile statically linked binaries without CGO or debug symbols
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -ldflags="-s -w" -o /bin/carefund-server ./cmd/server
RUN go build -ldflags="-s -w" -o /bin/carefund-worker ./cmd/worker
RUN go build -ldflags="-s -w" -o /bin/carefund-migrate ./cmd/migrate

# ------------------------------------------
# Stage 2: Base Minimal Runtime
# ------------------------------------------
FROM alpine:3.21 AS base

# Create non-root unprivileged service user and group
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -s /sbin/nologin -h /app

# Install runtime SSL certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
RUN chown -R appuser:appgroup /app

USER appuser:appgroup

ENV ENV=production

# ------------------------------------------
# Target: Server (API Process)
# ------------------------------------------
FROM base AS server

COPY --from=builder --chown=appuser:appgroup /bin/carefund-server /app/carefund-server

# Public HTTP API Port
EXPOSE 8080
# Internal Metrics Port (loopback/private interface by default)
EXPOSE 9090

ENTRYPOINT ["/app/carefund-server"]

# ------------------------------------------
# Target: Worker (Background Daemon / Single Pass)
# ------------------------------------------
FROM base AS worker

COPY --from=builder --chown=appuser:appgroup /bin/carefund-worker /app/carefund-worker

# Dedicated Worker Metrics Port
EXPOSE 9091

ENTRYPOINT ["/app/carefund-worker"]

# ------------------------------------------
# Target: Migrate (Forward-Only Schema CLI)
# ------------------------------------------
FROM base AS migrate

COPY --from=builder --chown=appuser:appgroup /bin/carefund-migrate /app/carefund-migrate
COPY --chown=appuser:appgroup migrations /app/migrations

ENTRYPOINT ["/app/carefund-migrate"]
CMD ["-path=/app/migrations", "up"]
