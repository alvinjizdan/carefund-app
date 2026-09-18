import { TokenPair } from "./types";

/**
 * Registry of in-flight token refresh operations keyed by refresh token string.
 *
 * IMPORTANT DEPLOYMENT CONSTRAINT:
 * This in-memory deduplication mechanism is strictly process-local to the running Node.js instance.
 * It prevents concurrent requests within the same server process from redundantly consuming
 * a single-use refresh token.
 *
 * It DOES NOT provide distributed locking across multiple independent server instances or serverless
 * execution environments. If CareFund is scaled horizontally to multiple Node.js instances in the future,
 * cross-instance coordination would require distributed locking (e.g., Redis mutex) or token rotation tolerance.
 */
const inFlightRefreshes = new Map<string, Promise<TokenPair>>();

/**
 * Executes a token refresh with single-flight concurrency deduplication.
 *
 * If a refresh is already in-flight for the provided refresh token, subsequent callers await
 * the same pending Promise rather than initiating a duplicate call to the Go backend.
 *
 * Invariants:
 * 1. Only one network call to Go API /auth/refresh per refresh token.
 * 2. In-flight state is always cleaned up in a finally block (no permanent locks / memory leaks).
 * 3. Failures reject consistently across all waiting callers.
 * 4. Success resolves the exact same TokenPair for all waiting callers.
 */
export async function singleFlightRefresh(
  refreshToken: string,
  executeRefresh: () => Promise<TokenPair>
): Promise<TokenPair> {
  const existingPromise = inFlightRefreshes.get(refreshToken);
  if (existingPromise) {
    return existingPromise;
  }

  const refreshPromise = (async () => {
    try {
      return await executeRefresh();
    } finally {
      inFlightRefreshes.delete(refreshToken);
    }
  })();

  inFlightRefreshes.set(refreshToken, refreshPromise);
  return refreshPromise;
}
