# CareFund — API Specification

**Version:** 1.0  
**Protocol:** REST / JSON  
**Base Public API:** `/api/v1`

## 1. API Architecture

```text
Client
  |
  v
NestJS BFF
  |
  v
Go Core API

Midtrans
  |
  v
Go Core API /api/v1/webhooks/midtrans
```

NestJS is the public frontend-facing API boundary.

Go is the authoritative internal API.

## 2. Conventions

### Headers

```http
Content-Type: application/json
Authorization: Bearer <access_token>
X-Request-ID: <uuid>
```

For idempotent mutation:

```http
Idempotency-Key: <unique-key>
```

### Response Envelope

Success:

```json
{
  "data": {},
  "meta": {}
}
```

Error:

```json
{
  "error": {
    "code": "CAMPAIGN_NOT_FOUND",
    "message": "Campaign not found",
    "details": {}
  },
  "request_id": "uuid"
}
```

## 3. Authentication

### POST /api/v1/auth/register

Create user.

Request:

```json
{
  "name": "Alvin",
  "email": "alvin@example.com",
  "password": "strong-password"
}
```

Response `201`:

```json
{
  "data": {
    "user": {
      "id": "uuid",
      "name": "Alvin",
      "email": "alvin@example.com",
      "roles": ["DONOR"]
    },
    "access_token": "token"
  }
}
```

### POST /api/v1/auth/login

Request:

```json
{
  "email": "alvin@example.com",
  "password": "strong-password"
}
```

### POST /api/v1/auth/refresh

Refresh access session/token.

### POST /api/v1/auth/logout

Invalidate session/refresh token.

### GET /api/v1/me

Get authenticated user.

## 4. Campaign API

### GET /api/v1/campaigns

Query:

```text
?page=1
&page_size=20
&category=education
&status=ACTIVE
&search=school
```

Response:

```json
{
  "data": [
    {
      "id": "uuid",
      "title": "Help Children Go To School",
      "slug": "help-children-go-to-school",
      "target_amount": 100000000,
      "current_amount": 35000000,
      "status": "ACTIVE",
      "start_at": "2026-08-01T00:00:00Z",
      "end_at": "2026-09-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

### GET /api/v1/campaigns/:campaign_id

Return campaign detail.

### POST /api/v1/campaigns

Auth: Campaign Owner

Request:

```json
{
  "title": "Help Children Go To School",
  "description": "Campaign description",
  "category_id": "uuid",
  "target_amount": 100000000,
  "start_at": "2026-09-01T00:00:00Z",
  "end_at": "2026-10-01T00:00:00Z"
}
```

Response `201`.

### PATCH /api/v1/campaigns/:campaign_id

Auth: owner/admin.

Only mutable fields allowed by campaign state.

### POST /api/v1/campaigns/:campaign_id/submit-review

Submit draft for moderation.

### POST /api/v1/campaigns/:campaign_id/approve

Auth: Admin.

### POST /api/v1/campaigns/:campaign_id/reject

Auth: Admin.

Request:

```json
{
  "reason": "Insufficient campaign documentation"
}
```

### POST /api/v1/campaigns/:campaign_id/suspend

Auth: Admin.

### GET /api/v1/campaigns/:campaign_id/donations

Return public donation summary/list according to privacy rules.

## 5. Donation API

### POST /api/v1/donations

Auth: optional/required depending on anonymous donation policy.

Headers:

```http
Idempotency-Key: donation-request-unique-id
```

Request:

```json
{
  "campaign_id": "uuid",
  "amount": 50000,
  "is_anonymous": true,
  "message": "Semoga bermanfaat."
}
```

Response `201`:

```json
{
  "data": {
    "donation_id": "uuid",
    "payment_id": "uuid",
    "amount": 50000,
    "status": "PENDING"
  }
}
```

Important:

- Go validates campaign state.
- Go validates amount.
- Go creates donation and payment atomically.
- Duplicate `Idempotency-Key` must return the original result.

### GET /api/v1/donations/:donation_id

Return donation detail.

### GET /api/v1/me/donations

Return current user's donation history.

## 6. Payment API

### POST /api/v1/payments/:payment_id/snap-token

Create/retrieve Midtrans Snap token.

Auth: owner of donation or permitted guest flow.

Response:

```json
{
  "data": {
    "payment_id": "uuid",
    "order_id": "CF-20260820-ABC123",
    "snap_token": "midtrans-snap-token",
    "expires_at": "2026-08-20T17:00:00Z"
  }
}
```

The Go backend is responsible for calling Midtrans.

Midtrans's Snap transaction endpoint is:

```text
POST /snap/v1/transactions
```

with sandbox and production endpoints documented by Midtrans. citeturn0search6

### GET /api/v1/payments/:payment_id

Return internal payment status.

Response:

```json
{
  "data": {
    "id": "uuid",
    "order_id": "CF-20260820-ABC123",
    "status": "SETTLED",
    "gross_amount": 50000,
    "payment_type": "qris",
    "transaction_at": "2026-08-20T16:00:00Z",
    "settled_at": "2026-08-20T16:01:00Z"
  }
}
```

## 7. Midtrans Webhook

### POST /api/v1/webhooks/midtrans

Auth:

- provider signature verification
- optional source IP allowlist where applicable
- no user authentication

Request body follows the selected Midtrans notification contract.

Example Core/Snap notification:

```json
{
  "transaction_time": "2026-08-20 23:00:00",
  "transaction_status": "settlement",
  "transaction_id": "provider-transaction-id",
  "order_id": "CF-20260820-ABC123",
  "gross_amount": "50000.00",
  "payment_type": "qris",
  "fraud_status": "accept"
}
```

Processing requirements:

1. Verify notification authenticity.
2. Locate payment by `order_id`.
3. Generate idempotency key.
4. Insert `payment_events`.
5. If duplicate event, return HTTP 200.
6. Lock payment row.
7. Validate state transition.
8. Update payment.
9. Update donation projection.
10. Write audit event.
11. Write outbox event.
12. Commit transaction.
13. Return HTTP 200.

Midtrans documents transaction notification handling and recommends checking status/fraud fields and using Get Status API when a notification is delayed. citeturn0search0turn0search2

## 8. Payment Status Mapping

Provider status should be normalized into CareFund internal status.

Example:

| Midtrans | CareFund |
|---|---|
| pending | PENDING |
| capture | CAPTURED |
| settlement | SETTLED |
| deny | FAILED |
| cancel | CANCELLED |
| expire | EXPIRED |
| failure | FAILED |
| refund | REFUNDED / PARTIALLY_REFUNDED |

Exact mapping must be implemented according to the payment method and latest Midtrans contract.

## 9. Payment Status Reconciliation

### POST /api/v1/internal/payments/:payment_id/reconcile

Auth: internal worker/admin only.

Process:

1. call Midtrans Get Status API.
2. compare provider status with internal status.
3. apply valid transition.
4. record reconciliation event.

Midtrans provides:

```text
GET /v2/{order_id}/status
```

for transaction status lookup. citeturn0search5

## 10. Refund API

### POST /api/v1/payments/:payment_id/refunds

Auth: Admin.

Request:

```json
{
  "amount": 50000,
  "reason": "Campaign cancelled"
}
```

Headers:

```http
Idempotency-Key: refund-request-unique-id
```

Rules:

```text
amount > 0
amount <= remaining_refundable_amount
```

Transaction must lock the payment row.

Response:

```json
{
  "data": {
    "refund_id": "uuid",
    "payment_id": "uuid",
    "amount": 50000,
    "status": "PENDING"
  }
}
```

## 11. Settlement API

### GET /api/v1/admin/campaigns/:campaign_id/settlement

Auth: Admin / authorized campaign owner.

### POST /api/v1/admin/campaigns/:campaign_id/settlement/calculate

Auth: Admin.

This should normally be worker-driven, but the endpoint may exist for controlled administrative recalculation.

### POST /api/v1/admin/settlements/:settlement_id/approve

Auth: Admin.

Once approved, settlement becomes immutable.

### POST /api/v1/admin/settlements/:settlement_id/execute

Auth: Admin / settlement operator.

MVP may only record execution state if actual payout integration is not yet implemented.

## 12. Admin API

### GET /api/v1/admin/campaigns

Moderation queue.

### GET /api/v1/admin/payments

Payment monitoring.

Query:

```text
?status=PENDING
&from=2026-08-01
&to=2026-08-20
```

### GET /api/v1/admin/payment-events

Webhook/payment event monitoring.

### GET /api/v1/admin/refunds

Refund monitoring.

### GET /api/v1/admin/audit-logs

Audit trail.

## 13. Category API

### GET /api/v1/categories

Public active categories.

### POST /api/v1/admin/categories

Admin only.

### PATCH /api/v1/admin/categories/:category_id

Admin only.

### DELETE /api/v1/admin/categories/:category_id

Prefer deactivation instead of destructive delete.

## 14. HTTP Status Codes

| Status | Usage |
|---|---|
| 200 | Successful read/update |
| 201 | Resource created |
| 202 | Async processing accepted |
| 204 | Successful operation without response body |
| 400 | Validation error |
| 401 | Authentication required/invalid |
| 403 | Forbidden |
| 404 | Resource not found |
| 409 | Conflict/state transition/idempotency conflict |
| 422 | Business rule violation |
| 429 | Rate limit |
| 500 | Internal error |
| 502 | External provider failure |
| 503 | Temporary service unavailable |

## 15. Error Codes

Examples:

```text
INVALID_REQUEST
UNAUTHORIZED
FORBIDDEN
CAMPAIGN_NOT_FOUND
CAMPAIGN_NOT_ACTIVE
CAMPAIGN_EXPIRED
DONATION_NOT_FOUND
DONATION_AMOUNT_INVALID
PAYMENT_NOT_FOUND
PAYMENT_ALREADY_SETTLED
PAYMENT_INVALID_STATE_TRANSITION
MIDTRANS_REQUEST_FAILED
MIDTRANS_NOTIFICATION_INVALID
PAYMENT_EVENT_DUPLICATE
REFUND_AMOUNT_EXCEEDED
REFUND_NOT_ALLOWED
SETTLEMENT_ALREADY_EXISTS
SETTLEMENT_NOT_ELIGIBLE
SETTLEMENT_IMMUTABLE
IDEMPOTENCY_CONFLICT
RATE_LIMITED
```

## 16. API Security Rules

- Never trust client-provided payment status.
- Never expose Midtrans Server Key.
- Never allow frontend to directly update payment status.
- Never allow frontend to directly update settlement.
- Webhook must not require user JWT.
- Webhook must verify provider authenticity.
- Admin endpoints require explicit role authorization.
- All mutation endpoints validate request body.
- Sensitive errors must not expose secrets/provider credentials.

## 17. API Contract Rule

The API specification is the contract between:

```text
Next.js
   ↕
NestJS
   ↕
Go
```

The implementation must not silently change:

- field names
- enum values
- HTTP status semantics
- required fields
- idempotency behavior

without updating this document and affected clients.
