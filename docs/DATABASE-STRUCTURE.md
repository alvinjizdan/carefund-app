# CareFund — Database Structure

**Version:** 1.0  
**Database:** PostgreSQL 16+  
**Purpose:** Database implementation reference for Codex / Antigravity

---

## 1. Database Architecture

PostgreSQL adalah **authoritative data store** CareFund.

Application flow:

```text
Next.js
   |
NestJS BFF
   |
Go Core API
   |
PostgreSQL
   |
Go Worker
```

Important rule:

> Hanya Go Core API dan Go Worker yang boleh melakukan perubahan langsung terhadap database. Next.js dan NestJS tidak boleh mengakses PostgreSQL secara langsung.

Financial data harus diperlakukan sebagai immutable/auditable records. Hindari hard delete untuk donation, payment, refund, settlement, dan audit data.

---

# 2. Naming Convention

Gunakan:

- table: `snake_case`, plural
- column: `snake_case`
- primary key: `id`
- foreign key: `<entity>_id`
- timestamp: `created_at`, `updated_at`
- PostgreSQL `uuid` untuk entity identifier
- `bigint` untuk nominal uang
- `timestamptz` untuk timestamp
- `jsonb` untuk payload/event data

Contoh:

```sql
campaigns
campaign_id
target_amount
created_at
```

---

# 3. Money Representation

Semua nominal uang menggunakan:

```sql
BIGINT
```

Jangan gunakan:

```sql
FLOAT
REAL
DOUBLE PRECISION
```

Contoh:

```text
Rp50.000
```

disimpan sebagai:

```text
50000
```

Karena CareFund menggunakan IDR, tidak diperlukan decimal fraction untuk nominal MVP.

Financial invariant:

```text
amount > 0
```

---

# 4. UUID

Gunakan UUID sebagai primary key.

Recommended PostgreSQL approach:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

Kemudian:

```sql
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

UUID digunakan agar identifier tidak mudah ditebak dan lebih aman untuk exposure melalui API dibanding sequential integer.

---

# 5. Core Tables

Database MVP terdiri dari:

```text
users
roles
user_roles

categories
campaigns

donations
payments
payment_events
refunds

settlements
settlement_items

outbox_events
audit_logs
```

Relationship utama:

```text
User
 ├── Campaign
 │     └── Donation
 │           └── Payment
 │                 ├── Payment Events
 │                 └── Refund
 │
 └── Audit Log

Campaign
 └── Settlement
       └── Settlement Items
```

---

# 6. users

Menyimpan account user.

## Columns

| Column | Type | Constraint | Description |
|---|---|---|---|
| id | uuid | PK | User ID |
| email | citext | UNIQUE, NOT NULL | Email login |
| password_hash | text | NOT NULL | Hashed password |
| name | varchar(150) | NOT NULL | Display/full name |
| phone | varchar(30) | NULL | Phone number |
| is_active | boolean | NOT NULL | Account active flag |
| created_at | timestamptz | NOT NULL | Creation time |
| updated_at | timestamptz | NOT NULL | Last update |

Recommended:

```sql
CREATE EXTENSION IF NOT EXISTS citext;
```

Email harus case-insensitive.

Example:

```text
ALVIN@example.com
alvin@example.com
```

harus dianggap account yang sama.

---

# 7. roles

Role-based authorization.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| name | varchar(50) | UNIQUE, NOT NULL |
| created_at | timestamptz | NOT NULL |

Initial roles:

```text
DONOR
CAMPAIGN_OWNER
ADMIN
```

System worker tidak perlu menjadi database user role. Worker merupakan application process dengan internal authorization.

---

# 8. user_roles

Many-to-many relationship antara users dan roles.

## Columns

| Column | Type | Constraint |
|---|---|---|
| user_id | uuid | PK, FK |
| role_id | uuid | PK, FK |

Primary key:

```sql
PRIMARY KEY (user_id, role_id)
```

Foreign key:

```sql
FOREIGN KEY (user_id) REFERENCES users(id)
FOREIGN KEY (role_id) REFERENCES roles(id)
```

---

# 9. categories

Kategori campaign.

Contoh:

```text
Education
Health
Disaster Relief
Social
Environment
```

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| name | varchar(100) | UNIQUE, NOT NULL |
| slug | varchar(120) | UNIQUE, NOT NULL |
| is_active | boolean | NOT NULL |
| created_at | timestamptz | NOT NULL |
| updated_at | timestamptz | NOT NULL |

Category sebaiknya tidak di-hard-delete jika sudah digunakan campaign.

Gunakan:

```text
is_active = false
```

---

# 10. campaigns

Campaign charity.

## Columns

| Column | Type | Constraint | Description |
|---|---|---|---|
| id | uuid | PK | Campaign ID |
| owner_id | uuid | FK users | Campaign owner |
| category_id | uuid | FK categories | Category |
| title | varchar(200) | NOT NULL | Campaign title |
| slug | varchar(240) | UNIQUE | Public URL slug |
| description | text | NOT NULL | Campaign description |
| target_amount | bigint | CHECK > 0 | Target IDR |
| current_amount | bigint | CHECK >= 0 | Projection |
| start_at | timestamptz | NOT NULL | Start time |
| end_at | timestamptz | NOT NULL | End time |
| status | varchar(30) | NOT NULL | Campaign state |
| rejection_reason | text | NULL | Admin rejection reason |
| created_at | timestamptz | NOT NULL | Creation |
| updated_at | timestamptz | NOT NULL | Update |

Campaign statuses:

```text
DRAFT
PENDING_REVIEW
REJECTED
ACTIVE
SUSPENDED
COMPLETED
CANCELLED
```

Constraint:

```sql
CHECK (target_amount > 0)
CHECK (current_amount >= 0)
CHECK (end_at > start_at)
```

## Important

`current_amount` adalah **projection**, bukan source of truth finansial.

Source of truth:

```text
payments
+
refunds
+
financial eligibility rules
```

`current_amount` boleh digunakan untuk read performance tetapi harus dapat direbuild/recalculate.

---

# 11. donations

Donation dibuat oleh donor terhadap campaign.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| campaign_id | uuid | FK campaigns |
| donor_id | uuid | FK users, NULL jika guest donation diizinkan |
| amount | bigint | CHECK > 0 |
| is_anonymous | boolean | NOT NULL |
| message | text | NULL |
| status | varchar(30) | NOT NULL |
| created_at | timestamptz | NOT NULL |
| updated_at | timestamptz | NOT NULL |

Donation statuses:

```text
PENDING
PAID
FAILED
EXPIRED
REFUNDED
PARTIALLY_REFUNDED
CANCELLED
```

MVP recommendation:

> Lebih sederhana jika donation membutuhkan authenticated user. Guest donation dapat ditambahkan kemudian.

---

# 12. payments

Payment adalah record transaksi payment gateway.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| donation_id | uuid | UNIQUE, FK donations |
| provider | varchar(30) | NOT NULL |
| order_id | varchar(100) | UNIQUE, NOT NULL |
| transaction_id | varchar(150) | NULL |
| payment_type | varchar(50) | NULL |
| gross_amount | bigint | CHECK > 0 |
| status | varchar(30) | NOT NULL |
| fraud_status | varchar(30) | NULL |
| transaction_at | timestamptz | NULL |
| settled_at | timestamptz | NULL |
| expired_at | timestamptz | NULL |
| created_at | timestamptz | NOT NULL |
| updated_at | timestamptz | NOT NULL |

Payment statuses:

```text
PENDING
AUTHORIZED
CAPTURED
SETTLED
FAILED
EXPIRED
CANCELLED
REFUNDED
PARTIALLY_REFUNDED
```

Provider:

```text
MIDTRANS
```

MVP hanya membutuhkan satu provider tetapi kolom `provider` dipertahankan agar desain extensible.

---

# 13. payment_events

Menyimpan setiap notification/event dari Midtrans.

Table ini sangat penting untuk:

- webhook idempotency
- audit
- debugging
- reconciliation
- provider event history

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| payment_id | uuid | NULL, FK payments |
| provider | varchar(30) | NOT NULL |
| idempotency_key | varchar(255) | UNIQUE, NOT NULL |
| event_type | varchar(100) | NOT NULL |
| provider_status | varchar(50) | NULL |
| payload | jsonb | NOT NULL |
| received_at | timestamptz | NOT NULL |
| processed_at | timestamptz | NULL |
| processing_status | varchar(30) | NOT NULL, CHECK (RECEIVED, PROCESSED, REJECTED) |
| rejection_reason | varchar(100) | NULL |

Processing status:

```text
RECEIVED
PROCESSED
DUPLICATE
FAILED
```

## Idempotency

Recommended key:

```text
hash(
  provider
  + order_id
  + transaction_status
  + transaction_id
)
```

Jika provider menyediakan event identifier yang stabil, gunakan identifier tersebut.

Constraint:

```sql
UNIQUE(idempotency_key)
```

---

# 14. refunds

Menyimpan refund terhadap payment.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| payment_id | uuid | FK payments |
| idempotency_key | varchar(100) | UNIQUE, NOT NULL |
| amount | bigint | CHECK > 0 |
| provider_refund_id | varchar(150) | NULL |
| status | varchar(30) | NOT NULL |
| reason | text | NOT NULL |
| requested_at | timestamptz | NOT NULL |
| completed_at | timestamptz | NULL |

Refund statuses:

```text
PENDING
COMPLETED
FAILED
CANCELLED
```

## Critical Invariant & Reservation Semantics

```text
active_refunds =
    SUM(PENDING + COMPLETED)

refundable_amount =
    payment.gross_amount - active_refunds
```

- In-flight `PENDING` refunds reserve the refundable balance so concurrent or multiple refund requests cannot exceed the payment's gross amount.
- `FAILED` or `CANCELLED` refunds release their reservation automatically because they are excluded from `active_refunds`.
- `COMPLETED` refunds are provider-confirmed and trigger state synchronization with `payments` and `donations`.

Refund transaction locking hierarchy:

```text
Payment (FOR UPDATE)
   ↓
Refund (FOR UPDATE)
   ↓
Donation (UpdateStatus)
```

Tujuannya mencegah concurrent balance depletion dan deadlocks. Network calls ke Midtrans tidak boleh dilakukan ketika database locks sedang dipegang.

---

# 15. settlements

Settlement merupakan hasil perhitungan dana campaign setelah campaign selesai.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| campaign_id | uuid | UNIQUE, FK campaigns |
| gross_amount | bigint | CHECK >= 0 |
| refund_amount | bigint | CHECK >= 0 |
| platform_fee | bigint | CHECK >= 0 |
| net_amount | bigint | CHECK >= 0 |
| status | varchar(30) | NOT NULL |
| calculated_at | timestamptz | NULL |
| approved_at | timestamptz | NULL |
| executed_at | timestamptz | NULL |
| created_at | timestamptz | NOT NULL |
| updated_at | timestamptz | NOT NULL |

Settlement statuses:

```text
CALCULATING
CALCULATED
APPROVED
EXECUTING
EXECUTED
FAILED
CANCELLED
```

## Important

```sql
UNIQUE(campaign_id)
```

Artinya satu campaign hanya memiliki satu settlement.

Settlement harus immutable setelah:

```text
APPROVED
```

---

# 16. settlement_items

Detail donation/payment yang masuk ke settlement.

Table ini membuat settlement dapat diaudit.

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| settlement_id | uuid | FK settlements |
| donation_id | uuid | FK donations |
| payment_id | uuid | FK payments |
| eligible_amount | bigint | CHECK >= 0 |
| created_at | timestamptz | NOT NULL |

Relationship:

```text
settlement
    |
    +-- settlement_item
    +-- settlement_item
    +-- settlement_item
```

Eligible amount dihitung berdasarkan payment yang memenuhi financial eligibility.

Contoh:

```text
Donation             Rp100.000
Payment settled      Rp100.000
Refund               Rp20.000
------------------------------
Eligible              Rp80.000
```

Settlement item:

```text
eligible_amount = 80000
```

---

# 17. outbox_events

Transactional Outbox Pattern.

Digunakan untuk event asynchronous.

Contoh event:

```text
DONATION_PAID
PAYMENT_SETTLED
REFUND_COMPLETED
CAMPAIGN_COMPLETED
SETTLEMENT_APPROVED
SETTLEMENT_EXECUTED
```

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| aggregate_type | varchar(50) | NOT NULL |
| aggregate_id | uuid | NOT NULL |
| event_type | varchar(100) | NOT NULL |
| payload | jsonb | NOT NULL |
| status | varchar(30) | NOT NULL |
| retry_count | integer | NOT NULL |
| available_at | timestamptz | NOT NULL |
| processed_at | timestamptz | NULL |
| created_at | timestamptz | NOT NULL |

Statuses:

```text
PENDING
PROCESSING
PROCESSED
FAILED
```

Worker menggunakan:

```sql
FOR UPDATE SKIP LOCKED
```

untuk mengambil job secara aman.

---

# 18. audit_logs

Append-only audit trail.

Digunakan untuk perubahan sensitif:

- campaign approval
- campaign rejection
- payment status transition
- refund
- settlement
- admin action
- role changes

## Columns

| Column | Type | Constraint |
|---|---|---|
| id | uuid | PK |
| actor_user_id | uuid | FK users, NULL for system |
| action | varchar(100) | NOT NULL |
| entity_type | varchar(50) | NOT NULL |
| entity_id | uuid | NOT NULL |
| before_data | jsonb | NULL |
| after_data | jsonb | NULL |
| request_id | varchar(100) | NULL |
| created_at | timestamptz | NOT NULL |

Contoh:

```json
{
  "action": "PAYMENT_STATUS_CHANGED",
  "entity_type": "payment",
  "before_data": {
    "status": "PENDING"
  },
  "after_data": {
    "status": "SETTLED"
  }
}
```

Audit logs jangan di-update atau di-delete oleh application biasa.

---

# 19. Foreign Key Map

```text
users
  ├── user_roles.user_id
  ├── campaigns.owner_id
  ├── donations.donor_id
  └── audit_logs.actor_user_id

roles
  └── user_roles.role_id

categories
  └── campaigns.category_id

campaigns
  ├── donations.campaign_id
  └── settlements.campaign_id

donations
  ├── payments.donation_id
  └── settlement_items.donation_id

payments
  ├── payment_events.payment_id
  ├── refunds.payment_id
  └── settlement_items.payment_id

settlements
  └── settlement_items.settlement_id
```

---

# 20. Recommended SQL Constraints

Minimum constraints:

```sql
CHECK (target_amount > 0);

CHECK (current_amount >= 0);

CHECK (amount > 0);

CHECK (gross_amount > 0);

CHECK (refund_amount >= 0);

CHECK (platform_fee >= 0);

CHECK (net_amount >= 0);

CHECK (eligible_amount >= 0);
```

Campaign:

```sql
CHECK (end_at > start_at);
```

---

# 21. Recommended Indexes

## Campaign

```sql
CREATE INDEX idx_campaigns_status_end_at
ON campaigns(status, end_at);

CREATE INDEX idx_campaigns_owner_id
ON campaigns(owner_id);

CREATE INDEX idx_campaigns_category_id
ON campaigns(category_id);
```

## Donations

```sql
CREATE INDEX idx_donations_campaign_id
ON donations(campaign_id);

CREATE INDEX idx_donations_donor_id
ON donations(donor_id);

CREATE INDEX idx_donations_status
ON donations(status);
```

## Payments

```sql
CREATE UNIQUE INDEX idx_payments_order_id
ON payments(order_id);

CREATE INDEX idx_payments_status
ON payments(status);

CREATE INDEX idx_payments_donation_id
ON payments(donation_id);
```

## Payment Events

```sql
CREATE UNIQUE INDEX idx_payment_events_idempotency_key
ON payment_events(idempotency_key);

CREATE INDEX idx_payment_events_payment_id
ON payment_events(payment_id);
```

## Refunds

```sql
CREATE INDEX idx_refunds_payment_id
ON refunds(payment_id);

CREATE INDEX idx_refunds_status
ON refunds(status);
```

## Settlement

```sql
CREATE UNIQUE INDEX idx_settlements_campaign_id
ON settlements(campaign_id);

CREATE INDEX idx_settlements_status
ON settlements(status);
```

## Outbox

```sql
CREATE INDEX idx_outbox_events_pending
ON outbox_events(status, available_at);
```

## Audit

```sql
CREATE INDEX idx_audit_logs_entity
ON audit_logs(entity_type, entity_id);

CREATE INDEX idx_audit_logs_actor
ON audit_logs(actor_user_id);

CREATE INDEX idx_audit_logs_created_at
ON audit_logs(created_at);
```

---

# 22. Soft Delete Policy

MVP tidak perlu generic `deleted_at` pada semua table.

Do not delete:

```text
donations
payments
payment_events
refunds
settlements
settlement_items
audit_logs
```

Untuk entity seperti:

```text
users
categories
campaigns
```

gunakan status/active flag bila perlu.

Contoh:

```text
users.is_active = false
categories.is_active = false
campaigns.status = CANCELLED
```

---

# 23. Transaction Boundaries

## Create Donation

Satu database transaction:

```text
BEGIN

validate campaign

INSERT donation

INSERT payment

INSERT audit/event if required

COMMIT
```

Jika Midtrans transaction creation dilakukan setelah DB commit, sistem harus memiliki recovery strategy.

Alternative:

```text
Create donation/payment
    ↓
commit
    ↓
call Midtrans
    ↓
update payment
```

Jika Midtrans gagal:

```text
payment = FAILED
```

jangan menghapus donation/payment record.

---

# 24. Webhook Transaction

Webhook Midtrans:

```text
BEGIN

find payment by order_id

INSERT payment_event

if duplicate:
    COMMIT
    return 200

SELECT payment FOR UPDATE

validate state transition

UPDATE payments

UPDATE donations

INSERT audit_logs

INSERT outbox_events

COMMIT
```

Never process payment state outside a transaction.

---

# 25. Refund Transaction

```text
BEGIN

SELECT payment
FOR UPDATE

calculate refundable amount

validate requested amount

INSERT refund

UPDATE payment status if necessary

INSERT audit log

INSERT outbox event

COMMIT
```

---

# 26. Settlement Transaction

```text
BEGIN

SELECT campaign
FOR UPDATE

verify campaign completed

verify settlement does not already exist

SELECT eligible payments

calculate gross

calculate refunds

calculate fees

calculate net

INSERT settlement

INSERT settlement_items

INSERT audit log

INSERT outbox event

COMMIT
```

---

# 27. Financial Eligibility Rule

Settlement hanya boleh mengambil payment yang memenuhi:

```text
payment.status = SETTLED
AND payment.settled_at IS NOT NULL
```

Kemudian:

```text
eligible_amount =
settled_payment_amount
-
successful_refund_amount
```

Do not use:

```text
donation.status = PAID
```

sebagai satu-satunya dasar settlement.

---

# 28. Campaign Current Amount

`campaigns.current_amount` adalah denormalized projection.

Recommended calculation:

```text
current_amount =
SUM(eligible contribution)
for eligible donations belonging to campaign
```

Ketika payment settled:

```text
current_amount += eligible amount
```

Ketika refund completed:

```text
current_amount -= refunded amount
```

Untuk recovery, worker/admin tool harus dapat rebuild projection dari authoritative records.

---

# 29. Database Migration Strategy

Gunakan migration tool seperti:

```text
golang-migrate
```

atau tool migration PostgreSQL yang setara.

Migration files:

```text
000001_extensions.up.sql
000002_users.up.sql
000003_roles.up.sql
000004_categories.up.sql
000005_campaigns.up.sql
000006_donations.up.sql
000007_payments.up.sql
000008_payment_events.up.sql
000009_refunds.up.sql
000010_settlements.up.sql
000011_settlement_items.up.sql
000012_outbox_events.up.sql
000013_audit_logs.up.sql
```

Rollback files harus tersedia:

```text
000001_extensions.down.sql
...
```

---

# 30. Seed Data

Initial seed:

### Roles

```text
DONOR
CAMPAIGN_OWNER
ADMIN
```

### Categories

Contoh:

```text
Education
Health
Disaster Relief
Social
Environment
```

Jangan menyimpan password admin dalam source code.

Gunakan environment variable atau secure bootstrap mechanism.

---

# 31. Database Rules for Codex / Antigravity

Implementer MUST:

1. Follow this schema.
2. Never invent financial tables without updating this document.
3. Never change payment state directly from frontend.
4. Never use floating point for money.
5. Use database transactions for financial state transitions.
6. Use `SELECT ... FOR UPDATE` for concurrent refund and relevant settlement operations.
7. Preserve payment event history.
8. Preserve audit history.
9. Use foreign keys.
10. Add indexes based on access patterns.
11. Use migrations rather than manually modifying production schema.
12. Update this document if schema changes.
13. Keep settlement immutable after approval.
14. Keep provider payloads in `jsonb` where raw event preservation is required.
15. Never hard-delete financial records.

---

# 32. Database Source of Truth

The hierarchy is:

```text
PostgreSQL
    |
    +-- payments        <- financial transaction state
    |
    +-- refunds         <- refund state
    |
    +-- donations       <- donation lifecycle
    |
    +-- settlements     <- settlement result
    |
    +-- payment_events  <- provider event history
    |
    +-- audit_logs      <- audit history
```

Frontend values are never authoritative.

NestJS values are never authoritative.

Cached/projection values are never authoritative.

The Go domain layer is responsible for enforcing business rules, while PostgreSQL enforces persistence and integrity constraints.

---

# 33. Final Relationship Summary

```text
USERS
  |
  +---- USER_ROLES ---- ROLES
  |
  +---- CAMPAIGNS ---- CATEGORIES
  |         |
  |         +---- DONATIONS
  |                  |
  |                  +---- PAYMENTS
  |                         |
  |                         +---- PAYMENT_EVENTS
  |                         |
  |                         +---- REFUNDS
  |
  +---- AUDIT_LOGS

CAMPAIGNS
  |
  +---- SETTLEMENTS
           |
           +---- SETTLEMENT_ITEMS
                    |
                    +---- DONATIONS
                    +---- PAYMENTS

BUSINESS TRANSACTIONS
  |
  +---- OUTBOX_EVENTS
```

This schema is the implementation baseline for CareFund MVP.
