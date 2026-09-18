/**
 * CareFund Native Web Crypto JWT Verification
 *
 * Implements strict HMAC-SHA256 (HS256) signature and expiration verification
 * using exclusively the standard Web Crypto API (crypto.subtle).
 * Fully compatible with Next.js Edge Runtime and Node.js with zero external dependencies.
 *
 * CRITICAL SECURITY INVARIANT:
 * Never decode or trust JWT claims without cryptographic signature and expiration validation.
 */

export interface JwtVerificationResult {
  valid: boolean;
  claims?: {
    sub?: string;
    roles?: string[];
    exp?: number;
    iat?: number;
    jti?: string;
    [key: string]: unknown;
  };
  reason?: string;
}

/**
 * Decodes a Base64URL string to a Uint8Array.
 */
export function base64UrlDecode(str: string): Uint8Array {
  let base64 = str.replace(/-/g, "+").replace(/_/g, "/");
  while (base64.length % 4) {
    base64 += "=";
  }
  const binStr = atob(base64);
  const bytes = new Uint8Array(binStr.length);
  for (let i = 0; i < binStr.length; i++) {
    bytes[i] = binStr.charCodeAt(i);
  }
  return bytes;
}

/**
 * Cryptographically verifies an HS256 JWT using Web Crypto API.
 * Validates header, HMAC signature, and exp claim.
 */
export async function verifyJwtHs256(
  tokenStr: string,
  secret: string
): Promise<JwtVerificationResult> {
  if (!tokenStr || !secret) {
    return { valid: false, reason: "Missing token or secret" };
  }

  const parts = tokenStr.split(".");
  if (parts.length !== 3) {
    return { valid: false, reason: "Malformed JWT structure" };
  }

  const [headerB64, payloadB64, signatureB64] = parts;

  try {
    const headerBytes = base64UrlDecode(headerB64);
    const headerJson = JSON.parse(new TextDecoder().decode(headerBytes));
    if (headerJson.alg !== "HS256" || headerJson.typ !== "JWT") {
      return { valid: false, reason: "Unsupported algorithm or type" };
    }

    const enc = new TextEncoder();
    const key = await crypto.subtle.importKey(
      "raw",
      enc.encode(secret),
      { name: "HMAC", hash: "SHA-256" },
      false,
      ["verify"]
    );

    const data = enc.encode(`${headerB64}.${payloadB64}`);
    const signatureBytes = base64UrlDecode(signatureB64);

    const isSignatureValid = await crypto.subtle.verify(
      "HMAC",
      key,
      signatureBytes as unknown as BufferSource,
      data
    );

    if (!isSignatureValid) {
      return { valid: false, reason: "Invalid signature" };
    }

    const payloadBytes = base64UrlDecode(payloadB64);
    const payloadJson = JSON.parse(new TextDecoder().decode(payloadBytes));

    const now = Math.floor(Date.now() / 1000);
    if (typeof payloadJson.exp === "number" && payloadJson.exp <= now) {
      return { valid: false, reason: "Token expired" };
    }

    return { valid: true, claims: payloadJson };
  } catch {
    return { valid: false, reason: "Verification exception" };
  }
}

/**
 * Inspects a JWT expiration claim (exp) without validating its signature.
 * NOTE: This is used solely to determine whether a token requires refresh.
 * It MUST NEVER be used to trust authorization claims or roles.
 */
export function isJwtExpired(tokenStr: string): boolean {
  if (!tokenStr) return true;
  const parts = tokenStr.split(".");
  if (parts.length !== 3) return true;
  try {
    const payloadBytes = base64UrlDecode(parts[1]);
    const payloadJson = JSON.parse(new TextDecoder().decode(payloadBytes));
    if (typeof payloadJson.exp !== "number") return true;
    const now = Math.floor(Date.now() / 1000);
    return payloadJson.exp <= now;
  } catch {
    return true;
  }
}
