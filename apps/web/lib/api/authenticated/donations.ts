/**
 * CareFund Authenticated Donations & Payments API Client
 *
 * Provides typed, server-only data access for authenticated user donation history,
 * donation details, and payment transaction details.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Hard Server Boundary: Strictly restricted to Server Components, Route Handlers, and Server Actions.
 * 2. Authenticated Transport: Uses serverApiClient with automatic session cookie resolution.
 * 3. Cache Isolation: Enforces `cache: "no-store"` on all authenticated personal reads.
 *    (User-specific records MUST NEVER be stored in the shared Next.js Data Cache).
 * 4. Authoritative Authorization: Go API validates user ownership, campaign ownership, or admin role.
 *    Next.js does not perform authorization overrides.
 * 5. Normalization: Explicit field-by-field mapping from Go backend PascalCase to frontend camelCase DTOs.
 * 6. Pagination Semantics: Backend returns `{ data: [...] }` with no total_count or has_more.
 *    `hasMore = items.length === limit` is strictly an unauthoritative client-side heuristic.
 */

import "next/dist/compiled/server-only";
import { serverApiClient, ServerApiClient } from "../server";
import { ServerApiRequestOptions } from "../types";
import { validateMonetaryAmount } from "../public/campaigns";

if (typeof window !== "undefined") {
  throw new Error(
    "Security Violation: authenticated donations API is server-only and cannot be executed in the browser."
  );
}

export type DonationStatus =
  | "PENDING"
  | "PAID"
  | "FAILED"
  | "EXPIRED"
  | "REFUNDED"
  | "PARTIALLY_REFUNDED"
  | "CANCELLED";

export type PaymentStatus =
  | "PENDING"
  | "AUTHORIZED"
  | "CAPTURED"
  | "SETTLED"
  | "FAILED"
  | "EXPIRED"
  | "CANCELLED"
  | "REFUNDED"
  | "PARTIALLY_REFUNDED";

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
 * Canonical frontend Donation presentation DTO.
 */
export interface Donation {
  id: string;
  campaignId: string;
  donorId: string | null;
  amount: number;
  isAnonymous: boolean;
  message: string;
  status: DonationStatus;
  createdAt: string;
  updatedAt: string;
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
 * Canonical frontend Payment presentation DTO.
 */
export interface Payment {
  id: string;
  donationId: string;
  provider: string;
  orderId: string;
  transactionId: string | null;
  paymentType: string | null;
  grossAmount: number;
  status: PaymentStatus;
  fraudStatus: string | null;
  transactionAt: string | null;
  settledAt: string | null;
  expiredAt: string | null;
  createdAt: string;
  updatedAt: string;
}

/**
 * Result envelope for paginated donation history queries.
 */
export interface DonationListResult {
  items: Donation[];
  limit: number;
  offset: number;
  /**
   * UNAUTHORITATIVE CLIENT-SIDE HEURISTIC ONLY.
   * Derived as `items.length === limit`. The Go backend does NOT provide `has_more` or `total_count`.
   */
  hasMore: boolean;
}

export interface GetUserDonationsOptions extends ServerApiRequestOptions {
  limit?: number;
  offset?: number;
  client?: ServerApiClient;
}

/**
 * Explicit normalization boundary from raw backend donation to frontend Donation DTO.
 */
export function normalizeDonation(raw: RawBackendDonation): Donation {
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
 * Explicit normalization boundary from raw backend payment to frontend Payment DTO.
 */
export function normalizePayment(raw: RawBackendPayment): Payment {
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
 * Fetches the authenticated user's donation history.
 *
 * Backend endpoint: GET /api/v1/me/donations
 *
 * Scoped authoritatively to the user identified by the session token in Go.
 * Strictly enforces `cache: "no-store"`.
 */
export async function getUserDonations(
  options?: GetUserDonationsOptions
): Promise<DonationListResult> {
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
 *
 * Authorized authoritatively in Go for:
 * - ADMIN role
 * - Donation owner (donor_id == authUser.id)
 * - Campaign owner (campaign.owner_id == authUser.id)
 * Any other actor receives 403 Forbidden.
 */
export async function getDonationDetail(
  donationId: string,
  options?: ServerApiRequestOptions & { client?: ServerApiClient }
): Promise<Donation> {
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
 *
 * Authorized authoritatively in Go under the same policy as donation detail.
 */
export async function getPaymentDetail(
  paymentId: string,
  options?: ServerApiRequestOptions & { client?: ServerApiClient }
): Promise<Payment> {
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
