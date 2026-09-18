import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import { getOrGenerateRequestId } from "@/lib/server/correlation";
import { createErrorResponse, forwardBackendError } from "@/lib/server/api-error";
import { setAuthCookies } from "@/lib/auth/cookies";
import { BackendLoginData, BffSuccessEnvelope, User } from "@/lib/auth/types";

export async function POST(req: NextRequest) {
  const requestId = getOrGenerateRequestId(req);

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return createErrorResponse(
      400,
      "INVALID_REQUEST",
      "Malformed JSON request body",
      requestId
    );
  }

  const { email, password } = (body as Record<string, unknown>) || {};
  if (
    typeof email !== "string" ||
    !email.trim() ||
    typeof password !== "string" ||
    !password
  ) {
    return createErrorResponse(
      400,
      "INVALID_REQUEST",
      "Email and password are required",
      requestId
    );
  }

  let backendRes: Response;
  try {
    backendRes = await fetch(`${serverEnv.goApiInternalUrl}/auth/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Accept": "application/json",
        "X-Request-ID": requestId,
      },
      body: JSON.stringify({ email: email.trim(), password }),
    });
  } catch {
    return createErrorResponse(
      504,
      "TIMEOUT",
      "Unable to communicate with authentication service",
      requestId
    );
  }

  if (!backendRes.ok) {
    return forwardBackendError(backendRes, requestId);
  }

  let rawJson: { data?: BackendLoginData };
  try {
    rawJson = await backendRes.json();
  } catch {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "Invalid response format from authentication service",
      requestId
    );
  }

  const loginData = rawJson?.data;
  if (!loginData?.access_token || !loginData?.refresh_token || !loginData?.user) {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "Authentication service returned incomplete credentials",
      requestId
    );
  }

  const canonicalUser: User = {
    id: loginData.user.id,
    name: loginData.user.name,
    email: loginData.user.email,
    roles: Array.isArray(loginData.user.roles) ? loginData.user.roles : [],
  };

  const response = NextResponse.json<BffSuccessEnvelope<{ user: User }>>(
    {
      data: {
        user: canonicalUser,
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

  setAuthCookies(response, {
    accessToken: loginData.access_token,
    refreshToken: loginData.refresh_token,
  });

  return response;
}
