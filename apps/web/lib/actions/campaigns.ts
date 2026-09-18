"use server";

/**
 * CareFund Campaign Mutation Server Actions
 *
 * Implements server-side actions for campaign lifecycle mutations:
 * - createCampaignAction: creates a new campaign (starts in DRAFT)
 * - updateCampaignAction: updates existing campaign details (owner only)
 * - submitReviewAction: transitions DRAFT → PENDING_REVIEW (owner only)
 * - approveCampaignAction: transitions PENDING_REVIEW / SUSPENDED → ACTIVE (admin only)
 * - rejectCampaignAction: transitions PENDING_REVIEW → REJECTED (admin only)
 * - suspendCampaignAction: transitions ACTIVE → SUSPENDED (admin only)
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Authority: Go API is the SOLE authority for ownership, RBAC, and state transitions.
 * 2. Token Security: Never accepts client-provided tokens, authorization headers, user IDs, or roles.
 * 3. CSRF Protection: Protected by Next.js Server Action framework-level Origin/Host validation.
 * 4. Error Boundary: Catches all exceptions and returns serializable ActionResult<T> discriminated unions.
 * 5. Retry Policy: Zero automatic retries on all mutations (POST/PATCH).
 * 6. Cache Invalidation: Triggers targeted revalidateTag on successful mutations; never on failure.
 */

import { revalidateTag } from "next/cache";
import { headers } from "next/headers";
import { serverApiClient, ServerApiClient } from "../api/server";
import { getOrGenerateRequestId } from "../server/correlation";
import { ActionResult, toActionError } from "./types";
import {
  CampaignDetail,
  normalizeCampaignDetail,
  RawBackendCampaign,
  validateMonetaryAmount,
} from "../api/public/campaigns";

export interface CreateCampaignInput {
  title: string;
  description: string;
  categoryId: string;
  targetAmount: number;
  startAt: string;
  endAt: string;
}

export interface UpdateCampaignInput {
  title: string;
  description: string;
  categoryId: string;
  targetAmount: number;
  startAt: string;
  endAt: string;
}

export interface ActionInvocationOptions {
  client?: ServerApiClient;
  requestId?: string;
}

/**
 * Safely resolves the incoming request ID from Next.js request headers,
 * or generates a new UUID if headers() is unavailable (e.g. test environment).
 */
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
 * Safely triggers Next.js Data Cache tag invalidation.
 * Silently handles environments where revalidateTag is called outside active request contexts.
 */
function safeRevalidateTag(tag: string): void {
  try {
    revalidateTag(tag);
  } catch {
    // revalidateTag may throw in non-request environments (e.g. unit test runner)
  }
}

/**
 * Creates a new campaign.
 *
 * Backend endpoint: POST /api/v1/campaigns
 *
 * Authorization: Authenticated user. Go automatically assigns OwnerID = authUser.ID.
 * Initial state: DRAFT, currentAmount: 0.
 * Zero automatic retries.
 */
export async function createCampaignAction(
  input: CreateCampaignInput,
  options?: ActionInvocationOptions
): Promise<ActionResult<CampaignDetail>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!input || typeof input !== "object") {
      throw new Error("Invalid input: expected an object");
    }

    const targetAmount = validateMonetaryAmount(input.targetAmount, "targetAmount");

    const client = options?.client || serverApiClient;
    const response = await client.post<RawBackendCampaign>(
      "/campaigns",
      {
        title: String(input.title ?? "").trim(),
        description: String(input.description ?? "").trim(),
        category_id: String(input.categoryId ?? "").trim(),
        target_amount: targetAmount,
        start_at: String(input.startAt ?? ""),
        end_at: String(input.endAt ?? ""),
      },
      {
        requestId,
        retries: 0, // Mandatory zero retry
      }
    );

    const campaign = normalizeCampaignDetail(response.data);
    return { ok: true, data: campaign };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}

/**
 * Updates an existing campaign.
 *
 * Backend endpoint: PATCH /api/v1/campaigns/{campaign_id}
 *
 * Authorization: Campaign owner only. Admin role does NOT bypass ownership.
 * State transition: Blocked on COMPLETED and CANCELLED.
 * Zero automatic retries.
 */
export async function updateCampaignAction(
  id: string,
  input: UpdateCampaignInput,
  options?: ActionInvocationOptions
): Promise<ActionResult<CampaignDetail>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!id || typeof id !== "string") {
      throw new Error("Invalid campaign ID: expected a non-empty string");
    }

    const targetAmount = validateMonetaryAmount(input.targetAmount, "targetAmount");

    const client = options?.client || serverApiClient;
    const response = await client.patch<RawBackendCampaign>(
      `/campaigns/${encodeURIComponent(id)}`,
      {
        title: String(input.title ?? "").trim(),
        description: String(input.description ?? "").trim(),
        category_id: String(input.categoryId ?? "").trim(),
        target_amount: targetAmount,
        start_at: String(input.startAt ?? ""),
        end_at: String(input.endAt ?? ""),
      },
      {
        requestId,
        retries: 0, // Mandatory zero retry
      }
    );

    const campaign = normalizeCampaignDetail(response.data);

    // Invalidate campaign detail cache
    safeRevalidateTag(`campaign-${id}`);

    // If campaign is active, invalidate public list cache
    if (campaign.status === "ACTIVE") {
      safeRevalidateTag("campaigns");
    }

    return { ok: true, data: campaign };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}

/**
 * Submits a draft campaign for administrative review.
 *
 * Backend endpoint: POST /api/v1/campaigns/{campaign_id}/submit-review
 *
 * Authorization: Campaign owner only.
 * State transition: DRAFT → PENDING_REVIEW. Any other state returns 409 Conflict.
 * Zero automatic retries.
 */
export async function submitReviewAction(
  id: string,
  options?: ActionInvocationOptions
): Promise<ActionResult<{ status: string }>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!id || typeof id !== "string") {
      throw new Error("Invalid campaign ID: expected a non-empty string");
    }

    const client = options?.client || serverApiClient;
    await client.post(
      `/campaigns/${encodeURIComponent(id)}/submit-review`,
      {},
      {
        requestId,
        retries: 0,
      }
    );

    safeRevalidateTag(`campaign-${id}`);
    return { ok: true, data: { status: "PENDING_REVIEW" } };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}

/**
 * Approves a campaign and publishes it to the public feed.
 *
 * Backend endpoint: POST /api/v1/campaigns/{campaign_id}/approve
 *
 * Authorization: ADMIN role only. Non-admin receives 403 Forbidden.
 * State transition: PENDING_REVIEW / SUSPENDED → ACTIVE.
 * Zero automatic retries.
 */
export async function approveCampaignAction(
  id: string,
  options?: ActionInvocationOptions
): Promise<ActionResult<{ status: string }>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!id || typeof id !== "string") {
      throw new Error("Invalid campaign ID: expected a non-empty string");
    }

    const client = options?.client || serverApiClient;
    await client.post(
      `/campaigns/${encodeURIComponent(id)}/approve`,
      {},
      {
        requestId,
        retries: 0,
      }
    );

    // Campaign is now active; invalidate both detail and public feed
    safeRevalidateTag("campaigns");
    safeRevalidateTag(`campaign-${id}`);

    return { ok: true, data: { status: "ACTIVE" } };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}

/**
 * Rejects a campaign during review.
 *
 * Backend endpoint: POST /api/v1/campaigns/{campaign_id}/reject
 *
 * Authorization: ADMIN role only. Non-admin receives 403 Forbidden.
 * State transition: PENDING_REVIEW → REJECTED.
 * Zero automatic retries.
 */
export async function rejectCampaignAction(
  id: string,
  reason: string,
  options?: ActionInvocationOptions
): Promise<ActionResult<{ status: string }>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!id || typeof id !== "string") {
      throw new Error("Invalid campaign ID: expected a non-empty string");
    }

    const client = options?.client || serverApiClient;
    await client.post(
      `/campaigns/${encodeURIComponent(id)}/reject`,
      {
        reason: String(reason ?? "").trim(),
      },
      {
        requestId,
        retries: 0,
      }
    );

    safeRevalidateTag(`campaign-${id}`);
    return { ok: true, data: { status: "REJECTED" } };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}

/**
 * Suspends an active campaign.
 *
 * Backend endpoint: POST /api/v1/campaigns/{campaign_id}/suspend
 *
 * Authorization: ADMIN role only. Non-admin receives 403 Forbidden.
 * State transition: ACTIVE → SUSPENDED.
 * Zero automatic retries.
 */
export async function suspendCampaignAction(
  id: string,
  options?: ActionInvocationOptions
): Promise<ActionResult<{ status: string }>> {
  const requestId = await resolveActionRequestId(options?.requestId);

  try {
    if (!id || typeof id !== "string") {
      throw new Error("Invalid campaign ID: expected a non-empty string");
    }

    const client = options?.client || serverApiClient;
    await client.post(
      `/campaigns/${encodeURIComponent(id)}/suspend`,
      {},
      {
        requestId,
        retries: 0,
      }
    );

    // Campaign is no longer active; invalidate both public feed and detail
    safeRevalidateTag("campaigns");
    safeRevalidateTag(`campaign-${id}`);

    return { ok: true, data: { status: "SUSPENDED" } };
  } catch (err: unknown) {
    return { ok: false, error: toActionError(err, requestId) };
  }
}
