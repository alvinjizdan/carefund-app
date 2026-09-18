/**
 * CareFund API Error Model
 *
 * Implements a normalized, strongly-typed error class for API communication.
 * Preserves exact HTTP statuses, backend error codes, and correlation IDs
 * without leaking internal server details or authorization tokens.
 */

export interface CareFundApiErrorOptions {
  status: number;
  code: string;
  message: string;
  requestId: string;
  retryAfter?: number;
}

/**
 * Standard HTTP Status to Semantic Error Code mapping.
 */
export function defaultErrorCodeForStatus(status: number): string {
  switch (status) {
    case 400:
      return "INVALID_REQUEST";
    case 401:
      return "UNAUTHORIZED";
    case 403:
      return "FORBIDDEN";
    case 404:
      return "NOT_FOUND";
    case 409:
      return "DUPLICATE";
    case 422:
      return "UNPROCESSABLE_ENTITY";
    case 429:
      return "TOO_MANY_REQUESTS";
    case 502:
      return "BAD_GATEWAY";
    case 503:
      return "SERVICE_UNAVAILABLE";
    case 504:
      return "TIMEOUT";
    default:
      return status >= 500 ? "INTERNAL_ERROR" : "UNKNOWN_ERROR";
  }
}

/**
 * Parses the Retry-After header into integer seconds.
 * Handles both delta-seconds and HTTP-date formats.
 */
export function parseRetryAfter(headerValue?: string | null): number | undefined {
  if (!headerValue) {
    return undefined;
  }

  const trimmed = headerValue.trim();

  // Delta-seconds: integer string
  if (/^-?\d+$/.test(trimmed)) {
    const deltaSeconds = parseInt(trimmed, 10);
    return deltaSeconds >= 0 ? deltaSeconds : undefined;
  }

  // HTTP-date format (e.g. "Wed, 21 Oct 2026 07:28:00 GMT")
  const httpDateMs = Date.parse(trimmed);
  if (!isNaN(httpDateMs)) {
    const diffSec = Math.max(0, Math.ceil((httpDateMs - Date.now()) / 1000));
    return diffSec;
  }

  return undefined;
}

/**
 * Produces a safe, sanitized error message for a given status code.
 */
export function sanitizeErrorMessage(status: number, rawMessage?: string): string {
  if (rawMessage && typeof rawMessage === "string" && rawMessage.trim()) {
    return rawMessage.trim();
  }
  if (status >= 500) {
    return "An internal server error occurred. Please try again later.";
  }
  return `Request failed with status ${status}`;
}

/**
 * Normalized CareFund API Error.
 *
 * CRITICAL INVARIANTS:
 * 1. Preserves exact HTTP status and backend error code.
 * 2. Never converts 401 into a generic network error.
 * 3. Never masks 403 as 404.
 * 4. Never converts 422 payment failure into a generic failure.
 * 5. Contains ZERO access tokens, refresh tokens, or backend credentials.
 */
export class CareFundApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string;
  readonly isTransient: boolean;
  readonly isPaymentFailure: boolean;
  readonly retryAfter?: number;

  constructor(options: CareFundApiErrorOptions) {
    super(options.message);
    this.name = "CareFundApiError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId;
    this.retryAfter = options.retryAfter;

    // Transient failure classification:
    // 502 (Bad Gateway), 503 (Service Unavailable), 504 (Gateway Timeout),
    // or explicit TIMEOUT code.
    this.isTransient =
      [502, 503, 504].includes(options.status) || options.code === "TIMEOUT";

    // Payment failure classification:
    // 422 Unprocessable Entity or backend payment_failed code.
    this.isPaymentFailure =
      options.status === 422 || options.code === "payment_failed";

    // Maintain prototype inheritance
    Object.setPrototypeOf(this, CareFundApiError.prototype);
  }
}
