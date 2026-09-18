import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import { CF_REFRESH_TOKEN_COOKIE } from "@/lib/auth/constants";
import { setAuthCookies, clearAuthCookies } from "@/lib/auth/cookies";
import { singleFlightRefresh } from "@/lib/auth/single-flight";
import { getOrGenerateRequestId } from "@/lib/server/correlation";
import {
  createErrorResponse,
  defaultErrorCodeForStatus,
} from "@/lib/server/api-error";
import {
  BackendErrorEnvelope,
  BackendRefreshData,
  BffSuccessEnvelope,
  TokenPair,
} from "@/lib/auth/types";

export async function POST(req: NextRequest) {
  const requestId = getOrGenerateRequestId(req);

  // Read refresh token from HttpOnly cookie first, with request body as fallback
  let refreshToken = req.cookies.get(CF_REFRESH_TOKEN_COOKIE)?.value;

  if (!refreshToken) {
    try {
      const body = (await req.json()) as { refresh_token?: string };
      if (typeof body?.refresh_token === "string" && body.refresh_token) {
        refreshToken = body.refresh_token;
      }
    } catch {
      // Body not provided or malformed, continue with empty token
    }
  }

  if (!refreshToken) {
    const res = createErrorResponse(
      401,
      "UNAUTHORIZED",
      "Refresh token missing",
      requestId
    );
    clearAuthCookies(res);
    return res;
  }

  try {
    const tokenPair: TokenPair = await singleFlightRefresh(
      refreshToken,
      async () => {
        let backendRes: Response;
        try {
          backendRes = await fetch(
            `${serverEnv.goApiInternalUrl}/auth/refresh`,
            {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "X-Request-ID": requestId,
              },
              body: JSON.stringify({ refresh_token: refreshToken }),
            }
          );
        } catch {
          const timeoutErr = new Error(
            "Unable to communicate with authentication service"
          );
          (timeoutErr as unknown as { status: number; code: string }).status = 504;
          (timeoutErr as unknown as { status: number; code: string }).code = "TIMEOUT";
          throw timeoutErr;
        }

        if (!backendRes.ok) {
          let errorPayload: BackendErrorEnvelope | null = null;
          try {
            errorPayload = (await backendRes.json()) as BackendErrorEnvelope;
          } catch {
            // Non-JSON response
          }

          const status = backendRes.status;
          const code =
            errorPayload?.error?.code || defaultErrorCodeForStatus(status);
          const message =
            errorPayload?.error?.message ||
            (status === 401
              ? "Session expired or revoked. Please log in again."
              : `Token refresh failed with status ${status}`);

          const backendErr = new Error(message);
          (backendErr as unknown as { status: number; code: string }).status = status;
          (backendErr as unknown as { status: number; code: string }).code = code;
          throw backendErr;
        }

        let rawJson: { data?: BackendRefreshData };
        try {
          rawJson = await backendRes.json();
        } catch {
          const parseErr = new Error(
            "Invalid response format from authentication service"
          );
          (parseErr as unknown as { status: number; code: string }).status = 500;
          (parseErr as unknown as { status: number; code: string }).code = "INTERNAL_ERROR";
          throw parseErr;
        }

        const refreshData = rawJson?.data;
        if (!refreshData?.access_token || !refreshData?.refresh_token) {
          const incompleteErr = new Error(
            "Authentication service returned incomplete credentials"
          );
          (incompleteErr as unknown as { status: number; code: string }).status = 500;
          (incompleteErr as unknown as { status: number; code: string }).code =
            "INTERNAL_ERROR";
          throw incompleteErr;
        }

        return {
          accessToken: refreshData.access_token,
          refreshToken: refreshData.refresh_token,
        };
      }
    );

    const response = NextResponse.json<BffSuccessEnvelope<{ success: boolean }>>(
      {
        data: {
          success: true,
        },
      },
      {
        status: 200,
        headers: {
          "X-Request-ID": requestId,
          "Content-Type": "application/json",
        },
      }
    );

    setAuthCookies(response, tokenPair);
    return response;
  } catch (err: unknown) {
    const status = (err as { status?: number })?.status || 500;
    const code = (err as { code?: string })?.code || "INTERNAL_ERROR";
    const message =
      (err as { message?: string })?.message || "Token refresh failed";

    const response = createErrorResponse(status, code, message, requestId);

    // If the refresh token was rejected by backend (401), clear both cookies
    if (status === 401) {
      clearAuthCookies(response);
    }

    return response;
  }
}
