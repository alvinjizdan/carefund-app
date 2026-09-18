import { NextResponse } from "next/server";
import { BackendErrorEnvelope, BffErrorEnvelope } from "@/lib/auth/types";

import { defaultErrorCodeForStatus } from "@/lib/api/errors";
export { defaultErrorCodeForStatus };

/**
 * Creates a standardized Next.js JSON error response.
 * Strips internal details/stack traces while preserving backend status code and semantic error code.
 */
export function createErrorResponse(
  status: number,
  code: string,
  message: string,
  requestId: string
): NextResponse<BffErrorEnvelope> {
  return NextResponse.json(
    {
      error: {
        code,
        message,
      },
      request_id: requestId,
    },
    {
      status,
      headers: {
        "X-Request-ID": requestId,
        "Content-Type": "application/json",
      },
    }
  );
}

/**
 * Parses and forwards an error response from the Go API.
 * Ensures the response envelope matches standard contract and preserves the correlation ID.
 */
export async function forwardBackendError(
  res: Response,
  fallbackRequestId: string
): Promise<NextResponse<BffErrorEnvelope>> {
  const status = res.status;
  const requestId =
    res.headers.get("x-request-id") ||
    res.headers.get("X-Request-ID") ||
    fallbackRequestId;

  try {
    const errorJson = (await res.json()) as BackendErrorEnvelope;
    if (errorJson?.error?.code && errorJson?.error?.message) {
      return createErrorResponse(
        status,
        errorJson.error.code,
        errorJson.error.message,
        requestId
      );
    }
  } catch {
    // If backend did not return valid JSON or stream already consumed
  }

  const defaultCode = defaultErrorCodeForStatus(status);
  const defaultMessage =
    status >= 500
      ? "An internal error occurred. Please try again later."
      : `Request failed with status ${status}`;

  return createErrorResponse(status, defaultCode, defaultMessage, requestId);
}
