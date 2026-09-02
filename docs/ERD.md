# CareFund — Entity Relationship Diagram (ERD)

**Version:** 1.0  
**Database:** PostgreSQL

## 1. Design Principles

1. PostgreSQL adalah authoritative data store.
2. Nominal uang menggunakan `BIGINT`, bukan floating point.
3. Foreign key wajib digunakan untuk integrity.
4. Financial records tidak boleh dihapus secara destructive.
5. Payment/provider event harus idempotent.
6. Settlement campaign bersifat 1:1.
7. Audit log bersifat append-only.
8. Status transition harus divalidasi oleh domain layer.

## 2. Core Entities

- users
- roles
- user_roles
- categories
- campaigns
- donations
- payments
- payment_events
- refunds
- settlements
- settlement_items
- outbox_events
- audit_logs

## 3. Mermaid ERD

```mermaid
erDiagram

    USERS {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar name
        varchar phone
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }

    ROLES {
        uuid id PK
        varchar name UK
        timestamptz created_at
    }

    USER_ROLES {
        uuid user_id PK, FK
        uuid role_id PK, FK
    }

    CATEGORIES {
        uuid id PK
        varchar name UK
        varchar slug UK
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }

    CAMPAIGNS {
        uuid id PK
        uuid owner_id FK
        uuid category_id FK
        varchar title
        varchar slug UK
        text description
        bigint target_amount
        bigint current_amount
        timestamptz start_at
        timestamptz end_at
        varchar status
        varchar rejection_reason
        timestamptz created_at
        timestamptz updated_at
    }

    DONATIONS {
        uuid id PK
        uuid campaign_id FK
        uuid donor_id FK
        bigint amount
        boolean is_anonymous
        text message
        varchar status
        timestamptz created_at
        timestamptz updated_at
    }

    PAYMENTS {
        uuid id PK
        uuid donation_id FK
        varchar provider
        varchar order_id UK
        varchar transaction_id
        varchar payment_type
        bigint gross_amount
        varchar status
        varchar fraud_status
        timestamptz transaction_at
        timestamptz settled_at
        timestamptz expired_at
        timestamptz created_at
        timestamptz updated_at
    }

    PAYMENT_EVENTS {
        uuid id PK
        uuid payment_id FK
        varchar provider
        varchar idempotency_key UK
        varchar event_type
        varchar provider_status
        jsonb payload
        timestamptz received_at
        timestamptz processed_at
        varchar processing_status
    }

    REFUNDS {
        uuid id PK
        uuid payment_id FK
        bigint amount
        varchar provider_refund_id
        varchar status
        varchar reason
        timestamptz requested_at
        timestamptz completed_at
    }

    SETTLEMENTS {
        uuid id PK
        uuid campaign_id UK, FK
        bigint gross_amount
        bigint refund_amount
        bigint platform_fee
        bigint net_amount
        varchar status
        timestamptz calculated_at
        timestamptz approved_at
        timestamptz executed_at
        timestamptz created_at
        timestamptz updated_at
    }

    SETTLEMENT_ITEMS {
        uuid id PK
        uuid settlement_id FK
        uuid donation_id FK
        uuid payment_id FK
        bigint eligible_amount
        timestamptz created_at
    }

    OUTBOX_EVENTS {
        uuid id PK
        varchar aggregate_type
        uuid aggregate_id
        varchar event_type
        jsonb payload
        varchar status
        int retry_count
        timestamptz available_at
        timestamptz processed_at
        timestamptz created_at
    }

    AUDIT_LOGS {
        uuid id PK
        uuid actor_user_id FK
        varchar action
        varchar entity_type
        uuid entity_id
        jsonb before_data
        jsonb after_data
        varchar request_id
        timestamptz created_at
    }

    USERS ||--o{ USER_ROLES : has
    ROLES ||--o{ USER_ROLES : grants

    USERS ||--o{ CAMPAIGNS : owns
    CATEGORIES ||--o{ CAMPAIGNS : categorizes

    USERS ||--o{ DONATIONS : makes
    CAMPAIGNS ||--o{ DONATIONS : receives

    DONATIONS ||--o| PAYMENTS : has
    PAYMENTS ||--o{ PAYMENT_EVENTS : generates
    PAYMENTS ||--o{ REFUNDS : receives

    CAMPAIGNS ||--o| SETTLEMENTS : settles
    SETTLEMENTS ||--o{ SETTLEMENT_ITEMS : contains
    DONATIONS ||--o{ SETTLEMENT_ITEMS : included
    PAYMENTS ||--o{ SETTLEMENT_ITEMS : included

    USERS ||--o{ AUDIT_LOGS : performs
```

## 4. Important Constraints

### Users

```sql
UNIQUE(email)
```

Email comparison should normally be case-insensitive. Prefer PostgreSQL `citext` or a normalized lowercase email strategy.

### Campaign

```sql
UNIQUE(slug)
CHECK(target_amount > 0)
CHECK(end_at > start_at)
```

### Donation

```sql
CHECK(amount > 0)
```

### Payment

```sql
UNIQUE(order_id)
CHECK(gross_amount > 0)
```

### Settlement

```sql
UNIQUE(campaign_id)
CHECK(gross_amount >= 0)
CHECK(refund_amount >= 0)
CHECK(platform_fee >= 0)
CHECK(net_amount >= 0)
```

A campaign can have at most one settlement.

### Refund

A refund must never make the total refunded amount exceed the payment amount.

This invariant is enforced by Go transaction logic with PostgreSQL row locking.

## 5. Financial Eligibility

Do not calculate settlement from all `DONATIONS` with `status = PAID` blindly.

Preferred eligibility:

```text
payment.status = SETTLED
AND payment.settled_at IS NOT NULL
AND payment is not fully refunded
```

For partial refund:

```text
eligible_amount = settled_amount - refunded_amount
```

## 6. Recommended Indexes

```sql
CREATE INDEX idx_campaigns_status_end_at
ON campaigns(status, end_at);

CREATE INDEX idx_donations_campaign_id
ON donations(campaign_id);

CREATE INDEX idx_donations_donor_id
ON donations(donor_id);

CREATE INDEX idx_payments_donation_id
ON payments(donation_id);

CREATE INDEX idx_payments_status
ON payments(status);

CREATE INDEX idx_payment_events_payment_id
ON payment_events(payment_id);

CREATE INDEX idx_outbox_events_status_available_at
ON outbox_events(status, available_at);

CREATE INDEX idx_audit_logs_entity
ON audit_logs(entity_type, entity_id);
```

## 7. Deletion Policy

Financial entities should not be hard-deleted:

- donations
- payments
- payment_events
- refunds
- settlements
- settlement_items

Use status transitions or archival policy instead.

## 8. ERD Implementation Notes

The ERD is intentionally designed so that:

```text
Donation
   ↓
Payment
   ↓
Payment Events
   ↓
Refund
   ↓
Settlement
```

can be audited end-to-end.
