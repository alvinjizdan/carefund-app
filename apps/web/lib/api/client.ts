/**
 * CareFund Typed API Client Core (Client-Safe)
 *
 * Provides a zero-dependency, Web Standards-compliant HTTP client abstraction
 * with timeout enforcement, caller cancellation protection, bounded retry
 * semantics for idempotent reads, request ID propagation, and structured error normalization.
 *
 * CRITICAL SECURITY INVARIANT:
 * This module is safe for both Client and Server components. It strictly does NOT
 * handle, store, or forward authorization tokens or private internal cluster URLs.
 * Server-authenticated operations must use the dedicated `@/lib/api/server` module.
 */

import {
  ApiRequestOptions,
  ApiResponse,
  HttpMethod,
  QueryParams,
} from "./types";
import {
  CareFundApiError,
  defaultErrorCodeForStatus,
  parseRetryAfter,
  sanitizeErrorMessage,
} from "./errors";

const REQUEST_ID_REGEX = /^[a-zA-Z0-9_-]{1,64}$/;

/**
 * Validates and sanitizes a Request ID according to the application contract.
 * Rejects IDs exceeding 64 characters or containing invalid characters.
 */
export function sanitizeRequestId(requestId?: string | null): string {
  if (requestId && REQUEST_ID_REGEX.test(requestId)) {
    return requestId;
  }
  return crypto.randomUUID();
}

/**
 * Serializes query parameters into a standard URLSearchParams instance.
 * Omits null and undefined values; handles array parameters.
 */
export function buildSearchParams(params?: QueryParams): URLSearchParams {
  const searchParams = new URLSearchParams();
  if (!params) {
    return searchParams;
  }

  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined) {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item !== null && item !== undefined) {
          searchParams.append(key, String(item));
        }
      }
    } else {
      searchParams.set(key, String(value));
    }
  }

  return searchParams;
}

/**
 * Calculates exponential backoff delay with small jitter for transient retries.
 * Attempt 1: ~200ms
 * Attempt 2: ~500ms
 */
function getRetryDelay(attempt: number): number {
  const baseMs = attempt === 1 ? 200 : 500;
  const jitterMs = Math.floor(Math.random() * 40) - 20; // +/- 20ms
  return Math.max(50, baseMs + jitterMs);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export interface ApiClientConfig {
  baseUrl?: string;
  defaultTimeoutMs?: number;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly defaultTimeoutMs: number;

  constructor(config: ApiClientConfig = {}) {
    this.baseUrl = config.baseUrl || "";
    this.defaultTimeoutMs = config.defaultTimeoutMs || 15000;
  }

  /**
   * Executes an HTTP request with timeout, parsing, and bounded retry semantics.
   */
  async request<TData, TMeta = unknown>(
    path: string,
    options: ApiRequestOptions = {}
  ): Promise<ApiResponse<TData, TMeta>> {
    const method: HttpMethod = options.method || "GET";
    const isIdempotentRead = method === "GET" || method === "HEAD";

    // Retry rule: Mutations (POST, PUT, PATCH, DELETE) have ZERO automatic retries.
    // Idempotent reads (GET, HEAD) are allowed up to 2 retries (3 total attempts).
    const maxRetries = isIdempotentRead ? Math.min(options.retries ?? 2, 2) : 0;
    const timeoutMs = options.timeoutMs || this.defaultTimeoutMs;
    const requestId = sanitizeRequestId(options.requestId);

    let attempt = 0;

    while (true) {
      attempt++;
      try {
        return await this.executeSingleAttempt<TData, TMeta>(
          path,
          method,
          options,
          requestId,
          timeoutMs
        );
      } catch (err: unknown) {
        // CASE B: If the request was explicitly aborted by the caller's AbortSignal,
        // it MUST NOT trigger automatic retry under any circumstances!
        if (
          options.signal?.aborted ||
          (err instanceof CareFundApiError && err.code === "ABORTED")
        ) {
          throw err;
        }

        const isLastAttempt = attempt > maxRetries;

        // CASE A: Eligible for retry only if idempotent read AND transient failure
        // (502, 503, 504, internal timeout, or network connection drop)
        const isTransientError =
          err instanceof CareFundApiError
            ? err.isTransient
            : err instanceof TypeError;

        if (!isIdempotentRead || isLastAttempt || !isTransientError) {
          throw err;
        }

        // Wait with backoff before next attempt
        const delay = getRetryDelay(attempt);
        await sleep(delay);
      }
    }
  }

  private async executeSingleAttempt<TData, TMeta>(
    path: string,
    method: HttpMethod,
    options: ApiRequestOptions,
    requestId: string,
    timeoutMs: number
  ): Promise<ApiResponse<TData, TMeta>> {
    // 1. Resolve Target URL
    const searchParams = buildSearchParams(options.params);
    const queryString = searchParams.toString();
    const fullPath = queryString
      ? `${path}${path.includes("?") ? "&" : "?"}${queryString}`
      : path;

    const targetUrl = this.baseUrl
      ? `${this.baseUrl.replace(/\/+$/, "")}/${fullPath.replace(/^\/+/, "")}`
      : fullPath;

    // 2. Prepare Headers
    const headers = new Headers(options.headers || {});
    headers.set("Accept", "application/json");
    headers.set("X-Request-ID", requestId);

    // 3. Serialize Body
    let body: BodyInit | undefined;
    if (options.body !== undefined && options.body !== null) {
      if (typeof options.body === "string" || options.body instanceof FormData) {
        body = options.body;
      } else {
        body = JSON.stringify(options.body);
        if (!headers.has("Content-Type")) {
          headers.set("Content-Type", "application/json");
        }
      }
    }

    // 4. Setup AbortSignal and Timeout
    const controller = new AbortController();
    let timedOut = false;

    const timerId = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, timeoutMs);

    // Chain caller's signal if provided
    if (options.signal) {
      if (options.signal.aborted) {
        controller.abort();
      } else {
        options.signal.addEventListener("abort", () => controller.abort(), {
          once: true,
        });
      }
    }

    // 5. Execute Fetch
    let res: Response;
    try {
      res = await fetch(targetUrl, {
        method,
        headers,
        body,
        cache: options.cache,
        next: options.next,
        signal: controller.signal,
      });
    } catch (fetchErr: unknown) {
      // CASE A: Internal timeout
      if (timedOut) {
        throw new CareFundApiError({
          status: 504,
          code: "TIMEOUT",
          message: `Request timed out after ${timeoutMs}ms`,
          requestId,
        });
      }

      // CASE B: Caller-provided signal explicitly cancelled
      if (options.signal?.aborted) {
        const abortReason = options.signal.reason;
        const msg =
          abortReason instanceof Error
            ? abortReason.message
            : typeof abortReason === "string"
            ? abortReason
            : "Request was cancelled by caller";

        throw new CareFundApiError({
          status: 499, // Client Closed Request
          code: "ABORTED",
          message: msg,
          requestId,
        });
      }

      // Network connection failure
      throw new CareFundApiError({
        status: 503,
        code: "SERVICE_UNAVAILABLE",
        message: (fetchErr as Error)?.message || "Network connection failed",
        requestId,
      });
    } finally {
      clearTimeout(timerId);
    }

    // 6. Handle Empty Response (204 No Content)
    if (res.status === 204) {
      return {
        data: undefined as unknown as TData,
        requestId,
      };
    }

    // 7. Parse Content Safely
    const contentType = res.headers.get("content-type") || "";
    let rawJson: unknown;

    if (contentType.includes("application/json")) {
      const text = await res.text();
      if (!text.trim()) {
        return {
          data: undefined as unknown as TData,
          requestId,
        };
      }
      try {
        rawJson = JSON.parse(text);
      } catch {
        if (!res.ok) {
          throw new CareFundApiError({
            status: res.status,
            code: defaultErrorCodeForStatus(res.status),
            message: sanitizeErrorMessage(res.status),
            requestId,
          });
        }
        throw new CareFundApiError({
          status: 500,
          code: "MALFORMED_RESPONSE",
          message: "Failed to parse backend JSON response",
          requestId,
        });
      }
    } else {
      // Non-JSON response
      const rawText = await res.text().catch(() => "");
      if (!res.ok) {
        throw new CareFundApiError({
          status: res.status,
          code: defaultErrorCodeForStatus(res.status),
          message: sanitizeErrorMessage(res.status, rawText.slice(0, 200)),
          requestId,
        });
      }
      return {
        data: rawText as unknown as TData,
        requestId,
      };
    }

    // 8. Handle HTTP Error Responses (!res.ok)
    if (!res.ok) {
      let errorCode = defaultErrorCodeForStatus(res.status);
      let errorMessage = sanitizeErrorMessage(res.status);

      if (typeof rawJson === "object" && rawJson !== null) {
        const errorEnvelope = rawJson as {
          error?: { code?: string; message?: string };
        };
        if (errorEnvelope.error?.code) {
          errorCode = errorEnvelope.error.code;
        }
        if (errorEnvelope.error?.message) {
          errorMessage = errorEnvelope.error.message;
        }
      }

      const retryAfter = parseRetryAfter(res.headers.get("retry-after"));

      throw new CareFundApiError({
        status: res.status,
        code: errorCode,
        message: errorMessage,
        requestId,
        retryAfter,
      });
    }

    // 9. Unwrap Backend Success Envelope: { "data": TData, "meta"?: TMeta }
    if (typeof rawJson === "object" && rawJson !== null && "data" in rawJson) {
      const successEnvelope = rawJson as { data: TData; meta?: TMeta };
      return {
        data: successEnvelope.data,
        meta: successEnvelope.meta, // Preserves undefined if backend omitted meta
        requestId,
      };
    }

    // Return raw JSON payload if backend omitted the data envelope
    return {
      data: rawJson as TData,
      requestId,
    };
  }

  // Convenience Methods
  get<TData, TMeta = unknown>(path: string, options?: ApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "GET" });
  }

  head(path: string, options?: ApiRequestOptions) {
    return this.request<void>(path, { ...options, method: "HEAD" });
  }

  post<TData, TMeta = unknown>(path: string, body?: unknown, options?: ApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "POST", body });
  }

  put<TData, TMeta = unknown>(path: string, body?: unknown, options?: ApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "PUT", body });
  }

  patch<TData, TMeta = unknown>(path: string, body?: unknown, options?: ApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "PATCH", body });
  }

  delete<TData, TMeta = unknown>(path: string, options?: ApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "DELETE" });
  }
}

/**
 * Factory to create an ApiClient with a specified base URL.
 */
export function createApiClient(baseUrl?: string): ApiClient {
  return new ApiClient({ baseUrl });
}

/**
 * Public API client instance.
 * Safe for general use; defaults to application relative path or configured public base URL.
 */
export const apiClient = createApiClient();
