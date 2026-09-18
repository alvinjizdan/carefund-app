/**
 * Request Correlation Utility
 *
 * Ensures every incoming and outgoing request carries a sanitized X-Request-ID.
 * Validates incoming headers (alphanumeric, dashes, underscores, max length 64).
 * Falls back to crypto.randomUUID() for missing or invalid IDs.
 */

const REQUEST_ID_REGEX = /^[a-zA-Z0-9_-]{1,64}$/;

export function getOrGenerateRequestId(req?: Request): string {
  if (!req) {
    return crypto.randomUUID();
  }

  const existing = req.headers.get("x-request-id") || req.headers.get("X-Request-ID");
  if (existing && REQUEST_ID_REGEX.test(existing)) {
    return existing;
  }

  return crypto.randomUUID();
}
