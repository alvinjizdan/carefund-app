# CareFund — Product Requirements Document (PRD)

**Version:** 1.0  
**Date:** 2026-08-20  
**Status:** Draft / Implementation Baseline

## 1. Product Overview

CareFund adalah platform charity crowdfunding yang memungkinkan pengguna membuat campaign, berdonasi melalui payment gateway Midtrans, dan melihat transparansi status dana serta aktivitas campaign.

Prinsip utama CareFund:

1. **Trust** — donor dapat mengetahui ke mana dana dialokasikan.
2. **Traceability** — setiap transaksi finansial dapat ditelusuri dari donation → payment → settlement/refund.
3. **Security** — perubahan status finansial hanya boleh dilakukan oleh backend Go.
4. **Consistency** — webhook Midtrans harus idempotent dan tidak boleh menyebabkan double processing.
5. **Separation of concerns** — Next.js menangani UI, NestJS menjadi BFF/API gateway bila diperlukan, dan Go menjadi business/financial authority.

## 2. Technology Stack

| Layer | Technology | Responsibility |
|---|---|---|
| Web | Next.js | UI, SSR/CSR, SEO, user experience |
| BFF | NestJS | API aggregation, session/user-facing orchestration, request shaping |
| Core Backend | Go | Business rules, authentication authority, donation/payment state, settlement, refund |
| Database | PostgreSQL | Authoritative persistent data |
| Payment | Midtrans | Payment processing |
| Async Worker | Go Worker | Reconciliation, campaign completion, settlement jobs, outbox delivery |
| Cache | Optional Redis | Rate limiting/cache/job coordination if needed |
| Object Storage | Optional S3-compatible storage | Campaign images/documents |

> **Important architecture decision:** NestJS bukan frontend. Karena requirement menyebut Next.js & NestJS sebagai frontend dan Go sebagai backend, dokumen ini menempatkan Next.js sebagai frontend dan NestJS sebagai BFF. Go tetap menjadi backend/domain/financial authority.

## 3. Goals

### MVP Goals

- User dapat register/login.
- User dapat melihat campaign.
- User dapat membuat campaign.
- User dapat melakukan donation.
- Donation dapat dibayar melalui Midtrans.
- Sistem dapat menerima dan memproses webhook Midtrans.
- Status pembayaran dapat dilihat donor.
- Campaign memiliki target, periode, dan status.
- Sistem dapat mencatat refund.
- Sistem memiliki audit trail untuk perubahan penting.
- Sistem memiliki settlement record setelah campaign memenuhi syarat settlement.
- Admin dapat memoderasi campaign dan mengelola proses settlement.

### Non-Goals MVP

- Mobile native application.
- Multi-currency.
- Multi-country payment provider.
- Complex recommendation engine.
- Cryptocurrency.
- Direct peer-to-peer fund transfer.
- Automated payout ke banyak beneficiary jika legal/compliance flow belum ditentukan.

## 4. User Roles

### Donor

- Register/login.
- Browse campaign.
- Donate.
- Melakukan pembayaran melalui Midtrans.
- Melihat riwayat donation.
- Melihat status pembayaran.

### Campaign Owner

- Membuat campaign.
- Mengubah campaign sebelum masuk status aktif/locked.
- Melihat donation campaign.
- Melihat status campaign dan settlement.

### Admin

- Review campaign.
- Approve/reject campaign.
- Suspend campaign.
- Review donation/payment anomalies.
- Trigger/review refund sesuai policy.
- Review settlement.
- Melihat audit logs.

### System Worker

- Reconciliation.
- Campaign completion.
- Settlement preparation.
- Outbox delivery.
- Retry failed operations.

## 5. Campaign Requirements

Campaign minimal memiliki:

- title
- slug
- description
- target_amount
- current_amount
- start_at
- end_at
- category
- owner
- status
- cover image

Campaign status:

```text
DRAFT
PENDING_REVIEW
REJECTED
ACTIVE
SUSPENDED
COMPLETED
CANCELLED
```

Rules:

- Donation hanya dapat dibuat untuk campaign `ACTIVE`.
- Campaign tidak boleh menerima donation setelah `end_at`.
- Campaign yang `SUSPENDED` tidak boleh menerima donation baru.
- `current_amount` bukan source of truth finansial; nilai tersebut merupakan projection/cache dari successful eligible donations.
- Source of truth finansial berasal dari payment/donation records yang memenuhi financial eligibility rules.

## 6. Donation Requirements

Donation minimal memiliki:

- donor
- campaign
- amount
- anonymous flag
- message
- status
- created_at

Donation status:

```text
PENDING
PAID
FAILED
EXPIRED
REFUNDED
PARTIALLY_REFUNDED
CANCELLED
```

Financial rule:

> Donation dianggap memenuhi syarat finansial hanya setelah payment dinyatakan settled/eligible berdasarkan status provider dan aturan settlement CareFund.

## 7. Payment Requirements

Payment harus menyimpan:

- internal payment ID
- donation ID
- Midtrans order ID
- Midtrans transaction ID
- payment method/type
- gross amount
- provider status
- internal status
- transaction timestamps
- raw provider event reference

Internal payment status:

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

**Payment Expiration Policy (TTL):**
- **Payment Pending TTL**: 45 minutes from payment intent creation (`created_at`).
- Reconciliation must occur before local expiration to verify provider status.
- A provider timeout or 5xx does not equal failure; it remains PENDING for retry.
- `PENDING` is not collected funds and does not inflate campaign totals.
- `CAPTURED` is not `SETTLED`.
- Provider verification is strictly required before expiration.
- If the provider reports `NOT FOUND` and the payment age is >= 45 minutes, it is definitively treated as `EXPIRED` under the MVP policy.

Payment state tidak boleh diubah langsung oleh frontend.

## 8. Midtrans Requirements

CareFund menggunakan Midtrans sebagai payment gateway.

Recommended MVP integration:

- Midtrans Snap untuk checkout.
- Backend Go membuat Snap transaction/token.
- Frontend menampilkan Snap payment UI.
- Midtrans mengirim notification/webhook ke backend Go.
- Backend Go memverifikasi notification.
- Backend Go melakukan idempotent state transition.
- Jika webhook terlambat/tidak diterima, reconciliation dapat menggunakan Midtrans Get Status API.

Midtrans mendokumentasikan Snap transaction endpoint dan notification flow. Notification dikirim ke merchant backend setelah status transaksi berubah. citeturn0search1turn0search6

> Frontend redirect/result dari Midtrans **bukan source of truth** untuk status pembayaran. Webhook/status API Midtrans yang diproses oleh Go menjadi sumber perubahan status payment.

## 9. Refund Requirements (Phase 5H Hardened)

Refund lifecycle & execution flow:
- **Local Intent & Reservation**: Request membuat record Refund berstatus PENDING dan OutboxEvent berstatus PENDING dalam transaksi database atomik dengan SELECT ... FOR UPDATE pada baris payments.
- **Reservation Rule**: ctive_refunds = SUM(PENDING + COMPLETED). Request baru ditolak jika 
equested_amount > GrossAmount - active_refunds.
- **Outbox Worker Dispatch**: Outbox worker mengeksekusi request ke Midtrans Direct Refund API di luar transaksi database.
- **Provider Idempotency**: Mengirimkan idempotency_key lokal sebagai 
efund_key Midtrans untuk menjamin at-least-once retry tidak memicu duplikasi refund di provider.
- **Ambiguous Failure Principle**: Timeout/5xx tidak boleh langsung menandai refund sebagai FAILED. Refund tetap PENDING dan outbox di-retry dengan backoff (1m, 2m, 5m, 10m, 30m, 60m+). Status definitif ditentukan oleh provider confirmation atau reconciliation.
- **State Synchronization**: Setelah provider mengonfirmasi refund (COMPLETED), status payments dan donations disinkronisasikan menjadi REFUNDED (jika total completed == gross amount) atau PARTIALLY_REFUNDED (jika total completed < gross amount).
- **Reservation Release**: Jika provider secara definitif menolak refund (FAILED), status refund diubah ke FAILED dan dana reservasi otomatis kembali dapat digunakan.

Invariant:

`	ext
SUM(active_refunds) <= payment.gross_amount
`

## 10. Settlement Requirements

Settlement merupakan proses internal CareFund setelah campaign selesai.

Settlement tidak sama dengan payment success.

```text
payment captured/settled
        ↓
financially eligible
        ↓
campaign completed
        ↓
settlement calculated
        ↓
settlement approved
        ↓
settlement executed
```

Settlement record harus immutable setelah `APPROVED`.

## 11. Transparency Requirements

Donor dapat melihat:

- campaign target
- jumlah dana terkumpul
- jumlah donor
- campaign status
- campaign period
- donation history sesuai privacy policy
- payment status milik dirinya
- refund status jika ada

Admin dapat melihat:

- complete payment trail
- webhook events
- refund trail
- settlement calculation
- audit log

## 12. Security Requirements

- Password harus di-hash menggunakan Argon2id atau bcrypt dengan parameter aman.
- Access token/session tidak boleh dipercaya hanya berdasarkan frontend.
- Financial endpoints wajib authorization.
- Midtrans secret key hanya berada di backend Go.
- Webhook Midtrans harus diverifikasi.
- Webhook processing harus idempotent.
- Semua mutation penting harus memiliki audit trail.
- Rate limiting untuk authentication, donation creation, webhook, dan admin endpoints.
- Input validation pada semua API boundary.
- Database transaction digunakan untuk state transition finansial.
- Jangan pernah menggunakan floating point untuk nominal IDR; gunakan integer smallest currency unit.

## 13. Observability

Minimal:

- structured logging
- request ID / correlation ID
- payment ID
- donation ID
- campaign ID
- Midtrans order ID
- metrics untuk payment success/failure
- webhook processing latency
- failed webhook count
- settlement processing errors

## 14. Acceptance Criteria MVP

### Donation

- User memilih campaign ACTIVE.
- User memasukkan nominal valid.
- Sistem membuat donation + payment dalam transaction.
- Go membuat transaction ke Midtrans.
- Frontend memperoleh Snap token.
- User membayar.
- Midtrans mengirim notification.
- Go memverifikasi notification.
- Payment transition diproses secara idempotent.
- Donation berubah ke status yang sesuai.

### Duplicate Webhook

Jika notification yang sama dikirim 2x atau lebih:

- hanya satu state transition yang berlaku.
- tidak terjadi duplicate donation.
- tidak terjadi duplicate settlement eligibility.
- event tetap tercatat untuk audit/debug.

### Campaign Completion

Setelah campaign berakhir:

- worker menemukan campaign yang eligible.
- campaign berubah menjadi COMPLETED.
- settlement calculation dibuat.
- settlement tidak dapat dihitung dua kali.

## 15. Future Enhancements

- Automated payout.
- Multiple beneficiaries.
- KYC/KYB.
- Tax/reporting module.
- Advanced fraud detection.
- Recurring donation.
- Email/WhatsApp notification.
- Public transparency ledger.
