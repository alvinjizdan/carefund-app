import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import {
  CF_ACCESS_TOKEN_COOKIE,
  CF_REFRESH_TOKEN_COOKIE,
} from "@/lib/auth/constants";
import { clearAuthCookies } from "@/lib/auth/cookies";
import { getOrGenerateRequestId } from "@/lib/server/correlation";
import { BffSuccessEnvelope } from "@/lib/auth/types";

export async function POST(req: NextRequest) {
  const requestId = getOrGenerateRequestId(req);

  const accessToken = req.cookies.get(CF_ACCESS_TOKEN_COOKIE)?.value;
  const refreshToken = req.cookies.get(CF_REFRESH_TOKEN_COOKIE)?.value;

  // Prepare backend request headers and body
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept": "application/json",
    "X-Request-ID": requestId,
  };

  if (accessToken) {
    headers["Authorization"] = `Bearer ${accessToken}`;
  }

  // Attempt backend token revocation server-side
  try {
    await fetch(`${serverEnv.goApiInternalUrl}/auth/logout`, {
      method: "POST",
      headers,
      body: refreshToken ? JSON.stringify({ refresh_token: refreshToken }) : undefined,
    });
  } catch {
    // Backend connectivity issue should not prevent client session termination
  }

  // Always clear session cookies on client response
  const response = NextResponse.json<BffSuccessEnvelope<{ success: boolean }>>(
    {
      data: {
        success: true,
      },
    },
    {
      status: 200,
      headers: {
        "X-Request-ID": requestId,
        "Content-Type": "application/json",
      },
    }
  );

  clearAuthCookies(response);
  return response;
}
