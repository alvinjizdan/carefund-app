import { NextResponse } from "next/server";
import {
  CF_ACCESS_TOKEN_COOKIE,
  CF_REFRESH_TOKEN_COOKIE,
  ACCESS_TOKEN_COOKIE_OPTIONS,
  REFRESH_TOKEN_COOKIE_OPTIONS,
} from "./constants";
import { TokenPair } from "./types";

/**
 * Attaches cf_access_token and cf_refresh_token HttpOnly cookies to a NextResponse.
 */
export function setAuthCookies(
  response: NextResponse,
  tokens: TokenPair
): void {
  response.cookies.set(
    CF_ACCESS_TOKEN_COOKIE,
    tokens.accessToken,
    ACCESS_TOKEN_COOKIE_OPTIONS
  );
  response.cookies.set(
    CF_REFRESH_TOKEN_COOKIE,
    tokens.refreshToken,
    REFRESH_TOKEN_COOKIE_OPTIONS
  );
}

/**
 * Clears both session cookies on a NextResponse by expiring them immediately.
 */
export function clearAuthCookies(response: NextResponse): void {
  response.cookies.set(CF_ACCESS_TOKEN_COOKIE, "", {
    ...ACCESS_TOKEN_COOKIE_OPTIONS,
    maxAge: 0,
    expires: new Date(0),
  });
  response.cookies.set(CF_REFRESH_TOKEN_COOKIE, "", {
    ...REFRESH_TOKEN_COOKIE_OPTIONS,
    maxAge: 0,
    expires: new Date(0),
  });
}

/**
 * Constructs an updated Cookie request header string by inserting or updating
 * cf_access_token and cf_refresh_token, preserving all other existing cookies.
 */
export function updateCookieHeader(
  currentHeader: string | null,
  tokens: TokenPair
): string {
  const map = new Map<string, string>();
  if (currentHeader) {
    for (const part of currentHeader.split(";")) {
      const idx = part.indexOf("=");
      if (idx !== -1) {
        const key = part.slice(0, idx).trim();
        const val = part.slice(idx + 1).trim();
        if (key) map.set(key, val);
      }
    }
  }
  map.set(CF_ACCESS_TOKEN_COOKIE, tokens.accessToken);
  map.set(CF_REFRESH_TOKEN_COOKIE, tokens.refreshToken);
  return Array.from(map.entries())
    .map(([k, v]) => `${k}=${v}`)
    .join("; ");
}
