/**
 * CareFund Redirect Path Sanitization
 *
 * Enforces strict local application path boundaries to prevent Open Redirect vulnerabilities.
 * Rejects external URLs, protocol-relative URLs, schemes, backslash evasions, and CRLF injection.
 */

export function sanitizeRedirectPath(path: unknown, fallback = "/"): string {
  if (typeof path !== "string" || !path.trim()) {
    return fallback;
  }

  const trimmed = path.trim();

  // Reject protocol-relative URLs (//evil.com) and backslash evasions (/\evil.com, \\evil.com)
  if (
    trimmed.startsWith("//") ||
    trimmed.startsWith("/\\") ||
    trimmed.startsWith("\\\\") ||
    trimmed.startsWith("\\/")
  ) {
    return fallback;
  }

  // Must strictly start with a single forward slash
  if (!trimmed.startsWith("/")) {
    return fallback;
  }

  // Check the path after the leading slash for any URI scheme (e.g. javascript:, https:, http:, data:)
  const withoutLeadingSlash = trimmed.slice(1);
  if (
    /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(withoutLeadingSlash) ||
    withoutLeadingSlash.startsWith("//")
  ) {
    return fallback;
  }

  // Reject control characters, newlines, null bytes, tabs (CRLF / header injection prevention)
  if (/[\r\n\0\t\x00-\x1f\x7f]/.test(trimmed)) {
    return fallback;
  }

  return trimmed;
}
