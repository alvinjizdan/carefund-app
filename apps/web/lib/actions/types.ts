/**
 * CareFund Server Action Contracts & Error Serialization
 *
 * Provides a standardized, type-safe, and serializable result contract
 * for all Next.js Server Actions across the application.
 *
 * CRITICAL SECURITY INVARIANTS:
 * 1. Action results must be plain serializable JavaScript objects (React 19 / Next.js 15).
 * 2. Errors must be mapped to sanitized ActionError objects.
 * 3. Prohibits leaking:
 *    - Stack traces
 *    - SQL or database exception messages
 *    - Internal hostnames, ports, and cluster connection strings
 *    - Authorization headers, access tokens, and refresh tokens
 *    - Raw response headers or complex exception objects
 */

import { CareFundApiError, defaultErrorCodeForStatus } from "../api/errors";

export interface ActionError {
  code: string;
  message: string;
  status: number;
  requestId?: string;
}

export type ActionResult<TData> =
  | {
      ok: true;
      data: TData;
    }
  | {
      ok: false;
      error: ActionError;
    };

/**
 * Patterns that indicate internal infrastructure, SQL, or credential leaks.
 */
const SENSITIVE_PATTERNS = [
  /pq:/i,
  /syntax error/i,
  /foreign key/i,
  /relation ".*" does not exist/i,
  /column ".*" does not exist/i,
  /violates unique constraint/i,
  /connection refused/i,
  /dial tcp/i,
  /127\.0\.0\.1/i,
  /localhost/i,
  /go-api/i,
  /:8080/i,
  /:5432/i,
  /bearer /i,
  /token/i,
  /password/i,
  /secret/i,
];

/**
 * Sanitizes an error message before returning it to the client.
 */
export function sanitizeActionMessage(status: number, rawMessage?: string): string {
  if (!rawMessage || typeof rawMessage !== "string" || !rawMessage.trim()) {
    if (status >= 500) {
      return "An internal server error occurred. Please try again later.";
    }
    return `Request failed with status ${status}`;
  }

  const trimmed = rawMessage.trim();

  // If status is a server-side failure or contains sensitive patterns, hide details
  if (status >= 500) {
    return "An internal server error occurred. Please try again later.";
  }

  for (const pattern of SENSITIVE_PATTERNS) {
    if (pattern.test(trimmed)) {
      return `Request failed with status ${status}`;
    }
  }

  return trimmed;
}

/**
 * Maps any caught error across a Server Action boundary into a safe,
 * serializable ActionError object.
 *
 * @param err The caught error (CareFundApiError, Error, or unknown)
 * @param fallbackRequestId Optional fallback correlation request ID
 * @returns Sanitized ActionError safe for Client Component consumption
 */
export function toActionError(err: unknown, fallbackRequestId?: string): ActionError {
  if (err instanceof CareFundApiError) {
    return {
      code: err.code || defaultErrorCodeForStatus(err.status),
      message: sanitizeActionMessage(err.status, err.message),
      status: err.status,
      requestId: err.requestId || fallbackRequestId,
    };
  }

  if (err instanceof Error) {
    return {
      code: "INTERNAL_ERROR",
      message: "An unexpected error occurred. Please try again later.",
      status: 500,
      requestId: fallbackRequestId,
    };
  }

  return {
    code: "UNKNOWN_ERROR",
    message: "An unknown error occurred. Please try again later.",
    status: 500,
    requestId: fallbackRequestId,
  };
}
