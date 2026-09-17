/**
 * CareFund Web Environment Contract
 * Strictly exposes only validated, browser-safe public variables.
 */

export const env = {
  apiBaseUrl: process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080/api/v1",
  midtransClientKey: process.env.NEXT_PUBLIC_MIDTRANS_CLIENT_KEY || "",
  midtransIsProduction: process.env.NEXT_PUBLIC_MIDTRANS_IS_PRODUCTION === "true",
} as const;
