/**
 * CareFund Public Categories API Client
 *
 * Provides typed access to public campaign categories.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Public Read Boundary: Executes server-side (Server Components).
 * 2. Zero Auth Invariant: Never sends Authorization headers or bearer tokens.
 * 3. Transport: Built on the locked F3.2.1 Typed API Client Core.
 * 4. Caching: Next.js Data Cache revalidate = 3600s (1 hour), tag = "categories".
 *    (Cache tag assigned/prepared; mutation-driven invalidation deferred).
 * 5. Normalization: Maps Go backend PascalCase to canonical frontend camelCase.
 * 6. Server Boundary: Guarded against client-side browser execution.
 */

import "next/dist/compiled/server-only";
import { ApiClient } from "../client";
import { ApiRequestOptions } from "../types";
import { serverEnv } from "@/lib/server/env";

if (typeof window !== "undefined") {
  throw new Error(
    "Security Violation: categories API is server-only and cannot be executed in the browser."
  );
}

/**
 * Raw category shape as serialized by the Go backend (PascalCase, no JSON tags).
 */
export interface RawBackendCategory {
  ID: string;
  Name: string;
  Slug: string;
  IsActive: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

/**
 * Canonical frontend Category presentation DTO.
 */
export interface Category {
  id: string;
  name: string;
  slug: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * Explicit normalization boundary from raw backend payload to frontend Category DTO.
 * Guarantees field-by-field mapping without arbitrary object spreading.
 */
export function normalizeCategory(raw: RawBackendCategory): Category {
  if (!raw || typeof raw !== "object") {
    throw new Error("Invalid category payload: expected an object");
  }

  return {
    id: String(raw.ID ?? ""),
    name: String(raw.Name ?? ""),
    slug: String(raw.Slug ?? ""),
    isActive: Boolean(raw.IsActive),
    createdAt: String(raw.CreatedAt ?? ""),
    updatedAt: String(raw.UpdatedAt ?? ""),
  };
}

export interface GetCategoriesOptions extends ApiRequestOptions {
  client?: ApiClient;
}

/**
 * Fetches the list of active categories from the Go backend.
 *
 * Backend is authoritative for filtering active categories (WHERE is_active = true).
 *
 * @param options Optional ApiRequestOptions to customize timeouts, cancellation, or caching.
 * @returns Promise resolving to normalized Category array.
 */
export async function getCategories(
  options?: GetCategoriesOptions
): Promise<Category[]> {
  const client =
    options?.client ||
    new ApiClient({
      baseUrl: serverEnv.goApiInternalUrl,
    });

  const response = await client.get<RawBackendCategory[]>("/categories", {
    ...options,
    next: {
      revalidate: 3600,
      tags: ["categories"],
      ...options?.next,
    },
  });

  const rawList = Array.isArray(response.data) ? response.data : [];
  return rawList.map(normalizeCategory);
}
