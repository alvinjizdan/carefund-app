/**
 * Request Correlation Utility
 *
 * Ensures every incoming and outgoing request carries a sanitized X-Request-ID.
 * Validates incoming headers (alphanumeric, dashes, underscores, max length 64).
 * Falls back to crypto.randomUUID() for missing or invalid IDs.
 */

const REQUEST_ID_REGEX = /^[a-zA-Z0-9_-]{1,64}$/;

export function getOrGenerateRequestId(
  req?: Request | Headers | { get(name: string): string | null } | string
): string {
  if (!req) {
    return crypto.randomUUID();
  }

  if (typeof req === "string") {
    return REQUEST_ID_REGEX.test(req) ? req : crypto.randomUUID();
  }

  const headers =
    "headers" in req && req.headers
      ? (req.headers as { get(name: string): string | null })
      : (req as { get(name: string): string | null });

  const existing = headers.get("x-request-id") || headers.get("X-Request-ID");
  if (existing && REQUEST_ID_REGEX.test(existing)) {
    return existing;
  }

  return crypto.randomUUID();
}
