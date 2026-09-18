/**
 * CareFund Server-Only Environment Configuration
 *
 * CRITICAL SECURITY CONSTRAINTS:
 * 1. This file must NEVER be imported into client components or client bundles.
 * 2. GO_API_INTERNAL_URL is strictly server-to-server and must NEVER be prefixed with NEXT_PUBLIC_.
 * 3. In production, this URL resolves to internal network topology (private VPC, internal DNS, container network).
 */

export const serverEnv = {
  goApiInternalUrl: process.env.GO_API_INTERNAL_URL || "http://localhost:8080/api/v1",
} as const;
