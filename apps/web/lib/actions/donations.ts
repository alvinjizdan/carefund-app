"use server";

/**
 * CareFund Donation Creation Server Action
 *
 * Implements server-side action for initiating campaign donations via the Go API.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Authority Delegation: Go API is the SOLE authority for user authentication and authorization.
 *    Identity is derived from the server-managed session cookie; no user/role input is accepted.
 * 2. Idempotency Invariant: A UUIDv4 Idempotency-Key is generated once per logical donation attempt.
 *    The exact same key is reused for internal retries (202 in-flight, transient 504 timeouts).
 *    A new key is NEVER generated per retry.
 * 3. 202 In-Flight Protocol: If backend returns 202 request_in_flight, respects Retry-After (1s)
 *    and retries with the same key and payload up to 3 total attempts (1 initial + 2 retries).
 * 4. Error Mapping & Non-Retry:
 *    - 400 INVALID_REQUEST (hash mismatch): Non-retryable client error.
 *    - 422 payment_failed: Definitive provider failure; no automatic mutation retry.
 *    - 429 TOO_MANY_REQUESTS: No automatic mutation retry; applies client-side 60s fallback penalty.
 *    - 504 TIMEOUT: Ambiguous processing state; does not assume payment failure.
 * 5. Token & Redirect Security:
 *    - payment_token: In-memory only; never persisted or logged.
 *    - redirect_url: Validated against exact Midtrans hostname whitelist.
 * 6. Status Invariant: Frontend NEVER derives or mutates donation.status from payment.status.
 * 7. Error Serialization: Returns sanitized, serializable ActionResult<T> discriminated union.
 */

import { headers } from "next/headers";
import { serverApiClient, ServerApiClient } from "../api/server";
import { getOrGenerateRequestId } from "../server/correlation";
import { ActionResult, toActionError } from "./types";
import { validateMonetaryAmount } from "../api/public/campaigns";
import { CareFundApiError } from "../api/errors";
import {
  DonationCreationResult,
  normalizeDonationCreation,
  RawBackendDonationCreation,
  isValidMidtransRedirectUrl,
} from "../api/authenticated/donations";

export interface CreateDonationInput {
  campaignId: string;
  amount: number;
  isAnonymous?: boolean;
  message?: string;
}

export interface CreateDonationActionOptions {
  client?: ServerApiClient;
  requestId?: string;
  idempotencyKey?: string;
  retryDelayMs?: number;
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

async function resolveActionRequestId(explicitId?: string): Promise<string> {
  if (explicitId) {
    return getOrGenerateRequestId(explicitId);
  }
  try {
    const headerList = await headers();
    return getOrGenerateRequestId(headerList);
  } catch {
    return getOrGenerateRequestId();
  }
}

/**
 * Server Action to initiate a donation for a campaign.
 *
 * Enforces strict input validation, single-logical-attempt idempotency key generation,
 * 202 in-flight bounded retry (max 3 total attempts), and redirect URL hostname verification.
 */
export async function createDonationAction(
  input: CreateDonationInput,
  options?: CreateDonationActionOptions
): Promise<ActionResult<DonationCreationResult>> {
  const requestId = await resolveActionRequestId(options?.requestId);
  const client = options?.client || serverApiClient;

  // 1. Strict Input Validation (Client MUST NOT pass auth tokens, user IDs, roles, or statuses)
  if (!input || typeof input !== "object") {
    return {
      ok: false,
      error: {
        code: "INVALID_REQUEST",
        message: "Invalid donation input: expected an object",
        status: 400,
        requestId,
      },
    };
  }

  const campaignId = typeof input.campaignId === "string" ? input.campaignId.trim() : "";
  if (!campaignId) {
    return {
      ok: false,
      error: {
        code: "INVALID_REQUEST",
        message: "campaignId is required and must be a non-empty string",
        status: 400,
        requestId,
      },
    };
  }

  let amount: number;
  try {
    amount = validateMonetaryAmount(input.amount, "amount");
  } catch (err) {
    return {
      ok: false,
      error: {
        code: "INVALID_REQUEST",
        message: (err as Error).message,
        status: 400,
        requestId,
      },
    };
  }

  if (amount <= 0) {
    return {
      ok: false,
      error: {
        code: "INVALID_REQUEST",
        message: "Donation amount must be greater than zero",
        status: 400,
        requestId,
      },
    };
  }

  const isAnonymous = Boolean(input.isAnonymous);
  const message = typeof input.message === "string" ? input.message.trim() : "";

  // 2. Idempotency Key Management: Generated ONCE per logical donation attempt
  const idempotencyKey = options?.idempotencyKey || crypto.randomUUID();

  const payload = {
    campaign_id: campaignId,
    amount,
    is_anonymous: isAnonymous,
    message,
  };

  const MAX_ATTEMPTS = 3;
  const inFlightDelayMs = options?.retryDelayMs ?? 1000;

  for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
    try {
      const response = await client.post<RawBackendDonationCreation | { error?: { code?: string; message?: string } }>(
        "/donations",
        payload,
        {
          requestId,
          headers: {
            "Idempotency-Key": idempotencyKey,
          },
        }
      );

      // Check if response body is an in-flight 202 envelope (when returned with 2xx ok)
      if (
        response.data &&
        typeof response.data === "object" &&
        "error" in response.data &&
        (response.data as { error?: { code?: string } }).error?.code === "request_in_flight"
      ) {
        if (attempt < MAX_ATTEMPTS) {
          if (inFlightDelayMs > 0) {
            await sleep(inFlightDelayMs);
          }
          continue; // Retry attempt 2 or 3 with the exact same key and payload
        }

        return {
          ok: false,
          error: {
            code: "REQUEST_IN_FLIGHT_TIMEOUT",
            message: "Your donation is still being processed. Please check your donation history in a few moments.",
            status: 202,
            requestId,
          },
        };
      }

      // Successful 201 response (new or replayed)
      const rawCreation = response.data as RawBackendDonationCreation;
      const result = normalizeDonationCreation(rawCreation);

      // Validate redirectUrl if present
      if (result.redirectUrl && !isValidMidtransRedirectUrl(result.redirectUrl)) {
        return {
          ok: false,
          error: {
            code: "SECURITY_VIOLATION",
            message: "Payment provider redirect URL failed security validation.",
            status: 500,
            requestId,
          },
        };
      }

      return {
        ok: true,
        data: result,
      };
    } catch (err: unknown) {
      if (err instanceof CareFundApiError) {
        // Handle 202 request_in_flight if surfaced as CareFundApiError
        if (err.status === 202 || err.code === "request_in_flight") {
          if (attempt < MAX_ATTEMPTS) {
            const delay = err.retryAfter ? err.retryAfter * 1000 : inFlightDelayMs;
            if (delay > 0) {
              await sleep(delay);
            }
            continue;
          }
          return {
            ok: false,
            error: {
              code: "REQUEST_IN_FLIGHT_TIMEOUT",
              message: "Your donation is still being processed. Please check your donation history in a few moments.",
              status: 202,
              requestId,
            },
          };
        }

        // 400 INVALID_REQUEST: Payload hash mismatch or input error -> non-retryable
        if (err.status === 400) {
          return {
            ok: false,
            error: toActionError(err, requestId),
          };
        }

        // 422 payment_failed: Definitive provider failure -> non-retryable
        if (err.status === 422 || err.code === "payment_failed") {
          return {
            ok: false,
            error: toActionError(err, requestId),
          };
        }

        // 429 TOO_MANY_REQUESTS: Do NOT auto-retry; enforce 60s fallback if missing Retry-After
        if (err.status === 429) {
          const actionErr = toActionError(err, requestId);
          actionErr.retryAfter = err.retryAfter ?? 60;
          return {
            ok: false,
            error: actionErr,
          };
        }

        // 504 TIMEOUT: Ambiguous processing state
        if (err.status === 504 || err.code === "TIMEOUT") {
          // One transient retry within attempt budget
          if (attempt < MAX_ATTEMPTS) {
            if (inFlightDelayMs > 0) {
              await sleep(inFlightDelayMs);
            }
            continue;
          }
          return {
            ok: false,
            error: {
              code: "TIMEOUT",
              message: "The donation request timed out. Please check your donation history before attempting again.",
              status: 504,
              requestId,
            },
          };
        }

        // Other API errors (401, 403, 404, 500, 502, 503)
        return {
          ok: false,
          error: toActionError(err, requestId),
        };
      }

      // Non-API Error
      return {
        ok: false,
        error: toActionError(err, requestId),
      };
    }
  }

  // Exhausted retry budget
  return {
    ok: false,
    error: {
      code: "REQUEST_IN_FLIGHT_TIMEOUT",
      message: "Your donation is still being processed. Please check your donation history in a few moments.",
      status: 202,
      requestId,
    },
  };
}
