/**
 * CareFund Server-Only API Client
 *
 * HARD SECURITY BOUNDARY:
 * This module is strictly restricted to Server Components, Route Handlers, and Server Actions.
 * It provides server-to-server HTTP communication against the internal Go API with
 * Bearer token forwarding and automatic session cookie resolution.
 *
 * COMPILE-TIME GUARD:
 * Imports from "next/headers" to prevent client bundling. Any Client Component importing
 * this file will trigger a Next.js compilation error.
 *
 * RUNTIME GUARD:
 * Enforces `typeof window === "undefined"` execution assertion.
 */

import { cookies } from "next/headers";
import { serverEnv } from "@/lib/server/env";
import { CF_ACCESS_TOKEN_COOKIE } from "@/lib/auth/constants";
import { ApiClient } from "./client";
import { ApiResponse, ServerApiRequestOptions } from "./types";

if (typeof window !== "undefined") {
  throw new Error(
    "Security Violation: serverApiClient is server-only and cannot be executed in the browser."
  );
}

export class ServerApiClient {
  private readonly client: ApiClient;

  constructor(baseUrl?: string) {
    this.client = new ApiClient({
      baseUrl: baseUrl || serverEnv.goApiInternalUrl,
    });
  }

  /**
   * Executes a server-to-server authenticated API request against the Go API.
   *
   * Token Resolution Priority:
   * 1. Explicit `options.serverToken`
   * 2. Automatic resolution from `cf_access_token` cookie via `next/headers` (unless `autoCookieToken: false`)
   */
  async request<TData, TMeta = unknown>(
    path: string,
    options: ServerApiRequestOptions = {}
  ): Promise<ApiResponse<TData, TMeta>> {
    let effectiveToken = options.serverToken;

    // Automatically resolve token from server cookie context if not explicitly provided
    if (!effectiveToken && options.autoCookieToken !== false) {
      try {
        const cookieStore = await cookies();
        effectiveToken = cookieStore.get(CF_ACCESS_TOKEN_COOKIE)?.value;
      } catch {
        // cookies() throws if called outside a request context (e.g. background worker or test)
      }
    }

    // Merge Authorization header safely into request options
    const headers: Record<string, string> = { ...(options.headers || {}) };
    if (effectiveToken) {
      headers["Authorization"] = `Bearer ${effectiveToken}`;
    }

    // Forward to underlying client (which handles requestId, timeouts, retries, and errors)
    return this.client.request<TData, TMeta>(path, {
      ...options,
      headers,
    });
  }

  // Convenience Server Methods
  get<TData, TMeta = unknown>(path: string, options?: ServerApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "GET" });
  }

  head(path: string, options?: ServerApiRequestOptions) {
    return this.request<void>(path, { ...options, method: "HEAD" });
  }

  post<TData, TMeta = unknown>(
    path: string,
    body?: unknown,
    options?: ServerApiRequestOptions
  ) {
    return this.request<TData, TMeta>(path, { ...options, method: "POST", body });
  }

  put<TData, TMeta = unknown>(
    path: string,
    body?: unknown,
    options?: ServerApiRequestOptions
  ) {
    return this.request<TData, TMeta>(path, { ...options, method: "PUT", body });
  }

  patch<TData, TMeta = unknown>(
    path: string,
    body?: unknown,
    options?: ServerApiRequestOptions
  ) {
    return this.request<TData, TMeta>(path, { ...options, method: "PATCH", body });
  }

  delete<TData, TMeta = unknown>(path: string, options?: ServerApiRequestOptions) {
    return this.request<TData, TMeta>(path, { ...options, method: "DELETE" });
  }
}

/**
 * Singleton server-only API client instance.
 *
 * RESTRICTED TO SERVER RUNTIMES.
 */
export const serverApiClient = new ServerApiClient();
