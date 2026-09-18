/**
 * CareFund Public Campaigns API Client
 *
 * Provides typed access to public campaigns and viewer-aware campaign details.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Public Read Boundary: Executes server-side (Server Components).
 * 2. Zero Auth Invariant (Public): getPublicCampaigns and getPublicCampaign send zero Authorization headers.
 * 3. Viewer-Aware Boundary: getCampaignForViewer uses serverApiClient (Server-only, no-store).
 * 4. Transport: Built on the locked F3.2.1 Typed API Client Core.
 * 5. Caching:
 *    - Public List: revalidate = 60s, tag = "campaigns"
 *    - Public Detail: revalidate = 60s, tag = "campaign-[id]"
 *    - Viewer Detail: cache = "no-store" (Strictly private SSR)
 *    (Cache tags assigned/prepared; mutation-driven invalidation deferred).
 * 6. Pagination: hasMore is strictly a client-side heuristic (items.length === limit).
 * 7. Category Filtering: Deferred until backend supports server-side filtering.
 * 8. Monetary Validation: Enforces non-negative safe integers (int64 IDR values).
 */

import "next/dist/compiled/server-only";
import { ApiClient } from "../client";
import { serverApiClient, ServerApiClient } from "../server";
import {
  ApiRequestOptions,
  PaginatedResult,
  PaginationParams,
  ServerApiRequestOptions,
} from "../types";
import { serverEnv } from "@/lib/server/env";

if (typeof window !== "undefined") {
  throw new Error(
    "Security Violation: campaigns API is server-only and cannot be executed in the browser."
  );
}

/**
 * Exact campaign lifecycle states matching Go backend (domain.CampaignState*).
 * Strictly typed literal union without `| string` escape.
 */
export type CampaignStatus =
  | "ACTIVE"
  | "DRAFT"
  | "PENDING_REVIEW"
  | "REJECTED"
  | "SUSPENDED"
  | "COMPLETED"
  | "CANCELLED";

/**
 * Raw campaign shape as serialized by the Go backend (PascalCase, no JSON tags).
 */
export interface RawBackendCampaign {
  ID: string;
  OwnerID: string;
  CategoryID: string;
  Title: string;
  Slug: string;
  Description: string;
  TargetAmount: number;
  CurrentAmount: number;
  StartAt: string;
  EndAt: string;
  Status: string;
  RejectionReason: string | null;
  CreatedAt: string;
  UpdatedAt: string;
}

/**
 * Canonical frontend public Campaign presentation DTO.
 *
 * PRIVACY INVARIANT:
 * Internal identifier `ownerId` and privileged field `rejectionReason`
 * are strictly omitted from public presentation types.
 */
export interface PublicCampaign {
  id: string;
  categoryId: string;
  title: string;
  slug: string;
  description: string;
  targetAmount: number;
  currentAmount: number;
  startAt: string;
  endAt: string;
  status: CampaignStatus;
  createdAt: string;
  updatedAt: string;
}

/**
 * Canonical frontend viewer-aware CampaignDetail presentation DTO.
 *
 * Retains `ownerId` and optional `rejectionReason` for owner/admin inspection.
 */
export interface CampaignDetail extends PublicCampaign {
  ownerId: string;
  rejectionReason?: string | null;
}

/**
 * Validates that a monetary amount received from the backend is a valid,
 * non-negative safe integer representing Rupiah currency units.
 *
 * Rejects negative numbers, floating-point numbers, NaN, Infinity, and unsafe integers.
 */
export function validateMonetaryAmount(amount: unknown, fieldName: string): number {
  if (typeof amount !== "number" || !Number.isSafeInteger(amount) || amount < 0) {
    throw new Error(
      `Monetary validation failed for ${fieldName}: must be a non-negative safe integer (received: ${amount})`
    );
  }
  return amount;
}

/**
 * Normalizes raw backend campaign into public presentation DTO.
 * Guarantees field-by-field mapping without arbitrary object spreading.
 */
export function normalizeCampaign(raw: RawBackendCampaign): PublicCampaign {
  if (!raw || typeof raw !== "object") {
    throw new Error("Invalid campaign payload: expected an object");
  }

  return {
    id: String(raw.ID ?? ""),
    categoryId: String(raw.CategoryID ?? ""),
    title: String(raw.Title ?? ""),
    slug: String(raw.Slug ?? ""),
    description: String(raw.Description ?? ""),
    targetAmount: validateMonetaryAmount(raw.TargetAmount, "targetAmount"),
    currentAmount: validateMonetaryAmount(raw.CurrentAmount, "currentAmount"),
    startAt: String(raw.StartAt ?? ""),
    endAt: String(raw.EndAt ?? ""),
    status: raw.Status as CampaignStatus,
    createdAt: String(raw.CreatedAt ?? ""),
    updatedAt: String(raw.UpdatedAt ?? ""),
  };
}

/**
 * Normalizes raw backend campaign into viewer-aware detail presentation DTO.
 * Preserves ownerId and rejectionReason.
 */
export function normalizeCampaignDetail(raw: RawBackendCampaign): CampaignDetail {
  const base = normalizeCampaign(raw);
  return {
    ...base,
    ownerId: String(raw.OwnerID ?? ""),
    rejectionReason: raw.RejectionReason ?? null,
  };
}

export interface GetPublicCampaignsOptions extends ApiRequestOptions {
  client?: ApiClient;
}

/**
 * Fetches a paginated list of public active campaigns.
 *
 * Backend is authoritative for visibility (WHERE status = 'ACTIVE' ORDER BY created_at DESC).
 *
 * ARCHITECTURAL CONSTRAINTS:
 * - Category filtering is DEFERRED until backend supports server-side filtering.
 * - hasMore is a CLIENT-SIDE HEURISTIC ONLY (`items.length === limit`), NOT authoritative backend truth.
 * - Sends zero Authorization headers.
 *
 * @param params Optional pagination parameters (limit, offset).
 * @param options Optional ApiRequestOptions for customizing transport/cache.
 */
export async function getPublicCampaigns(
  params?: PaginationParams,
  options?: GetPublicCampaignsOptions
): Promise<PaginatedResult<PublicCampaign>> {
  let limit = 10;
  if (params?.limit !== undefined) {
    if (typeof params.limit === "number" && Number.isInteger(params.limit) && params.limit > 0) {
      limit = Math.min(params.limit, 100);
    }
  }

  let offset = 0;
  if (params?.offset !== undefined) {
    if (typeof params.offset === "number" && Number.isInteger(params.offset) && params.offset >= 0) {
      offset = params.offset;
    }
  }

  const client =
    options?.client ||
    new ApiClient({
      baseUrl: serverEnv.goApiInternalUrl,
    });

  const response = await client.get<RawBackendCampaign[]>("/campaigns", {
    ...options,
    params: {
      limit,
      offset,
    },
    next: {
      revalidate: 60,
      tags: ["campaigns"],
      ...options?.next,
    },
  });

  const rawItems = Array.isArray(response.data) ? response.data : [];
  const items = rawItems.map(normalizeCampaign);

  return {
    items,
    limit,
    offset,
    hasMore: items.length === limit, // CLIENT-SIDE HEURISTIC ONLY
  };
}

export interface GetPublicCampaignOptions extends ApiRequestOptions {
  client?: ApiClient;
}

/**
 * Fetches a single public campaign by ID.
 *
 * PUBLIC INVARIANTS:
 * - Sends zero Authorization headers.
 * - Next.js Data Cache: revalidate = 60s, tag = "campaign-[id]".
 * - If non-active campaign is requested by anonymous caller, backend returns 404 (NOT_FOUND).
 *
 * @param id Campaign UUID.
 * @param options Optional ApiRequestOptions for customizing transport/cache.
 */
export async function getPublicCampaign(
  id: string,
  options?: GetPublicCampaignOptions
): Promise<PublicCampaign> {
  const client =
    options?.client ||
    new ApiClient({
      baseUrl: serverEnv.goApiInternalUrl,
    });

  const response = await client.get<RawBackendCampaign>(
    `/campaigns/${encodeURIComponent(id)}`,
    {
      ...options,
      next: {
        revalidate: 60,
        tags: [`campaign-${id}`],
        ...options?.next,
      },
    }
  );

  return normalizeCampaign(response.data);
}

export interface GetCampaignForViewerOptions extends ServerApiRequestOptions {
  client?: ServerApiClient;
}

/**
 * Fetches a single campaign by ID with optional viewer authentication context.
 *
 * SERVER-ONLY INVARIANTS:
 * - Executes strictly server-side using ServerApiClient.
 * - Forwards Bearer token from session cookie if present.
 * - Enforces `cache: "no-store"` (dynamic SSR only, never publicly cached).
 * - Backend uses optional authentication:
 *     - Anonymous: ACTIVE -> success; non-ACTIVE -> 404.
 *     - Authenticated owner/admin: receives private lifecycle states and rejectionReason.
 *
 * @param id Campaign UUID.
 * @param options Optional ServerApiRequestOptions (allows explicit serverToken).
 */
export async function getCampaignForViewer(
  id: string,
  options?: GetCampaignForViewerOptions
): Promise<CampaignDetail> {
  const client = options?.client || serverApiClient;

  const response = await client.get<RawBackendCampaign>(
    `/campaigns/${encodeURIComponent(id)}`,
    {
      ...options,
      cache: "no-store",
    }
  );

  return normalizeCampaignDetail(response.data);
}
