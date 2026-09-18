/**
 * CareFund Authenticated Domain Types
 *
 * Provides strongly-typed interfaces and status unions for authenticated user
 * data access across donations, payments, and donor history.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Strict Status Unions: Supported exactly by the Go domain model.
 * 2. Safe Money Representation: Monetary amounts represent int64 IDR currency values
 *    validated as safe integers. No floating-point arithmetic.
 * 3. Client Heuristic Attribution: Pagination `hasMore` is strictly an unauthoritative
 *    client heuristic (Go backend returns raw arrays without total/has_more metadata).
 */

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

export interface DonationHistoryItem {
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

export type DonationDetail = DonationHistoryItem;

export interface PaymentDetail {
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

export interface DonationCreationResult {
  donationId: string;
  paymentId: string;
  orderId: string;
  amount: number;
  status: DonationStatus;
  paymentToken: string | null;
  redirectUrl: string | null;
}

export interface DonationHistoryResult {
  items: DonationHistoryItem[];
  limit: number;
  offset: number;
  /**
   * UNAUTHORITATIVE CLIENT HEURISTIC ONLY.
   * Derived as `items.length === limit`. The Go backend does NOT provide `has_more` or `total_count`.
   */
  hasMore: boolean;
}

export type PaymentPollingStatus =
  | "SUCCESS"
  | "FAILURE"
  | "REFUNDED"
  | "TIMEOUT"
  | "AUTH_ERROR"
  | "NOT_FOUND"
  | "ACCESS_DENIED";

export interface PaymentPollingResult {
  status: PaymentPollingStatus;
  payment?: PaymentDetail;
  attempts: number;
  error?: {
    code: string;
    message: string;
    status: number;
  };
}

// Backward-compatibility aliases
export type Donation = DonationDetail;
export type Payment = PaymentDetail;
export type DonationListResult = DonationHistoryResult;
