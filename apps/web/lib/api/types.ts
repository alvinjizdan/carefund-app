/**
 * CareFund API Client Types
 *
 * Defines the core types for HTTP communication, request configurations,
 * response envelopes, and pagination models across server and client boundaries.
 */

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD";

export type QueryParamValue = string | number | boolean | null | undefined;
export type QueryParams = Record<string, QueryParamValue | QueryParamValue[]>;

export interface NextFetchConfig {
  revalidate?: number | false;
  tags?: string[];
}

/**
 * Public / Base Request Options.
 *
 * CRITICAL SECURITY INVARIANT:
 * This interface strictly does NOT accept authorization tokens.
 * Client Components must NEVER pass access or refresh tokens into API requests.
 */
export interface ApiRequestOptions<TBody = unknown> {
  method?: HttpMethod;
  headers?: Record<string, string>;
  params?: QueryParams;
  body?: TBody;
  requestId?: string;
  timeoutMs?: number;
  cache?: RequestCache;
  next?: NextFetchConfig;
  /**
   * Bounded retry count for idempotent reads (GET/HEAD). Default is 2 (3 total attempts).
   * Mutations (POST, PUT, PATCH, DELETE) ignore this setting and are NEVER automatically retried.
   */
  retries?: number;
  signal?: AbortSignal;
}

/**
 * Server-Authenticated Request Options.
 *
 * RESTRICTED TO SERVER-SIDE ENVIRONMENTS (Server Components, Route Handlers, Server Actions).
 * Allows server-to-server token forwarding.
 */
export interface ServerApiRequestOptions<TBody = unknown>
  extends ApiRequestOptions<TBody> {
  /**
   * Optional Bearer access token for server-to-server Go API requests.
   * MUST NEVER be used or exposed in Client Components.
   */
  serverToken?: string;
  /**
   * If true (default), automatically reads cf_access_token from server request cookies
   * when serverToken is not explicitly provided.
   */
  autoCookieToken?: boolean;
}

/**
 * Raw Go Backend Success Envelope:
 * { "data": TData, "meta"?: TMeta }
 */
export interface BackendSuccessEnvelope<TData, TMeta = unknown> {
  data: TData;
  meta?: TMeta;
}

/**
 * Raw Go Backend Error Detail:
 * { "code": "STRING", "message": "STRING" }
 */
export interface BackendErrorDetail {
  code: string;
  message: string;
}

/**
 * Raw Go Backend Error Envelope:
 * { "error": { "code": string, "message": string }, "request_id"?: string }
 */
export interface BackendErrorEnvelope {
  error: BackendErrorDetail;
  request_id?: string;
}

/**
 * Normalized Frontend API Response.
 */
export interface ApiResponse<TData, TMeta = unknown> {
  data: TData;
  meta?: TMeta;
  requestId: string;
}

/**
 * Standard Offset Pagination Query Parameters.
 */
export interface PaginationParams {
  limit?: number;
  offset?: number;
}

/**
 * Normalized Offset Paginated Result.
 */
export interface PaginatedResult<T> {
  items: T[];
  limit: number;
  offset: number;
  hasMore: boolean;
}
