# CareFund Production Deployment Runbook

**Document Version:** 1.0.0
**Target Environment:** Production
**Target Architecture:** Single-Host Docker Compose + Edge Reverse Proxy + External Managed PostgreSQL
**Authoritative Services:** CareFund API, CareFund Worker, Caddy Reverse Proxy, Prometheus Observability
**External Dependencies:** Cloud Managed PostgreSQL 16, Midtrans Payment Gateway, Public DNS & Let's Encrypt ACME

---

## 1. Scope & Architecture

### 1.1 Overview
CareFund is a charitable donation and campaign management platform. The production deployment topology is designed for high reliability, minimal operational surface area, strict network isolation, and defense-in-depth security.

### 1.2 System Topology Diagram

```
                        [ Internet Traffic ]
                                 笏・                                 笏・HTTPS (:443) / HTTP (:80 -> 301)
                                 笆ｼ
                     笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・                     笏・  Caddy Reverse Proxy 笏・(Edge TLS, HSTS, Request Body Limit,
                     笏・  (Host Ports 80,443) 笏・ Metrics 404 Interception)
                     笏披楳笏笏笏笏笏笏笏笏笏笏笏ｬ笏笏笏笏笏笏笏笏笏笏笏笏・                                 笏・                                 笏・Internal Docker Bridge Network (`carefund-internal`)
                                 笏・Unexposed / Isolated Inter-Service Communication
                                 笆ｼ
             笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏ｴ笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・             笏・                                      笏・             笆ｼ :8080                                 笆ｼ :9091
笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・            笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・笏・      CareFund API       笏・            笏・    CareFund Worker      笏・笏・ - Public REST Endpoints 笏・            笏・ - Outbox Daemon (5s)    笏・笏・ - Financial Authority   笏・            笏・ - Reconciliation (1m)   笏・笏・ - Internal Metrics:9090 笏・            笏・ - Dedicated Metrics:9091笏・笏披楳笏笏笏笏笏笏笏笏笏笏笏笏ｬ笏笏笏笏笏笏笏笏笏笏笏笏笏笏・            笏披楳笏笏笏笏笏笏笏笏笏笏笏笏ｬ笏笏笏笏笏笏笏笏笏笏笏笏笏笏・             笏・                                      笏・             笏・          笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏､
             笏・          笏・                          笏・             笆ｼ           笆ｼ                           笆ｼ
     笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・       笏娯楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・     笏・  Managed PostgreSQL 16  笏・       笏・  Prometheus Monitoring  笏・     笏・ (External Cloud Host)   笏・       笏・ (Internal Metrics Scrape笏・     笏・ - TLS Transport Enforced笏・       笏・  api:9090, worker:9091) 笏・     笏・ - Authoritative Ledger  笏・       笏披楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・     笏披楳笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏笏・```

### 1.3 Component Roles & Boundaries

| Component | Container Name | Target Image | Responsibilities | Ingress / Exposure |
|-----------|----------------|--------------|------------------|-------------------|
| **Caddy** | `caddy` | `caddy:2-alpine` | Edge TLS termination (TLS 1.2/1.3), ACME certificate automation, request size enforcement (2MB), security headers (`-Server`, HSTS), blocking internal `/metrics*` endpoints with HTTP 404, reverse proxy to API `:8080`. | Public ports `80` (HTTP redirect) and `443` (HTTPS). |
| **CareFund API** | `carefund-api` | `carefund-server:production` | Business and financial domain authority, JWT authentication, user/campaign/donation management, Midtrans webhook ingestion. Runs as unprivileged `appuser` (UID 10001). | Internal `:8080` (API) and `:9090` (Metrics). **NOT published to host.** |
| **CareFund Worker** | `carefund-worker` | `carefund-worker:production` | Asynchronous background processing: transactional outbox sweeper (5s loop) and payment status reconciliation engine (1m loop). Runs as `appuser` (UID 10001). | Internal `:9091` (Metrics). **NOT published to host.** |
| **CareFund Migrate** | `carefund-migrate` | `carefund-migrate:production` | Forward-only database schema migration CLI (`000001` through `000025`). Runs as one-off pre-flight task as `appuser` (UID 10001). | Ephemeral container. Zero network ports exposed. |
| **Prometheus** | `carefund-prometheus` | `prom/prometheus:v2.51.0` | In-memory and TSDB metric collection, rule evaluation, 19 alert rules in 5 alert groups. | Internal `:9090` (Scraping/Web). **NOT published to host.** |
| **Managed PostgreSQL** | External Service | Cloud Provider | Authoritative ACID relational datastore. Managed by external cloud provider. | Direct private/TLS network access from API, Worker, and Migrate containers. |
| **Midtrans** | External Provider | Midtrans Gateway | External payment provider for Snap transaction tokens, status polling, and refunds. | External HTTPS outbound API calls; incoming webhooks via Caddy. |

---

## 2. Prerequisites

### 2.1 Host Infrastructure Requirements (Recommendations)
The following hardware recommendations are baseline operational sizing guidelines for typical low-to-medium volume workloads and should be evaluated against actual traffic:
- **Operating System:** Ubuntu 22.04 LTS or 24.04 LTS (x86_64 / amd64).
- **Compute (Recommended Baseline):** 2 vCPU, 4 GB RAM, 25 GB SSD.
- **Network Interfaces:** Static public IPv4 address (and IPv6 if available).
- **Firewall Rules (Host / Security Group):**
  - **Inbound:** Port `22` (SSH - restricted to admin IPs), Port `80` (HTTP - public for ACME and redirect), Port `443` (HTTPS - public).
  - **Inbound Denied:** All other ports (`5432`, `8080`, `9090`, `9091`, `2019`).
  - **Outbound:** Port `443` (HTTPS to Midtrans, ACME CAs, container registries), Port `5432` (PostgreSQL to managed database host).

### 2.2 Software Packages
The host system must have the following software installed:
- **Docker Engine:** Version `>= 24.0.0`
- **Docker Compose:** Plugin Version `>= v2.20.0`
- **Git:** Version `>= 2.34.0`
- **Curl / OpenSSL:** For local health inspections and secret generation

### 2.3 External Service Accounts & Credentials
- **Domain Name & DNS Provider:** Control over authoritative DNS records for the deployment domain.
- **Managed PostgreSQL 16 Instance:** Provisioned instance with active credentials, TLS capability, and network connectivity from the VPS.
- **Midtrans Production Account:** Production Server Key and Client Key from [Midtrans Production Dashboard](https://dashboard.midtrans.com).

---

## 3. Environment & Secrets Management

### 3.1 Environment Configuration File
Production configuration is governed by `.env.production.example`. Create the real production environment file:
```bash
cp .env.production.example .env.production
chmod 600 .env.production
```

> [!CAUTION]
> NEVER commit `.env.production` or any file containing real credentials to version control. Ensure `.env.production` remains ignored in `.gitignore`.

### 3.2 Environment Variable Classification

| Variable | Class | Category | Default / Example | Description & Constraints |
|----------|-------|----------|-------------------|---------------------------|
| `DOMAIN` | **Required** | Non-Secret | `api.carefund.id` | FQDN for public HTTPS ingress. Caddy uses this for ACME certificates. |
| `ENV` | **Required** | Non-Secret | `production` | Must be `production`. Enforces strict security validation at application startup. |
| `PORT` | Optional | Non-Secret | `8080` | Internal HTTP listening port for CareFund API. |
| `LOG_FORMAT` | Optional | Non-Secret | `json` | Log format. Must be `json` for structured log forwarding. |
| `DB_HOST` | **Required** | Non-Secret | `pg.provider.internal` | Fully qualified hostname or private IP of the managed database. |
| `DB_PORT` | Optional | Non-Secret | `5432` | PostgreSQL listening port. |
| `DB_USER` | **Required** | Non-Secret | `carefund_app` | Dedicated database role with least privilege. |
| `DB_PASSWORD` | **Required** | **SECRET** | `<generated-secret>` | Database password. Fallbacks are rejected in production. |
| `DB_NAME` | **Required** | Non-Secret | `carefund_production` | PostgreSQL database name. |
| `DB_SSLMODE` | **Required** | Non-Secret | `require` | Transport encryption. Must be `require`, `verify-ca`, or `verify-full`. `disable` is rejected. |
| `DB_MAX_OPEN_CONNS` | Optional | Non-Secret | `25` | Maximum active open database connections per service. |
| `DB_MAX_IDLE_CONNS` | Optional | Non-Secret | `10` | Maximum idle database connections retained in pool. |
| `DB_CONN_MAX_LIFETIME` | Optional | Non-Secret | `15m` | Maximum duration a connection may be reused. |
| `DB_STATEMENT_TIMEOUT`| Optional | Non-Secret | `15s` | Server-side query execution deadline. |
| `DB_LOCK_TIMEOUT` | Optional | Non-Secret | `5s` | Server-side table/row lock acquisition deadline. |
| `DB_IDLE_IN_TRANSACTION_TIMEOUT` | Optional | Non-Secret | `10s` | Server-side open transaction idle deadline. |
| `JWT_SECRET` | **Required** | **SECRET** | `<generated-secret>` | Cryptographic secret for signing HMAC-SHA256 JWT tokens. Must be at least 32 bytes. |
| `JWT_ACCESS_TTL` | Optional | Non-Secret | `15m` | Lifetime of user access tokens. |
| `MIDTRANS_SERVER_KEY`| **Required** | **SECRET** | `Mid-server-...` | Production Server Key from Midtrans Dashboard. |
| `MIDTRANS_CLIENT_KEY`| **Required** | Non-Secret | `Mid-client-...` | Production Client Key from Midtrans Dashboard. |
| `MIDTRANS_ENVIRONMENT`| **Required**| Non-Secret | `production` | Must be `production` for live transactions. |
| `PAYMENT_PENDING_TTL`| Optional | Non-Secret | `45m` | Window before uncompleted payments expire. |
| `OUTBOX_PROCESSING_TTL`| Optional | Non-Secret | `15m` | Lease duration for outbox message dispatch. |
| `CORS_ALLOWED_ORIGINS`| **Required** | Non-Secret | `https://app.carefund.id` | Comma-delimited list of allowed frontend origins. Rejects localhost in production. |
| `TRUSTED_PROXY_CIDRS`| Optional | Non-Secret | `172.18.0.0/16` | CIDR subnet of Caddy reverse proxy on internal Docker bridge. |
| `METRICS_HOST` | Optional | Non-Secret | `0.0.0.0` | API metrics bind address (accessible only on internal network). |
| `METRICS_PORT` | Optional | Non-Secret | `9090` | API metrics port. |
| `WORKER_METRICS_HOST` | Optional | Non-Secret | `0.0.0.0` | Worker metrics bind address (accessible only on internal network). |
| `WORKER_METRICS_PORT` | Optional | Non-Secret | `9091` | Worker metrics port. |

### 3.3 Secret Generation Procedures
To generate cryptographically secure secrets on the deployment host:
```bash
# Generate 256-bit JWT Secret (base64)
openssl rand -base64 32

# Generate Database Password
openssl rand -base64 24
```

---

## 4. DNS Setup & Verification

### 4.1 Required DNS Records
Configure the following DNS records with your authoritative DNS registrar:
- **Record Type:** `A` (and `AAAA` if IPv6 is enabled on host)
- **Host / Name:** Subdomain matching `DOMAIN` (e.g. `api.carefund.id`)
- **Value / Target:** Public IP address of the deployment VPS
- **TTL:** `300` (5 minutes during initial rollout for fast convergence)

### 4.2 External Verification
Verify DNS propagation from an external network before starting Caddy:
```bash
# Query public DNS for A record
dig +short A api.carefund.id @8.8.8.8

# Or using nslookup
nslookup api.carefund.id 8.8.8.8
```
> [!IMPORTANT]
> Do NOT start Caddy in production before DNS records resolve to the host IP. Starting Caddy with an unresolvable domain will trigger ACME validation failures and temporary rate limits from Let's Encrypt.

---

## 5. Managed PostgreSQL Preparation

### 5.1 Pre-Migration Connectivity Check
Before attempting migrations, verify network routing and database connectivity from the VPS host using `pg_isready` or `psql`:
```bash
# Test network socket and authentication
pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME"
```

### 5.2 Transport Encryption Validation
Verify that SSL/TLS is actively required by testing connectivity with TLS mode:
```bash
psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=$DB_SSLMODE" -c '\conninfo'
```
Expected output confirms SSL connection:
`You are connected to database "carefund_production" as user "carefund_app" on host "..." via socket in ... with SSL encryption...`

---

## 6. Deployment Sequence

Deployment must proceed in strict dependency order. Never start the public ingress or workers before the database schema has been verified.

```
[ Step 1: Preflight ] 笏笏笏笏笏笏笏笏笆ｺ Validate Compose Specification & Environment File
          笏・          笆ｼ
[ Step 2: Connectivity ] 笏笏笏笏笆ｺ Verify PostgreSQL Network & TLS Accessibility
          笏・          笆ｼ
[ Step 3: Migration ] 笏笏笏笏笏笏笏笆ｺ Run One-Off Container `carefund-migrate` (000001 -> 000025)
          笏・          笆ｼ
[ Step 4: API Service ] 笏笏笏笏笏笆ｺ Start `carefund-api` (Listens on internal :8080)
          笏・          笆ｼ
[ Step 5: Worker Service ] 笏笏笆ｺ Start `carefund-worker` (Initializes Outbox & Reconciliation)
          笏・          笆ｼ
[ Step 6: Prometheus ] 笏笏笏笏笏笏笆ｺ Start `carefund-prometheus` (Scrapes API & Worker)
          笏・          笆ｼ
[ Step 7: Caddy Ingress ] 笏笏笏笆ｺ Start Caddy Edge Proxy (Acquires TLS & routes public traffic)
          笏・          笆ｼ
[ Step 8: Verification ] 笏笏笏笏笆ｺ Execute Health, Readiness, and Security Audit Checks
```

### 6.1 Step 1: Preflight Configuration Validation
Validate the docker-compose template against the populated `.env.production`:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml config --quiet
```
Ensure exit code is `0`. If any required variable is missing, Compose will fail fast and print the offending variable name.

### 6.2 Step 2: Database Connectivity Check
Ensure the external PostgreSQL database is reachable before triggering migration containers.

### 6.3 Step 3: Forward Database Migration
Run the isolated migration container to apply all pending forward schema migrations:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate
```
Verify the output:
```text
[INFO] [component=Migrator] Executing forward database migrations... path=/app/migrations
[INFO] [component=Migrator] Database migrations applied successfully
[INFO] [component=Migrator] Current database schema status version=25 dirty=false
```
Then confirm the recorded version:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate version
```
Expected output: `Database schema version version=25 dirty=false`.

### 6.4 Step 4: Start CareFund API
Start the API service in the background:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d api
```
Inspect logs to confirm clean database initialization:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs api --tail 20
```

### 6.5 Step 5: Start CareFund Worker
Start the background processing worker:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d worker
```
Inspect logs to confirm worker loops have started:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs worker --tail 20
```

### 6.6 Step 6: Start Prometheus Observability
Start the internal Prometheus monitoring server:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d prometheus
```
Confirm Prometheus has parsed config and alert rules:
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs prometheus --tail 20
```

### 6.7 Step 7: Start Caddy Reverse Proxy
Start Caddy edge proxy:
```bash
docker run -d \
  --name carefund-caddy \
  --network carefund-internal \
  -p 80:80 \
  -p 443:443 \
  -e DOMAIN="$DOMAIN" \
  -v "$(pwd)/deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
  -v caddy_data:/data \
  -v caddy_config:/config \
  --restart unless-stopped \
  caddy:2-alpine
```
Inspect Caddy logs to confirm successful ACME certificate acquisition:
```bash
docker logs carefund-caddy --tail 30
```
Expected log entries:
- `"certificate obtained successfully"`
- `"serving initial configuration"`

---

## 7. Migration Operations & Recovery Protocol

### 7.1 Forward Migration Policy
All database schema changes in CareFund are **forward-only**. The migration CLI strictly forbids the automated `down` command in production (`cmd/migrate/main.go:51`).

### 7.2 Idempotency Rule
Running `migrate up` against an up-to-date database is a verified no-op. It logs `Database schema is already up to date (no change)` and exits with code `0`.

### 7.3 Dirty Migration Handling & Recovery
A migration enters a `dirty=true` state if a migration statement fails mid-execution (e.g. statement timeout, network interruption, lock acquisition failure, or SQL syntax error).

> [!CAUTION]
> **CRITICAL RECOVERY DIRECTIVE:**
> NEVER execute `UPDATE schema_migrations SET dirty = false` or `migrate force` as a knee-jerk recovery action to "unblock" a deployment. Setting `dirty=false` without first repairing physical table structures leaves partial tables, missing constraints, or orphaned indexes in place. When application code attempts to write against this corrupted schema, it causes silent financial discrepancies or immediate service crashes.
>
> The ONLY safe conceptual order is:
> **failure 竊・stop deployment 竊・inspect migration state 竊・determine exactly what was applied 竊・repair schema/data consistency 竊・verify consistency 竊・only then clear dirty state if justified 竊・rerun migration 竊・verify final schema**.

#### Canonical 9-Step Dirty Migration Recovery Protocol:

1. **Step 1: Acknowledge Failure & Stop Deployment Immediately:**
   Halt deployment CI/CD pipelines immediately. Stop API and Worker containers to prevent concurrent application writes against a partially migrated, non-deterministic database state:
   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml stop api worker
   ```

2. **Step 2: Inspect Migration State:**
   Query `schema_migrations` in PostgreSQL to confirm the failed version and dirty flag status:
   ```sql
   SELECT version, dirty FROM schema_migrations;
   ```
   Confirm that `dirty = true` and note the target version `<N>`.

3. **Step 3: Determine Exactly What Was Applied:**
   Inspect migration file `migrations/<version>_*.up.sql`. Connect to PostgreSQL and query `information_schema.tables`, `pg_attribute`, and PostgreSQL error logs to determine exactly which statements in `<version>` succeeded and which statement failed.
   ```sql
   -- Example: inspect existing columns on affected table
   SELECT column_name, data_type, is_nullable
   FROM information_schema.columns
   WHERE table_name = '<target_table>';
   ```

4. **Step 4: Capture Emergency Database Snapshot:**
   Before executing ANY corrective DDL/DML, create an immediate point-in-time database dump:
   ```bash
   pg_dump -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -Fc -f "/backups/emergency_dirty_v${N}_$(date +%s).dump"
   ```

5. **Step 5: Repair Schema & Data Consistency:**
   Depending on the nature of the failure, apply ONE of two manual remediation strategies:
   - **Strategy A (Complete Forward):** Manually execute the remaining unapplied statements of migration `<N>` inside an explicit transaction block (`BEGIN; ... COMMIT;`), ensuring all tables, constraints, and indexes exist.
   - **Strategy B (Rollback to N-1):** Manually execute reverse DDL to safely undo only the partial statements that succeeded from `<N>`, returning the physical schema cleanly to version `N-1`.

6. **Step 6: Verify Schema Consistency Independently:**
   Query PostgreSQL system catalogs to verify that the physical database schema strictly matches the intended state (either complete `<N>` or clean `<N-1>`). Confirm no orphaned locks or uncommitted transactions remain.

7. **Step 7: Clear Dirty State Only When Justified:**
   Only after physical schema consistency is verified in Step 6, update `schema_migrations`:
   ```sql
   -- If Strategy A (completed forward manually to version N):
   UPDATE schema_migrations SET dirty = false WHERE version = <N>;

   -- If Strategy B (reverted cleanly to prior version N-1):
   UPDATE schema_migrations SET version = <N-1>, dirty = false WHERE version = <N>;
   ```

8. **Step 8: Rerun Migration Command:**
   Execute `carefund-migrate` to prove that the database accepts migration processing:
   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate up
   ```
   If Strategy A was used, output will confirm: `Database schema is already up to date (no change)`.
   If Strategy B was used with a fixed migration file, the migrator will re-apply `<N>` cleanly.

9. **Step 9: Verify Final Schema Status:**
   Confirm schema version and clean state:
   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate version
   ```
   Must output: `Database schema version version=<N> dirty=false`. Only then resume API and Worker services.

---

## 8. Post-Deployment Verification

Execute these verification checks immediately following deployment:

### 8.1 Process & Container Health
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
```
All containers (`carefund-api`, `carefund-worker`, `carefund-prometheus`, `carefund-caddy`) must report status `Up` (or `Up (healthy)`).

### 8.2 API Liveness & Readiness
```bash
# Internal Liveness Check (Process alive)
docker exec carefund-api wget -qO- http://127.0.0.1:8080/health
# Expected: {"status":"ok"}

# Internal Readiness Check (Database ping valid)
docker exec carefund-api wget -qO- http://127.0.0.1:8080/ready
# Expected: {"status":"ready"}
```

### 8.3 Worker Liveness & Telemetry
```bash
docker exec carefund-worker wget -qO- http://127.0.0.1:9091/metrics | grep worker_last_heartbeat_timestamp_seconds
```
Verify that `outbox` and `reconciliation` heartbeats have timestamps within the last 120 seconds.

### 8.4 Prometheus Targets & Alerts
```bash
# Check scrape target health
docker exec carefund-prometheus wget -qO- http://127.0.0.1:9090/api/v1/targets | grep -o '"health":"[^"]*"'
# Expected: All targets report "health":"up"

# Check active alert rules
docker exec carefund-prometheus wget -qO- http://127.0.0.1:9090/api/v1/rules
# Expected: All 19 rules present in 5 alert groups with "health":"ok"
```

### 8.5 Public HTTPS Ingress & Routing Verification
Execute from an external machine:
```bash
# Verify HTTPS response and security headers
curl -I -s https://api.carefund.id/health
```
Expected output:
- `HTTP/2 200`
- `strict-transport-security: max-age=31536000`
- **NO `Server` header** (verified stripped)

### 8.6 Metrics Endpoint Isolation Check
Execute from an external machine:
```bash
curl -I -s https://api.carefund.id/metrics
curl -I -s https://api.carefund.id/metrics/subpath
```
Expected output: `HTTP/2 404 Not Found`. Caddy must intercept and drop all external metrics access.

---

## 9. Rollback Procedures

### 9.1 Application Rollback vs. Database Rollback
> [!IMPORTANT]
> Application rollback and database schema rollback are strictly decoupled operations.
> Reverting container application binaries almost never requires rolling back database schema changes. Do NOT run database rollbacks unless a schema defect itself is causing active financial data corruption.

### 9.2 Application Rollback Protocol
When a newly deployed application image contains a software regression:
1. Re-tag or edit `docker-compose.prod.yml` to specify the previous stable image tag (e.g. `carefund-server:<previous-git-sha>`).
2. Re-create the running containers:
   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml up -d --no-deps api worker
   ```
3. Verify `/health` and `/ready`.

### 9.3 Database Rollback Protocol (Emergency Only)
If a migration introduced an incompatible schema constraint:
1. Stop API and Worker containers immediately.
2. Review the corresponding `migrations/<version>_*.down.sql` script manually.
3. Review the down migration for destructive statements (`DROP TABLE`, `DROP COLUMN`) that would cause permanent financial data loss.
4. Execute the rollback statements manually using `psql` within an explicit transaction where possible.
5. Update `schema_migrations` table to reflect the reverted version.

---

## 10. Operational Incident Procedures

### Summary Matrix

| Incident | Primary Alert | Immediate Containment | Investigation Focus |
|----------|---------------|-----------------------|---------------------|
| API Unavailable | `APIInstanceDown` / Caddy 502 | Restart container / check OOM | Check `docker logs carefund-api`, dmesg for OOM killer |
| Worker Unavailable | `WorkerInstanceDown` | Restart container | Check `docker logs carefund-worker` |
| Worker Heartbeat Missing | `WorkerHeartbeatMissing` | Restart worker container | Investigate database locks or Midtrans hanging HTTP calls |
| Outbox Backlog Elevated | `OutboxBacklogGrowing` | Verify Midtrans connectivity | Check outbox lease duration and failure logs |
| Outbox Dead-Letter Event | `OutboxDeadLetterDetected` | Review failed payload | Inspect `outbox_events` table for failure reasons |
| Reconciliation Mismatch | `PaymentReconciliationMismatchDetected` | Halt automated settlement | Review payment logs vs Midtrans transaction status |
| Aged Pending Payments | `PendingPaymentsAgedPastTTL` | Run manual reconciliation sweep | Investigate missing webhook delivery from Midtrans |
| Settlement Failed | `SettlementExecutionFailed` | Block payout execution | Verify campaign donation sums against payments ledger |
| Refund Errors | `RefundProviderErrorsDetected` | Hold refund requests | Check Midtrans account balance and API status |
| DB Pool Exhaustion | `PostgreSQLConnectionPoolExhaustion` | Restart API/Worker | Check PostgreSQL `pg_stat_activity` for stuck queries |
| High 5xx Rate | `APIHigh5xxErrorRate` | Check DB latency / errors | Inspect structured JSON logs for error codes |
| High Latency | `APIHighLatencyP99` | Check DB lock contention | Analyze DB statement durations and slow query logs |

### Detailed Procedures

#### 10.1 Outbox Dead-Letter Event (`OutboxDeadLetterDetected`)
- **Detection:** Prometheus alert fires when `outbox_events_dead_letter_total > 0`.
- **Immediate Containment:** The event has stopped retrying automatically; it is sequestered in `DEAD_LETTER` status and will not cause repeated crashes.
- **Investigation:**
  ```sql
  SELECT id, event_type, payload, retry_count, error_message, updated_at
  FROM outbox_events
  WHERE status = 'DEAD_LETTER'
  ORDER BY updated_at DESC LIMIT 10;
  ```
- **Recovery:** Resolve the underlying issue (e.g. invalid refund payload, upstream account configuration). Re-queue the event by updating its status:
  ```sql
  UPDATE outbox_events
  SET status = 'PENDING', retry_count = 0, error_message = NULL, execution_lease_until = NULL
  WHERE id = '<event-uuid>';
  ```
- **Verification:** Confirm worker picks up the event, metric increments success, and status becomes `COMPLETED`.

#### 10.2 PostgreSQL Connection Pool Exhaustion (`PostgreSQLConnectionPoolExhaustion`)
- **Detection:** Prometheus alert fires when `db_in_use_connections >= 23` for > 3 minutes.
- **Immediate Containment:** Inspect active queries; terminate blocking transactions.
- **Investigation:**
  ```sql
  SELECT pid, usename, client_addr, state, age(clock_timestamp(), query_start), query
  FROM pg_stat_activity
  WHERE state != 'idle' AND usename = 'carefund_app'
  ORDER BY query_start ASC;
  ```
- **Recovery:** Cancel long-running blocking queries:
  ```sql
  SELECT pg_cancel_backend(<pid>);
  ```
- **Verification:** Observe `db_in_use_connections` drop back to baseline (< 5).

---

## 11. Backup & Recovery Contract (Provider-Neutral)

Because the production cloud PostgreSQL provider has not yet been selected, the following operational targets define the required data durability criteria:

### 11.1 Target Service Level Objectives
- **Target Recovery Point Objective (RPO):** `< 1 hour` (operational target pending validation against the selected production infrastructure/provider; target achieved via continuous write-ahead log (WAL) archiving or automated cloud snapshot schedules).
- **Target Recovery Time Objective (RTO):** `< 4 hours` (operational target pending validation against the selected production infrastructure/provider; target achieved via automated volume restoration or standby instance promotion and cold-start verification).

> [!NOTE]
> These RPO and RTO values represent **operational targets pending validation against the selected production infrastructure/provider**, NOT an achieved SLA or cloud provider contract guarantee.

### 11.2 Backup Standards
1. **Automated Snapshots:** Full daily automated snapshots retained for 30 days.
2. **Point-In-Time Recovery (PITR):** Transaction log / WAL archiving enabled with 7-day retention window minimum.
3. **Encryption:** Backups encrypted at rest with AES-256 and in transit via TLS.
4. **Periodic Restoration Drills:** Monthly automated test restoration into an isolated verification database to confirm backup integrity.

---

## 12. Disaster Recovery Strategy

### 12.1 Disaster Scenarios & Playbooks

| Scenario | Blast Radius | Recovery Procedure |
|----------|--------------|-------------------|
| **Total VPS Loss** | Application stack down; database unaffected. | Provision fresh Linux host. Install Docker/Compose. Clone repository to verified release tag. Decrypt `.env.production` from off-site secrets vault. Run preflight and deploy sequence. Point DNS A record to new host IP. |
| **Database Corruption / Loss** | Relational state damaged. | Stop application containers immediately. Initiate Point-In-Time-Recovery (PITR) on managed database to the timestamp immediately preceding corruption. Validate ledger integrity. Restart application containers. |
| **Container Image Loss** | Deployment cannot pull images. | Re-execute multi-stage Docker build from source checkout using verified git commit SHA. |
| **DNS Provider Outage** | Ingress traffic dropped. | Update domain nameservers at registrar to secondary DNS provider with pre-configured A records. |

### 12.2 Critical Off-Host Assets Inventory
The following assets MUST exist outside the deployment host at all times:
1. Version-controlled source code repository (GitHub).
2. Off-site secrets vault (Bitwarden, AWS Secrets Manager, 1Password) storing `.env.production` keys.
3. Managed PostgreSQL cloud console access and off-site backup snapshots.
4. Domain registrar credentials with two-factor authentication (2FA).
5. Midtrans merchant dashboard credentials with 2FA.

---

## 13. Security Hardening Checklist

- [ ] **Zero Public Internal Ports:** Verified via `docker ps` that ports `5432`, `8080`, `9090`, and `9091` have NO host port mappings (`0.0.0.0:xxxx`).
- [ ] **Least Privilege User:** Verified via `id` in all application containers that processes run as UID `10001` (`appuser`).
- [ ] **No Secrets in Images:** Verified container filesystem `/app` contains no `.env` files, no `.git` directory, and no Go source files.
- [ ] **Compiler Absent:** Verified Go toolchain (`which go`) is absent from runtime containers.
- [ ] **Encrypted Database Transport:** `DB_SSLMODE` set to `require`, `verify-ca`, or `verify-full`.
- [ ] **CORS Origin Locked:** `CORS_ALLOWED_ORIGINS` explicitly set to production frontend domain(s); default localhost rejected.
- [ ] **Edge Security Headers:** Caddyfile enforces `Strict-Transport-Security "max-age=31536000"` and removes `Server` header.
- [ ] **Edge Metrics Shield:** Caddyfile enforces `respond /metrics* 404` to block public exposure of operational metrics.
- [ ] **Rate Limiting Active:** Application middleware enforces per-IP and per-user token bucket rate limits on sensitive endpoints (`/api/v1/auth/*`, `/api/v1/donations`, `/api/v1/webhooks/*`).
- [ ] **Trusted Proxy CIDR Restricted:** `TRUSTED_PROXY_CIDRS` restricted strictly to internal Caddy container/bridge subnet; broad RFC 1918 blocks forbidden.

---

## 14. Maintenance & Service Lifecycle

### 14.1 Graceful Shutdown Protocol
When `docker compose down` or `docker stop` is invoked, Docker sends `SIGTERM`:
1. **API Server:** Halts acceptance of new HTTP connections. Allows existing in-flight HTTP requests up to `15s` to complete. Shuts down metrics server concurrently.
2. **Worker Daemon:** Context cancellation signals background goroutines. Outbox worker finishes current event in flight. Reconciliation loop aborts sleep and completes current transaction. Clean shutdown completes within `15s`.
3. **Stop Grace Period:** Configured to `stop_grace_period: 20s` in Compose, providing 5 seconds of safety headroom above the application's internal 15s deadline.

### 14.2 Maintenance Mode Procedure
CareFund does not maintain an internal "maintenance mode" flag in code. When maintenance is required:
1. Update Caddy to temporarily return a 503 maintenance page for non-health routes:
   ```caddy
   handle /health {
       reverse_proxy api:8080
   }
   handle {
       respond "Service temporarily undergoing maintenance. Please retry shortly." 503
   }
   ```
2. Reload Caddy configuration seamlessly without dropping TCP connections:
   ```bash
   docker exec carefund-caddy caddy reload --config /etc/caddy/Caddyfile
   ```

---

## 15. Go / No-Go Deployment Checklist

Prior to approving any production deployment, all items must be checked:

| # | Verification Item | Command / Source | Required Result | Verified? |
|---|-------------------|------------------|-----------------|-----------|
| 1 | Git Release Tag | `git rev-parse HEAD` | Matches approved release SHA | [ ] |
| 2 | Compose Syntax | `docker compose ... config` | Exit code 0, no warnings | [ ] |
| 3 | Production Secrets | `.env.production` inspection | Non-empty, no placeholders | [ ] |
| 4 | External DB Connectivity | `pg_isready` | Accepting connections over TLS | [ ] |
| 5 | Migrations Applied | `migrate version` | `version=25 dirty=false` | [ ] |
| 6 | Container Health | `docker compose ps` | All services Up (healthy) | [ ] |
| 7 | API Liveness | `GET /health` | HTTP 200 `{"status":"ok"}` | [ ] |
| 8 | API Readiness | `GET /ready` | HTTP 200 `{"status":"ready"}` | [ ] |
| 9 | Worker Heartbeat | `worker_last_heartbeat...` | Updated < 120s ago | [ ] |
| 10 | Prometheus Scrapes | `/api/v1/targets` | Both targets `health: "up"` | [ ] |
| 11 | Edge TLS Active | `curl -I https://$DOMAIN` | HTTP/2 200, TLS 1.3 | [ ] |
| 12 | Metrics Shielded | `curl -I https://$DOMAIN/metrics`| HTTP/2 404 Not Found | [ ] |
| 13 | Host Exposure | `docker ps` | Zero internal ports on host | [ ] |
