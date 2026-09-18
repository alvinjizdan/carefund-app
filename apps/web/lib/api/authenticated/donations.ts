/**
 * CareFund Authenticated Donations & Payments API Client
 *
 * Provides typed, server-only data access for authenticated user donation history,
 * donation details, payment transaction details, and bounded payment polling.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Hard Server Boundary: Strictly restricted to Server Components, Route Handlers, and Server Actions.
 * 2. Authenticated Transport: Uses serverApiClient with automatic session cookie resolution.
 * 3. Cache Isolation: Enforces `cache: "no-store"` on all authenticated personal reads.
 *    (User-specific financial records MUST NEVER be stored in the shared Next.js Data Cache).
 * 4. Authoritative Authorization: Go API validates user ownership, campaign ownership, or admin role.
 *    Next.js does not perform authorization overrides or IDOR checks locally.
 * 5. Normalization: Explicit field-by-field mapping from Go backend PascalCase/snake_case to frontend camelCase DTOs.
 * 6. Pagination Semantics: Backend returns `{ data: [...] }` with no total_count or has_more.
 *    `hasMore = items.length === limit` is strictly an unauthoritative client heuristic.
 * 7. Bounded Polling Safety: Polling is a frontend safety mechanism (max 10 attempts, 3s interval),
 *    NOT a backend contract. Polling stops immediately on success, failure, refund, or auth error.
 * 8. Status Invariant: The frontend MUST NOT derive or mutate donation.status from payment.status.
 */

import "next/dist/compiled/server-only";
import { serverApiClient, ServerApiClient } from "../server";
import { ServerApiRequestOptions } from "../types";
import { validateMonetaryAmount } from "../public/campaigns";
import { CareFundApiError } from "../errors";
import {
  DonationStatus,
  PaymentStatus,
  DonationHistoryItem,
  DonationDetail,
  PaymentDetail,
  DonationCreationResult,
  DonationHistoryResult,
  PaymentPollingStatus,
  PaymentPollingResult,
  Donation,
  Payment,
  DonationListResult,
} from "./types";

if (typeof window !== "undefined") {
  throw new Error(
    "Security Violation: authenticated donations API is server-only and cannot be executed in the browser."
  );
}

// Re-export all domain types
export type {
  DonationStatus,
  PaymentStatus,
  DonationHistoryItem,
  DonationDetail,
  PaymentDetail,
  DonationCreationResult,
  DonationHistoryResult,
  PaymentPollingStatus,
  PaymentPollingResult,
  Donation,
  Payment,
  DonationListResult,
};

// Polling Safety Constants (Frontend Safety Policy)
export const POLL_INTERVAL_MS = 3000;
export const MAX_POLL_ATTEMPTS = 10;
export const MAX_POLLING_DURATION_MS = 30000;

// Redirect Validation Constants
export const ALLOWED_MIDTRANS_HOSTNAMES = Object.freeze([
  "app.midtrans.com",
  "app.sandbox.midtrans.com",
] as const);

// Exhaustive Polling Classification Groups (All 9 statuses represented)
export const CHECKOUT_SUCCESS_STATUSES: readonly PaymentStatus[] = Object.freeze([
  "CAPTURED",
  "SETTLED",
]);

export const CHECKOUT_FAILURE_STATUSES: readonly PaymentStatus[] = Object.freeze([
  "FAILED",
  "EXPIRED",
  "CANCELLED",
]);

export const CHECKOUT_REFUND_STATUSES: readonly PaymentStatus[] = Object.freeze([
  "REFUNDED",
  "PARTIALLY_REFUNDED",
]);

export const CHECKOUT_CONTINUE_STATUSES: readonly PaymentStatus[] = Object.freeze([
  "PENDING",
  "AUTHORIZED",
]);

/**
 * Validates a Midtrans redirect URL against an exact hostname whitelist.
 * Prohibits substring matching, startsWith, regex prefix, userinfo, or port confusion bypasses.
 */
export function isValidMidtransRedirectUrl(url: string | null | undefined): boolean {
  if (!url || typeof url !== "string") {
    return false;
  }
  try {
    const parsed = new URL(url);
    // 1. Protocol must be strictly https:
    if (parsed.protocol !== "https:") {
      return false;
    }
    // 2. Prohibit userinfo (username / password) to prevent credential confusion
    if (parsed.username || parsed.password) {
      return false;
    }
    // 3. Prohibit non-standard ports to prevent port confusion
    if (parsed.port && parsed.port !== "443") {
      return false;
    }
    // 4. Hostname exact match against whitelist
    return ALLOWED_MIDTRANS_HOSTNAMES.includes(
      parsed.hostname as (typeof ALLOWED_MIDTRANS_HOSTNAMES)[number]
    );
  } catch {
    return false;
  }
}

/**
 * Raw donation shape as serialized by the Go backend (PascalCase, no JSON tags).
 */
export interface RawBackendDonation {
  ID: string;
  CampaignID: string;
  DonorID: string | null;
  Amount: number;
  IsAnonymous: boolean;
  Message: string;
  Status: string;
  CreatedAt: string;
  UpdatedAt: string;
}

/**
 * Raw payment shape as serialized by the Go backend (PascalCase, no JSON tags).
 */
export interface RawBackendPayment {
  ID: string;
  DonationID: string;
  Provider: string;
  OrderID: string;
  TransactionID: string | null;
  PaymentType: string | null;
  GrossAmount: number;
  Status: string;
  FraudStatus: string | null;
  TransactionAt: string | null;
  SettledAt: string | null;
  ExpiredAt: string | null;
  CreatedAt: string;
  UpdatedAt: string;
}

/**
 * Raw donation creation response as serialized by Go backend buildDonationResponse (snake_case).
 */
export interface RawBackendDonationCreation {
  donation_id: string;
  payment_id: string;
  order_id: string;
  amount: number;
  status: string;
  payment_token?: string | null;
  redirect_url?: string | null;
}

export interface GetUserDonationsOptions extends ServerApiRequestOptions {
  limit?: number;
  offset?: number;
  client?: ServerApiClient;
}

export interface PaymentPollingOptions extends ServerApiRequestOptions {
  intervalMs?: number;
  maxAttempts?: number;
  maxDurationMs?: number;
  client?: ServerApiClient;
}

/**
 * Normalizes raw backend donation into frontend DonationDetail DTO.
 */
export function normalizeDonation(raw: RawBackendDonation): DonationDetail {
  if (!raw || typeof raw !== "object") {
    throw new Error("Invalid donation payload: expected an object");
  }

  const validAmount = validateMonetaryAmount(raw.Amount, "Amount");

  return {
    id: String(raw.ID ?? ""),
    campaignId: String(raw.CampaignID ?? ""),
    donorId: raw.DonorID ? String(raw.DonorID) : null,
    amount: validAmount,
    isAnonymous: Boolean(raw.IsAnonymous),
    message: String(raw.Message ?? ""),
    status: (raw.Status ?? "PENDING") as DonationStatus,
    createdAt: String(raw.CreatedAt ?? ""),
    updatedAt: String(raw.UpdatedAt ?? ""),
  };
}

/**
 * Normalizes raw backend payment into frontend PaymentDetail DTO.
 */
export function normalizePayment(raw: RawBackendPayment): PaymentDetail {
  if (!raw || typeof raw !== "object") {
    throw new Error("Invalid payment payload: expected an object");
  }

  const validGrossAmount = validateMonetaryAmount(raw.GrossAmount, "GrossAmount");

  return {
    id: String(raw.ID ?? ""),
    donationId: String(raw.DonationID ?? ""),
    provider: String(raw.Provider ?? ""),
    orderId: String(raw.OrderID ?? ""),
    transactionId: raw.TransactionID ? String(raw.TransactionID) : null,
    paymentType: raw.PaymentType ? String(raw.PaymentType) : null,
    grossAmount: validGrossAmount,
    status: (raw.Status ?? "PENDING") as PaymentStatus,
    fraudStatus: raw.FraudStatus ? String(raw.FraudStatus) : null,
    transactionAt: raw.TransactionAt ? String(raw.TransactionAt) : null,
    settledAt: raw.SettledAt ? String(raw.SettledAt) : null,
    expiredAt: raw.ExpiredAt ? String(raw.ExpiredAt) : null,
    createdAt: String(raw.CreatedAt ?? ""),
    updatedAt: String(raw.UpdatedAt ?? ""),
  };
}

/**
 * Normalizes raw backend donation creation response into DonationCreationResult DTO.
 */
export function normalizeDonationCreation(raw: RawBackendDonationCreation): DonationCreationResult {
  if (!raw || typeof raw !== "object") {
    throw new Error("Invalid donation creation payload: expected an object");
  }

  const validAmount = validateMonetaryAmount(raw.amount, "amount");

  return {
    donationId: String(raw.donation_id ?? ""),
    paymentId: String(raw.payment_id ?? ""),
    orderId: String(raw.order_id ?? ""),
    amount: validAmount,
    status: (raw.status ?? "PENDING") as DonationStatus,
    paymentToken: raw.payment_token ? String(raw.payment_token) : null,
    redirectUrl: raw.redirect_url ? String(raw.redirect_url) : null,
  };
}

/**
 * Fetches the authenticated user's donation history.
 *
 * Backend endpoint: GET /api/v1/me/donations
 * Scoped authoritatively to the session user in Go.
 * Strictly enforces `cache: "no-store"`.
 */
export async function getUserDonations(
  options?: GetUserDonationsOptions
): Promise<DonationHistoryResult> {
  const client = options?.client || serverApiClient;

  let limit = 10;
  if (options?.limit !== undefined) {
    if (typeof options.limit === "number" && Number.isInteger(options.limit) && options.limit > 0) {
      limit = Math.min(options.limit, 100);
    }
  }

  let offset = 0;
  if (options?.offset !== undefined) {
    if (typeof options.offset === "number" && Number.isInteger(options.offset) && options.offset >= 0) {
      offset = options.offset;
    }
  }

  const response = await client.get<RawBackendDonation[]>("/me/donations", {
    ...options,
    cache: "no-store",
    params: {
      limit: String(limit),
      offset: String(offset),
    },
  });

  const rawList = Array.isArray(response.data) ? response.data : [];
  const items = rawList.map(normalizeDonation);

  return {
    items,
    limit,
    offset,
    hasMore: items.length === limit,
  };
}

/**
 * Fetches a single donation by ID.
 *
 * Backend endpoint: GET /api/v1/donations/{donation_id}
 * Authorized authoritatively in Go (donor, campaign owner, or ADMIN).
 * Strictly enforces `cache: "no-store"`.
 */
export async function getDonationDetail(
  donationId: string,
  options?: ServerApiRequestOptions & { client?: ServerApiClient }
): Promise<DonationDetail> {
  if (!donationId || typeof donationId !== "string") {
    throw new Error("Invalid donationId: expected a non-empty string");
  }

  const client = options?.client || serverApiClient;

  const response = await client.get<RawBackendDonation>(
    `/donations/${encodeURIComponent(donationId)}`,
    {
      ...options,
      cache: "no-store",
    }
  );

  return normalizeDonation(response.data);
}

/**
 * Fetches a single payment transaction by ID.
 *
 * Backend endpoint: GET /api/v1/payments/{payment_id}
 * Authorized authoritatively in Go under the same policy as donation detail.
 * Strictly enforces `cache: "no-store"`.
 */
export async function getPaymentDetail(
  paymentId: string,
  options?: ServerApiRequestOptions & { client?: ServerApiClient }
): Promise<PaymentDetail> {
  if (!paymentId || typeof paymentId !== "string") {
    throw new Error("Invalid paymentId: expected a non-empty string");
  }

  const client = options?.client || serverApiClient;

  const response = await client.get<RawBackendPayment>(
    `/payments/${encodeURIComponent(paymentId)}`,
    {
      ...options,
      cache: "no-store",
    }
  );

  return normalizePayment(response.data);
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Bounded server-side payment polling helper.
 *
 * FRONTEND SAFETY MECHANISM ONLY:
 * - Minimum interval: 3,000ms (configurable in options for tests)
 * - Maximum attempts: 10 attempts (30 seconds maximum total duration)
 * - Polling is allowed ONLY while status is PENDING or AUTHORIZED.
 * - Stops immediately on terminal statuses (CAPTURED, SETTLED, FAILED, EXPIRED, CANCELLED, REFUNDED, PARTIALLY_REFUNDED).
 * - Stops immediately on auth/resource errors (401, 403, 404).
 * - Enforces `cache: "no-store"`.
 * - TIMEOUT means safety budget exhausted; NEVER infer payment failure or expiry from timeout.
 */
export async function pollPaymentStatus(
  paymentId: string,
  options?: PaymentPollingOptions
): Promise<PaymentPollingResult> {
  if (!paymentId || typeof paymentId !== "string") {
    throw new Error("Invalid paymentId: expected a non-empty string");
  }

  const intervalMs = options?.intervalMs ?? POLL_INTERVAL_MS;
  const maxAttempts = options?.maxAttempts ?? MAX_POLL_ATTEMPTS;
  const maxDurationMs = options?.maxDurationMs ?? MAX_POLLING_DURATION_MS;
  const startTime = Date.now();
  let lastPayment: PaymentDetail | undefined;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    // Enforce strict wall-clock safety budget
    if (Date.now() - startTime >= maxDurationMs) {
      return {
        status: "TIMEOUT",
        payment: lastPayment,
        attempts: attempt > 1 ? attempt - 1 : 1,
      };
    }

    try {
      // retries: 0 guarantees that each polling attempt is strictly 1 upstream HTTP request
      const payment = await getPaymentDetail(paymentId, {
        ...options,
        cache: "no-store",
        retries: options?.retries ?? 0,
      });
      lastPayment = payment;

      // 1. Checkout Success: CAPTURED or SETTLED -> stop polling
      if (CHECKOUT_SUCCESS_STATUSES.includes(payment.status)) {
        return {
          status: "SUCCESS",
          payment,
          attempts: attempt,
        };
      }

      // 2. Checkout Failure: FAILED, EXPIRED, CANCELLED -> stop polling
      if (CHECKOUT_FAILURE_STATUSES.includes(payment.status)) {
        return {
          status: "FAILURE",
          payment,
          attempts: attempt,
        };
      }

      // 3. Refund: REFUNDED or PARTIALLY_REFUNDED -> stop polling
      if (CHECKOUT_REFUND_STATUSES.includes(payment.status)) {
        return {
          status: "REFUNDED",
          payment,
          attempts: attempt,
        };
      }

      // 4. Non-terminal: PENDING or AUTHORIZED -> continue polling if attempts and time remain
      if (CHECKOUT_CONTINUE_STATUSES.includes(payment.status)) {
        if (attempt >= maxAttempts) {
          return {
            status: "TIMEOUT",
            payment,
            attempts: attempt,
          };
        }
        const elapsed = Date.now() - startTime;
        const remainingTime = maxDurationMs - elapsed;
        if (remainingTime <= 0) {
          return {
            status: "TIMEOUT",
            payment,
            attempts: attempt,
          };
        }
        const sleepMs = Math.min(intervalMs, remainingTime);
        if (sleepMs > 0) {
          await sleep(sleepMs);
        }
        continue;
      }

      // Unknown status fallback -> stop polling
      return {
        status: "FAILURE",
        payment,
        attempts: attempt,
      };
    } catch (err: unknown) {
      if (err instanceof CareFundApiError) {
        // Immediate termination on non-retryable authorization/resource errors
        if (err.status === 401) {
          return {
            status: "AUTH_ERROR",
            attempts: attempt,
            error: { code: err.code, message: err.message, status: 401 },
          };
        }
        if (err.status === 403) {
          return {
            status: "ACCESS_DENIED",
            attempts: attempt,
            error: { code: err.code, message: err.message, status: 403 },
          };
        }
        if (err.status === 404) {
          return {
            status: "NOT_FOUND",
            attempts: attempt,
            error: { code: err.code, message: err.message, status: 404 },
          };
        }

        // For transient errors (502, 503, 504), wait and continue if attempts remain
        if (attempt >= maxAttempts) {
          return {
            status: "TIMEOUT",
            payment: lastPayment,
            attempts: attempt,
            error: { code: err.code, message: err.message, status: err.status },
          };
        }
      } else {
        if (attempt >= maxAttempts) {
          return {
            status: "TIMEOUT",
            payment: lastPayment,
            attempts: attempt,
          };
        }
      }

      const elapsed = Date.now() - startTime;
      const remainingTime = maxDurationMs - elapsed;
      if (remainingTime <= 0) {
        return {
          status: "TIMEOUT",
          payment: lastPayment,
          attempts: attempt,
        };
      }
      const sleepMs = Math.min(intervalMs, remainingTime);
      if (sleepMs > 0) {
        await sleep(sleepMs);
      }
    }
  }

  return {
    status: "TIMEOUT",
    payment: lastPayment,
    attempts: maxAttempts,
  };
}
