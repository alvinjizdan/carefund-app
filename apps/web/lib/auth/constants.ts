/**
 * CareFund Session & Cookie Security Constants
 *
 * Enforces strict cookie parameters:
 * - cf_access_token: Path=/, Max-Age=900 (15 min)
 * - cf_refresh_token: Path=/api/auth, Max-Age=604800 (7 days)
 * - Both: HttpOnly, SameSite=Lax, Secure in production
 *
 * NOTE: cf_user is intentionally omitted per F3.1 architecture (deferred as optional optimization).
 */

export const CF_ACCESS_TOKEN_COOKIE = "cf_access_token" as const;
export const CF_REFRESH_TOKEN_COOKIE = "cf_refresh_token" as const;

export const ACCESS_TOKEN_MAX_AGE = 900; // 15 minutes in seconds
export const REFRESH_TOKEN_MAX_AGE = 604800; // 7 days in seconds

export const ACCESS_TOKEN_COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "lax" as const,
  path: "/",
  maxAge: ACCESS_TOKEN_MAX_AGE,
} as const;

export const REFRESH_TOKEN_COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "lax" as const,
  path: "/api/auth",
  maxAge: REFRESH_TOKEN_MAX_AGE,
} as const;
