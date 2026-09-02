# CareFund 窶・Detailed System Design Document (DSD)

**Version:** 1.0  
**Architecture:** Next.js + NestJS BFF + Go Core Backend + Go Worker + PostgreSQL + Midtrans

## 1. Architecture Decision

CareFund menggunakan tiga application layers:

```text
Browser
   |
   v
Next.js
   |
   v
NestJS BFF
   |
   v
Go Core API
   |
   +----> PostgreSQL
   |
   +----> Midtrans
   |
   +----> Outbox
              |
              v
          Go Worker
```

### Why this architecture?

Next.js fokus pada presentation.

NestJS menjadi BFF untuk:

- request aggregation
- frontend-oriented DTO
- session/cookie handling bila dipilih
- API composition
- frontend-specific authorization checks

Go menjadi **sole business and financial authority**.

Go bertanggung jawab atas:

- donation rules
- campaign state
- payment state
- Midtrans integration
- webhook verification
- refund rules
- settlement
- audit
- financial invariants

NestJS tidak boleh melakukan financial state transition langsung ke PostgreSQL.

## 2. Service Responsibilities

### Next.js

Responsibilities:

- page rendering
- SSR/CSR
- SEO
- forms
- client-side state
- campaign browsing
- donation checkout UI
- admin UI

Never:

- store Midtrans server secret
- calculate authoritative settlement
- directly mutate PostgreSQL
- decide payment success

### NestJS BFF

Responsibilities:

- expose frontend-oriented API
- authenticate user session
- forward request to Go
- aggregate multiple Go API responses when beneficial
- normalize frontend DTO

Never:

- bypass Go business rules
- directly update financial tables
- process Midtrans webhook
- own settlement calculation

### Go Core API

Responsibilities:

- authentication authority
- campaign domain
- donation domain
- payment domain
- refund domain
- settlement domain
- Midtrans integration
- webhook verification
- database transaction
- audit logging

### Go Worker

Responsibilities:

- campaign completion sweep
- settlement preparation
- reconciliation
- outbox processing
- retry processing
- notification jobs

## 3. Repository Strategy

Recommended:

```text
carefund/
笏懌楳笏 apps/
笏・  笏懌楳笏 web/              # Next.js
笏・  笏懌楳笏 bff/              # NestJS
笏・  笏懌楳笏 api/              # Go API
笏・  笏披楳笏 worker/           # Go Worker
笏・笏懌楳笏 packages/
笏・  笏披楳笏 contracts/        # shared API schemas/types where appropriate
笏・笏懌楳笏 db/
笏・  笏懌楳笏 migrations/
笏・  笏披楳笏 seeds/
笏・笏懌楳笏 docs/
笏・  笏懌楳笏 PRD.md
笏・  笏懌楳笏 ERD.md
笏・  笏懌楳笏 DSD.md
笏・  笏披楳笏 API-SPECIFICATION.md
笏・笏懌楳笏 docker-compose.yml
笏披楳笏 README.md
```

If team size is small, separate repositories are also acceptable. Do not split services merely for the sake of microservices.

## 4. Go Backend Structure

Recommended modular monolith:

```text
apps/api/
笏懌楳笏 cmd/
笏・  笏披楳笏 server/
笏懌楳笏 internal/
笏・  笏懌楳笏 auth/
笏・  笏懌楳笏 users/
笏・  笏懌楳笏 campaigns/
笏・  笏懌楳笏 donations/
笏・  笏懌楳笏 payments/
笏・  笏懌楳笏 refunds/
笏・  笏懌楳笏 settlements/
笏・  笏懌楳笏 webhook/
笏・  笏懌楳笏 audit/
笏・  笏懌楳笏 outbox/
笏・  笏懌楳笏 platform/
笏・  笏披楳笏 shared/
笏懌楳笏 migrations/
笏披楳笏 pkg/
```

Avoid creating one Go service per domain in MVP.

## 5. NestJS Structure

```text
apps/bff/
笏懌楳笏 src/
笏・  笏懌楳笏 auth/
笏・  笏懌楳笏 campaigns/
笏・  笏懌楳笏 donations/
笏・  笏懌楳笏 payments/
笏・  笏懌楳笏 admin/
笏・  笏懌楳笏 users/
笏・  笏懌楳笏 common/
笏・  笏披楳笏 main.ts
```

NestJS controllers should call Go APIs, not repositories.

## 6. Next.js Structure

```text
apps/web/
笏懌楳笏 app/
笏・  笏懌楳笏 (public)/
笏・  笏懌楳笏 campaign/
笏・  笏懌楳笏 donate/
笏・  笏懌楳笏 dashboard/
笏・  笏披楳笏 admin/
笏懌楳笏 components/
笏懌楳笏 lib/
笏披楳笏 services/
```

## 7. Payment Architecture

Recommended flow:

```text
User
 竊・Next.js
 竊・NestJS
 竊・Go
 竊・Create Donation
 竊・Create Payment
 竊・Midtrans Snap
 竊・Return Snap Token
 竊・Next.js opens Snap
 竊・User pays
 竊・Midtrans
 竊・POST /webhooks/midtrans
 竊・Go verifies notification
 竊・DB transaction
 竊・Payment state transition
 竊・Donation projection update
 竊・Outbox event
```

Midtrans recommends using notification handling on the merchant backend and provides a Get Status API for checking the latest transaction status when needed. 訷cite訷Ｕurn0search1訷Ｕurn0search2訷Ｕurn0search5訷・
## 8. Payment State Machine

```text
PENDING
  |
  +--> CAPTURED
  |      |
  |      v
  |    SETTLED
  |
  +--> FAILED
  |
  +--> EXPIRED
  |
  +--> CANCELLED

SETTLED
  |
  +--> PARTIALLY_REFUNDED
  |
  +--> REFUNDED

**Payment Expiration Rule:**
- **TTL**: 45 minutes from payment intent creation (`created_at`).
- Reconciliation must verify status via `PaymentGateway.GetPaymentStatus()`.
- Provider timeout or 5xx leaves the payment `PENDING`.
- Provider `NOT FOUND` after 45 minutes safely transitions to `EXPIRED`.
- Late webhooks cannot revert `EXPIRED` to `CAPTURED` (blocked by state machine).
```

Go must reject invalid transitions.

## 9. Webhook Idempotency

Every Midtrans notification must have an internal idempotency key.

Recommended key:

```text
hash(
    provider
    + order_id
    + transaction_status
    + transaction_id
)
```

If Midtrans provides a stable event identifier in the selected integration, prefer that identifier.

Processing:

```text
receive webhook
    竊・verify authenticity
    竊・derive idempotency key
    竊・[Stage A: Audit Persistence]
    竊・INSERT payment_event (Status: RECEIVED)
    竊・if duplicate -> return 200
    竊・commit Stage A
    竊・[Stage B: Financial Mutation]
    竊・BEGIN TRANSACTION
    竊・lock payment FOR UPDATE
    竊・validate state transition and amount mismatch
    竊・update payment
    竊・update donation projection
    竊・update payment_event (Status: PROCESSED)
    竊・commit Stage B (return 200)

If Stage B is rejected safely (e.g. invalid transition):
    竊・update payment_event (Status: REJECTED, Reason: ...)
    竊・return 200
```

Midtrans documentation explicitly recommends robust notification handling and notes that provider notifications may be retried. 訷cite訷Ｕurn0search0訷Ｕurn0search2訷・
## 10. Database Transactions

Payment webhook transition must strictly separate the immutable audit event from the financial mutation.

Conceptually:

```text
[Stage A - Audit]
INSERT payment_event (RECEIVED)
COMMIT

[Stage B - Financial]
BEGIN
  lock payment FOR UPDATE
  validate transition / amount
  update payment
  update donation
  update payment_event -> PROCESSED
  insert audit_log
  insert outbox_event
COMMIT

[If Stage B Fails domain rules]
  update payment_event -> REJECTED
```

If the database or network crashes unexpectedly inside Stage B, it rolls back, but `payment_event` remains `RECEIVED`, which perfectly allows Midtrans to safely retry it.

## 11. Refund Concurrency

Refund flow:

```text
BEGIN

SELECT payment
FOR UPDATE

calculate:
already_refunded
remaining_amount

reject if requested_amount > remaining_amount

create refund

create audit log

COMMIT
```

This prevents two concurrent requests from refunding more than the payment amount.

## 12. Settlement

Worker periodically searches:

```text
campaign.status = ACTIVE
AND campaign.end_at <= NOW()
```

Then:

1. lock campaign.
2. verify campaign has not already been completed.
3. mark campaign COMPLETED.
4. calculate eligible settled payments.
5. exclude refunded amounts.
6. create settlement.
7. create settlement_items.
8. create audit event.
9. publish outbox event.

Settlement has unique `campaign_id`.

## 13. Outbox Pattern

When a business transaction needs an asynchronous event:

```text
DB transaction
 笏懌楳笏 business update
 笏披楳笏 outbox_events insert
```

Worker:

```text
SELECT pending outbox
FOR UPDATE SKIP LOCKED
```

Then deliver/process event.

This prevents the classic failure:

```text
DB COMMIT succeeded
but event publishing failed
```

## 14. Authentication

Recommended MVP:

- email/password
- access token
- refresh token or secure session
- role-based authorization

Token/session validation should ultimately be trusted by Go.

Example roles:

```text
DONOR
CAMPAIGN_OWNER
ADMIN
```

NestJS may reject obviously unauthorized requests, but Go must enforce domain authorization.

## 15. API Boundary

External client:

```text
Browser -> NestJS
```

Internal:

```text
NestJS -> Go
```

Webhook:

```text
Midtrans -> Go
```

Do not route Midtrans webhook through Next.js.

## 16. Security Boundaries

Secrets:

```text
NEXT_PUBLIC_*
```

must never contain:

- Midtrans Server Key
- database credentials
- JWT signing secret
- internal service credentials

Only Go should have Midtrans Server Key.

Midtrans documentation identifies server-side credentials and merchant notification configuration as part of the integration requirements. 訷cite訷Ｕurn0search4訷・
## 17. Environment Variables

### Next.js

```env
NEXT_PUBLIC_APP_URL=
NEXT_PUBLIC_BFF_URL=
```

### NestJS

```env
PORT=
GO_API_URL=
GO_INTERNAL_API_KEY=
SESSION_SECRET=
```

### Go API

```env
APP_ENV=
HTTP_PORT=
DATABASE_URL=
JWT_SECRET=
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_MERCHANT_ID=
MIDTRANS_BASE_URL=
MIDTRANS_NOTIFICATION_URL=
```

### Worker

```env
DATABASE_URL=
APP_ENV=
```

## 18. Deployment

MVP:

```text
Vercel
  笏披楳笏 Next.js

Cloud/VPS/Container
  笏懌楳笏 NestJS
  笏懌楳笏 Go API
  笏披楳笏 Go Worker

Managed PostgreSQL
  笏披楳笏 PostgreSQL
```

Midtrans communicates directly with Go API webhook endpoint.

## 19. Non-Functional Requirements

### Availability

Target MVP:

- 99.5% application availability.

### Performance

Target:

- public campaign API p95 < 500ms excluding external provider calls.
- internal Go API p95 < 300ms for normal CRUD.
- webhook acknowledgment should be fast after durable processing.

### Reliability

- webhook idempotency
- DB transactions
- retryable worker
- outbox pattern
- reconciliation

### Security

- TLS
- secret management
- password hashing
- RBAC
- rate limiting
- audit trail
- webhook verification

## 20. Implementation Order

Recommended coding order:

1. PostgreSQL migrations.
2. Go domain/core API.
3. Authentication.
4. Campaign module.
5. Donation module.
6. Midtrans integration.
7. Webhook module.
8. Refund module.
9. Settlement module.
10. Outbox/worker.
11. NestJS BFF.
12. Next.js UI.
13. Admin UI.
14. Observability and hardening.

The reason is deliberate: financial authority should exist before frontend flows are built around it.

## 15. Lock Hierarchy

To prevent database deadlocks, especially when multiple workers or webhooks process transactions concurrently, all transactions must acquire row-level locks in a strict top-down hierarchy.

### Authoritative Lock Graph
```text
campaigns
   ↓
payment_events
   ↓
payments
   ↓
refunds
   ↓
donations
```
*(with `outbox_events` independently claimed by OutboxWorker using `FOR UPDATE SKIP LOCKED`)*

### Use Case Specific Hierarchies:
1. **Settlement Calculation (`SettleCampaign`)**:
   - `campaigns` (FOR UPDATE) -> `settlements` -> `settlement_items` -> `payments` (UPDATE status) -> `campaigns` (UPDATE current_amount) -> `outbox_events` (INSERT)

2. **Refund Intent & Finalization (`ProcessLocalRefund` / `FinalizeRefund`)**:
   - `payments` (FOR UPDATE) -> `refunds` (FOR UPDATE) -> `donations` (UPDATE status) -> `outbox_events` (INSERT)

3. **Webhook Processing (`ProcessNotification`)**:
   - `payment_events` (INSERT RECEIVED) -> `payments` (FOR UPDATE) -> `refunds` (Sync if applicable) -> `donations` (UPDATE status) -> `payment_events` (UPDATE PROCESSED/REJECTED)

## 21. Cross-Phase Financial Integrity & Audit Invariants

### 1. Refund vs Settlement Invariants
- **Settlement Exclusion**: A payment is eligible for settlement IF AND ONLY IF `p.status = 'CAPTURED'`, `d.status = 'PAID'`, `p.gross_amount > 0`, `NOT EXISTS (settlement_items)`, AND `NOT EXISTS (refunds WHERE status IN ('PENDING', 'COMPLETED'))`.
- **Reservation Isolation**: Any payment with a `PENDING` or `COMPLETED` refund is strictly excluded from settlement calculation to prevent double-counting or settling reserved/refunded funds.
- **Post-Settlement Refunds**: Payments with status `SETTLED` remain eligible for refunds.

### 2. At-Least-Once Delivery vs Local Idempotency
- **Local Financial Operations**: Guaranteed exactly-once and atomic using PostgreSQL ACID transactions and `UNIQUE` constraints (`refunds.idempotency_key`, `outbox_events.idempotency_key`, `payment_events.idempotency_key`).
- **External Provider Operations**: At-least-once delivery with exponential retry backoff schedule (1m, 2m, 5m, 10m, 30m, 60m). Provider idempotency is enforced using deterministic `RefundKey` mapping to `refund.idempotency_key`.

### 3. Payment Event Audit Immutability
- Every incoming provider webhook or reconciliation notification is recorded as a `PaymentEvent` in `payment_events` before financial state mutation.
- Rejected events (e.g. invalid state transition, amount mismatch) are persisted with `processing_status = 'REJECTED'` and a descriptive `rejection_reason`.

