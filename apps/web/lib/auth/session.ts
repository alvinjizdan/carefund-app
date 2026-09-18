import { cookies } from "next/headers";
import { serverEnv } from "@/lib/server/env";
import { CF_ACCESS_TOKEN_COOKIE } from "./constants";
import { BackendMeData, User } from "./types";

export type SessionStatus =
  | "AUTHENTICATED"
  | "UNAUTHENTICATED"
  | "SESSION_ERROR";

export interface SessionResult {
  status: SessionStatus;
  user: User | null;
  error?: {
    code: string;
    message: string;
  };
}

/**
 * Resolves the current session state server-side for Server Components.
 *
 * Invariants:
 * 1. Reads the effective request access token (propagated from middleware or request cookies).
 * 2. Calls GET /api/v1/me with the access token.
 * 3. Returns AUTHENTICATED when valid.
 * 4. Returns UNAUTHENTICATED when token is missing or rejected with 401.
 * 5. Returns SESSION_ERROR when backend returns 5xx or network timeout (never silently converted to unauthenticated).
 *
 * NOTE: Server Components do NOT consume or rotate refresh tokens during page rendering
 * because Next.js App Router prevents mutating response cookies during document render.
 * Session token refresh is handled upstream by Next.js Edge Middleware, which updates both
 * downstream request headers and upstream response Set-Cookie headers atomically.
 */
export async function getSession(): Promise<SessionResult> {
  let cookieStore;
  try {
    cookieStore = await cookies();
  } catch {
    // cookies() may throw if called outside a request context
    return { status: "UNAUTHENTICATED", user: null };
  }

  const accessToken = cookieStore.get(CF_ACCESS_TOKEN_COOKIE)?.value;

  if (!accessToken) {
    return { status: "UNAUTHENTICATED", user: null };
  }

  const requestId = crypto.randomUUID();

  try {
    const meRes = await fetch(`${serverEnv.goApiInternalUrl}/me`, {
      method: "GET",
      headers: {
        "Authorization": `Bearer ${accessToken}`,
        "Accept": "application/json",
        "X-Request-ID": requestId,
      },
      cache: "no-store",
    });

    if (meRes.ok) {
      const json = (await meRes.json()) as { data?: BackendMeData };
      const meData = json?.data;
      if (meData?.id && meData?.email) {
        return {
          status: "AUTHENTICATED",
          user: {
            id: meData.id,
            name: meData.name,
            email: meData.email,
            roles: Array.isArray(meData.roles) ? meData.roles : [],
          },
        };
      }
    }

    if (meRes.status === 401) {
      return { status: "UNAUTHENTICATED", user: null };
    }

    return {
      status: "SESSION_ERROR",
      user: null,
      error: {
        code: "SERVICE_ERROR",
        message: `User service responded with status ${meRes.status}`,
      },
    };
  } catch {
    return {
      status: "SESSION_ERROR",
      user: null,
      error: {
        code: "TIMEOUT",
        message: "Unable to reach user authentication service",
      },
    };
  }
}

/**
 * Returns the current authenticated User or null.
 */
export async function getCurrentUser(): Promise<User | null> {
  const session = await getSession();
  return session.user;
}
