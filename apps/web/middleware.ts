import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import {
  CF_ACCESS_TOKEN_COOKIE,
  CF_REFRESH_TOKEN_COOKIE,
} from "@/lib/auth/constants";
import { setAuthCookies, clearAuthCookies, updateCookieHeader } from "@/lib/auth/cookies";
import { verifyJwtHs256, isJwtExpired } from "@/lib/auth/crypto";
import { sanitizeRedirectPath } from "@/lib/auth/redirect";
import { singleFlightRefresh } from "@/lib/auth/single-flight";
import { BackendRefreshData, TokenPair } from "@/lib/auth/types";

/**
 * Performs a single token refresh attempt against the Go backend.
 */
async function performRefresh(
  refreshToken: string,
  requestId: string
): Promise<{ status: number; tokenPair?: TokenPair; error?: string }> {
  try {
    const tokenPair = await singleFlightRefresh(refreshToken, async () => {
      const res = await fetch(`${serverEnv.goApiInternalUrl}/auth/refresh`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Accept": "application/json",
          "X-Request-ID": requestId,
        },
        body: JSON.stringify({ refresh_token: refreshToken }),
        cache: "no-store",
      });

      if (!res.ok) {
        const err = new Error("Token refresh failed");
        (err as unknown as { status: number }).status = res.status;
        throw err;
      }

      const rawJson = (await res.json()) as { data?: BackendRefreshData };
      const data = rawJson?.data;
      if (!data?.access_token || !data?.refresh_token) {
        const err = new Error("Incomplete credentials");
        (err as unknown as { status: number }).status = 500;
        throw err;
      }

      return {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
      };
    });

    return { status: 200, tokenPair };
  } catch (err: unknown) {
    const status = (err as { status?: number })?.status || 500;
    return { status, error: (err as Error)?.message || "Refresh error" };
  }
}

export async function middleware(req: NextRequest) {
  const { pathname, search } = req.nextUrl;

  // 1. Strict Route Exclusions
  // Exclude auth route handlers, static assets, and internal routes from refresh logic
  if (
    pathname.startsWith("/api/auth") ||
    pathname === "/unauthorized" ||
    pathname.startsWith("/_next") ||
    pathname === "/favicon.ico"
  ) {
    return NextResponse.next();
  }

  const requestId = req.headers.get("x-request-id") || crypto.randomUUID();
  const accessToken = req.cookies.get(CF_ACCESS_TOKEN_COOKIE)?.value;
  const refreshToken = req.cookies.get(CF_REFRESH_TOKEN_COOKIE)?.value;

  const isDonorRoute =
    pathname.startsWith("/dashboard") || pathname.startsWith("/donations");
  const isAdminRoute = pathname.startsWith("/admin");
  const isProtectedRoute = isDonorRoute || isAdminRoute;

  // 2. Unauthenticated check on protected routes
  if (isProtectedRoute && !accessToken && !refreshToken) {
    const safeRedirect = sanitizeRedirectPath(pathname + search);
    const loginUrl = new URL("/login", req.url);
    loginUrl.searchParams.set("redirect", safeRedirect);
    return NextResponse.redirect(loginUrl);
  }

  // 3. Determine if access token requires refresh
  const isExpired = !accessToken || isJwtExpired(accessToken);
  const canRefresh = isExpired && Boolean(refreshToken);

  let activeAccessToken = accessToken;
  let activeRefreshToken = refreshToken;
  let rotatedTokenPair: TokenPair | undefined;

  // 4. Execute Middleware Session Refresh if needed
  if (canRefresh && refreshToken) {
    const refreshResult = await performRefresh(refreshToken, requestId);

    if (refreshResult.status === 200 && refreshResult.tokenPair) {
      rotatedTokenPair = refreshResult.tokenPair;
      activeAccessToken = rotatedTokenPair.accessToken;
      activeRefreshToken = rotatedTokenPair.refreshToken;
    } else if (refreshResult.status === 401) {
      // Refresh token is expired or revoked -> clear cookies
      if (isProtectedRoute) {
        const safeRedirect = sanitizeRedirectPath(pathname + search);
        const loginUrl = new URL("/login", req.url);
        loginUrl.searchParams.set("redirect", safeRedirect);
        const redirectRes = NextResponse.redirect(loginUrl);
        clearAuthCookies(redirectRes);
        return redirectRes;
      }

      // Public route: clear stale cookies and continue
      const publicRes = NextResponse.next();
      clearAuthCookies(publicRes);
      return publicRes;
    } else {
      // 429, 500, 504, or network failure -> do NOT clear valid cookies
      // Allow request through with error indicator header
      if (isProtectedRoute) {
        const requestHeaders = new Headers(req.headers);
        requestHeaders.set("x-session-error", "1");
        requestHeaders.set("x-request-id", requestId);
        return NextResponse.next({ request: { headers: requestHeaders } });
      }
      return NextResponse.next();
    }
  }

  // 5. Admin Coarse Gating
  if (isAdminRoute && activeAccessToken) {
    const jwtSecret = process.env.AUTH_JWT_SECRET;
    if (jwtSecret) {
      const verification = await verifyJwtHs256(activeAccessToken, jwtSecret);

      if (verification.valid && verification.claims) {
        const roles = verification.claims.roles;
        const isAdmin = Array.isArray(roles) && roles.includes("ADMIN");

        if (!isAdmin) {
          const unauthorizedUrl = new URL("/unauthorized", req.url);
          const redirectRes = NextResponse.redirect(unauthorizedUrl);
          if (rotatedTokenPair) {
            setAuthCookies(redirectRes, rotatedTokenPair);
          }
          return redirectRes;
        }
      } else if (!activeRefreshToken) {
        const safeRedirect = sanitizeRedirectPath(pathname + search);
        const loginUrl = new URL("/login", req.url);
        loginUrl.searchParams.set("redirect", safeRedirect);
        const redirectRes = NextResponse.redirect(loginUrl);
        if (rotatedTokenPair) {
          setAuthCookies(redirectRes, rotatedTokenPair);
        }
        return redirectRes;
      }
    }
    // If AUTH_JWT_SECRET is missing, do NOT trust unverified claims;
    // allow through to Go backend where Go authoritatively returns 403 Forbidden.
  }

  // 6. Construct Response with Cookie & Header Propagation
  let response: NextResponse;

  if (rotatedTokenPair) {
    // Both downstream request header mutation and upstream response Set-Cookie are mandatory
    const requestHeaders = new Headers(req.headers);
    const updatedCookieHeader = updateCookieHeader(
      req.headers.get("cookie"),
      rotatedTokenPair
    );
    requestHeaders.set("cookie", updatedCookieHeader);
    requestHeaders.set("x-request-id", requestId);

    response = NextResponse.next({
      request: {
        headers: requestHeaders,
      },
    });

    // Set rotated cookies on the outgoing response for the browser
    setAuthCookies(response, rotatedTokenPair);
  } else {
    response = NextResponse.next();
  }

  // 7. Cache-Control Security for Authenticated / Protected Responses
  if (isProtectedRoute) {
    response.headers.set(
      "Cache-Control",
      "private, no-cache, no-store, max-age=0, must-revalidate"
    );
    response.headers.set("Pragma", "no-cache");
  }

  return response;
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - static image formats (svg, png, jpg, jpeg, gif, webp)
     */
    "/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)",
  ],
};
