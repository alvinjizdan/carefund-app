# CareFund Local Docker Deployment Runbook

> **Document Classification:** Engineering Operations / Local Deployment
> **Phase:** 5M.5C
> **Scope:** **LOCAL DOCKER / DOCKER COMPOSE ONLY**
> **Notice:** VPS and Cloud infrastructure are explicitly **OUT OF SCOPE** for this phase. This runbook documents the procedures for operating the CareFund backend locally in a production-like Docker environment using provider-neutral mechanisms.

---

## 1. Prerequisites

Before running the production-like stack locally, ensure the following tools are installed and operational:
- **Docker Engine:** Version 26.0+ (Docker Desktop on Windows/macOS or Docker Engine on Linux)
- **Docker Compose:** Version v2.20+ / v5+
- **OpenSSL CLI:** Required for generating local self-signed TLS certificates for local PostgreSQL transport
- **Curl:** Required for healthcheck and readiness probes
- **PostgreSQL Client Tools (optional):** `psql`, `pg_dump`, `pg_restore` (or use ephemeral Docker containers)

---

## 2. Environment Preparation

1. Create a local production environment file:
   ```bash
   cp .env.production.example .env.production.local
   ```
2. Populate the local file with test/sandbox credentials (do NOT use real production secrets):
   ```ini
   DOMAIN=localhost
   ENV=production
   PORT=8080
   LOG_FORMAT=json
   DB_HOST=carefund-pg-prodlike
   DB_PORT=5432
   DB_USER=carefund_prod
   DB_PASSWORD=carefund_prod_pass_2026
   DB_NAME=carefund_production
   DB_SSLMODE=require
   JWT_SECRET=super-secret-jwt-key-min-32-chars-length-test-2026
   MIDTRANS_SERVER_KEY=SB-Mid-server-test-key-mock
   MIDTRANS_CLIENT_KEY=SB-Mid-client-test-key-mock
   MIDTRANS_ENVIRONMENT=sandbox
   CORS_ALLOWED_ORIGINS=https://app.carefund.org
   TRUSTED_PROXY_CIDRS=
   METRICS_HOST=0.0.0.0
   METRICS_PORT=9090
   WORKER_METRICS_HOST=0.0.0.0
   WORKER_METRICS_PORT=9091
   PAYMENT_PENDING_TTL=45m
   OUTBOX_PROCESSING_TTL=15m
   ```
3. Confirm that `.env.production.local` is ignored by `.gitignore` to prevent secret commits.

---

## 3. Image Build

Build the production multi-target images locally from the repository root:

```bash
# 1. Build API Server Image
docker build -t carefund-server:production --target server -f apps/api/Dockerfile apps/api

# 2. Build Worker Daemon Image
docker build -t carefund-worker:production --target worker -f apps/api/Dockerfile apps/api

# 3. Build Migration CLI Image
docker build -t carefund-migrate:production --target migrate -f apps/api/Dockerfile apps/api
```

Verify the images:
```bash
docker images | grep carefund
```
Confirm non-root user (`appuser`, UID 10001) and minimal footprint (~25–35 MB per image).

---

## 4. Local Database Initialization (Disposable PostgreSQL with TLS)

Because `ENV=production` enforces `DB_SSLMODE=require`, start a local PostgreSQL container with self-signed TLS certificates:

```bash
# 1. Generate local certificates
mkdir -p scratch/pg_certs
docker run --rm -v "$(pwd)/scratch/pg_certs:/certs" alpine:3.21 sh -c "
  apk add --no-cache openssl && \
  openssl req -new -x509 -days 365 -nodes -out /certs/server.crt -keyout /certs/server.key -subj '/CN=carefund-pg-prodlike' && \
  chmod 600 /certs/server.key && chmod 644 /certs/server.crt && chown -R 70:70 /certs
"

# 2. Create isolated Docker bridge network
docker network create carefund-internal 2>/dev/null || true

# 3. Start PostgreSQL container
docker run -d --name carefund-pg-prodlike \
  --network carefund-internal \
  -e POSTGRES_USER=carefund_prod \
  -e POSTGRES_PASSWORD=carefund_prod_pass_2026 \
  -e POSTGRES_DB=carefund_production \
  -v "$(pwd)/scratch/pg_certs:/certs:ro" \
  postgres:16-alpine \
  postgres -c ssl=on -c ssl_cert_file=/certs/server.crt -c ssl_key_file=/certs/server.key
```

---

## 5. Migration Execution

Execute forward schema migrations (`000001` through `000025`):

```bash
docker compose --env-file .env.production.local -f docker-compose.prod.yml run --rm migrate up
```

Verify the schema version:
```bash
docker compose --env-file .env.production.local -f docker-compose.prod.yml run --rm migrate version
```
Expected output: `Database schema version version=25 dirty=false`.

---

## 6. Stack Startup

Start the backend application services via Docker Compose:

```bash
docker compose --env-file .env.production.local -f docker-compose.prod.yml up -d
```

Start the Caddy edge reverse proxy:
```bash
docker run -d --name carefund-caddy \
  --network carefund-internal \
  -p 80:80 \
  -p 443:443 \
  -e DOMAIN=localhost \
  -v "$(pwd)/deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
  caddy:2-alpine
```

---

## 7. Health & Readiness Verification

Run health checks against the running containers:

```bash
# 1. API Direct Liveness
docker exec carefund-api wget -qO- http://127.0.0.1:8080/health
# Expected: {"status":"ok"}

# 2. API Direct Readiness (Database Ping)
docker exec carefund-api wget -qO- http://127.0.0.1:8080/ready
# Expected: {"status":"ready"}

# 3. Edge Reverse Proxy Ingress (HTTPS via Caddy)
curl -k -s https://localhost/health
# Expected: {"status":"ok"}
```

---

## 8. Monitoring Verification

Verify Prometheus scraping and alert rule evaluation:

```bash
# 1. Prometheus Target Health
docker run --rm --network carefund-internal curlimages/curl:8.12.1 -s http://carefund-prometheus:9090/api/v1/targets

# 2. Verify Scrape Targets:
# - api:9090 -> health: "up"
# - worker:9091 -> health: "up" (requires CORS_ALLOWED_ORIGINS remediation)
```

---

## 9. Container Restart & Re-creation

To simulate a deployment restart or host reboot:

```bash
# Graceful stop
docker compose --env-file .env.production.local -f docker-compose.prod.yml stop

# Start stack
docker compose --env-file .env.production.local -f docker-compose.prod.yml up -d
```

Verify that existing database records persist and migrations remain at version 25.

---

## 10. Local Backup Procedure

Generate a full database snapshot using PostgreSQL-native custom archive format:

```bash
docker exec carefund-pg-prodlike pg_dump -U carefund_prod -d carefund_production -Fc -f /tmp/backup_carefund.dump
```

Copy the backup archive to the host:
```bash
docker cp carefund-pg-prodlike:/tmp/backup_carefund.dump ./backup_$(date +%Y%m%d_%H%M%S).dump
```

---

## 11. Local Restore Procedure

To restore the backup into a clean database:

```bash
# 1. Create clean target database
docker exec carefund-pg-prodlike psql -U carefund_prod -d postgres -c "CREATE DATABASE carefund_restored;"

# 2. Restore schema and data
docker exec carefund-pg-prodlike pg_restore -U carefund_prod -d carefund_restored /tmp/backup_carefund.dump

# 3. Verify restored records
docker exec carefund-pg-prodlike psql -U carefund_prod -d carefund_restored -c "SELECT count(*) FROM users;"
docker exec carefund-pg-prodlike psql -U carefund_prod -d carefund_restored -c "SELECT version, dirty FROM schema_migrations;"
```

---

## 12. Application Rollback Simulation

If a newly deployed image fails startup checks:

```bash
# 1. Stop degraded container
docker compose --env-file .env.production.local -f docker-compose.prod.yml stop api

# 2. Roll back image tag in Compose or environment
# e.g., deploy previous known-good tag: carefund-server:v1
docker run -d --name carefund-api-rollback --network carefund-internal \
  carefund-server:production

# 3. Verify database state is untouched (no down migrations executed)
docker exec carefund-pg-prodlike psql -U carefund_prod -d carefund_production -c "SELECT version, dirty FROM schema_migrations;"
```

---

## 13. Complete Stack Teardown & Cleanup

To cleanly remove all local test containers, networks, and scratch certificates:

```bash
# Stop and remove compose containers and volumes
docker compose --env-file .env.production.local -f docker-compose.prod.yml down -v

# Remove Caddy and PostgreSQL
docker rm -f carefund-caddy carefund-pg-prodlike 2>/dev/null || true

# Remove bridge network
docker network rm carefund-internal 2>/dev/null || true

# Remove local certs and env file
rm -rf scratch/pg_certs .env.production.local
```
