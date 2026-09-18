import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import { getOrGenerateRequestId } from "@/lib/server/correlation";
import { createErrorResponse, forwardBackendError } from "@/lib/server/api-error";
import { setAuthCookies } from "@/lib/auth/cookies";
import {
  BackendMeData,
  BackendRegisterData,
  BffSuccessEnvelope,
  User,
} from "@/lib/auth/types";

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

  const { name, email, password } = (body as Record<string, unknown>) || {};
  if (
    typeof name !== "string" ||
    !name.trim() ||
    typeof email !== "string" ||
    !email.trim() ||
    typeof password !== "string" ||
    !password
  ) {
    return createErrorResponse(
      400,
      "INVALID_REQUEST",
      "Name, email, and password are required",
      requestId
    );
  }

  let backendRes: Response;
  try {
    backendRes = await fetch(`${serverEnv.goApiInternalUrl}/auth/register`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Accept": "application/json",
        "X-Request-ID": requestId,
      },
      body: JSON.stringify({
        name: name.trim(),
        email: email.trim(),
        password,
      }),
    });
  } catch {
    return createErrorResponse(
      504,
      "TIMEOUT",
      "Unable to communicate with registration service",
      requestId
    );
  }

  if (!backendRes.ok) {
    return forwardBackendError(backendRes, requestId);
  }

  let rawJson: { data?: BackendRegisterData };
  try {
    rawJson = await backendRes.json();
  } catch {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "Invalid response format from registration service",
      requestId
    );
  }

  const registerData = rawJson?.data;
  if (
    !registerData?.access_token ||
    !registerData?.refresh_token ||
    !registerData?.user
  ) {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "Registration service returned incomplete credentials",
      requestId
    );
  }

  // Obtain authoritative user and role information via GET /api/v1/me
  // without inventing or fabricating roles.
  let canonicalUser: User;
  try {
    const meRes = await fetch(`${serverEnv.goApiInternalUrl}/me`, {
      method: "GET",
      headers: {
        "Authorization": `Bearer ${registerData.access_token}`,
        "Accept": "application/json",
        "X-Request-ID": requestId,
      },
    });

    if (meRes.ok) {
      const meJson = (await meRes.json()) as { data?: BackendMeData };
      const meData = meJson?.data;
      if (meData?.id && meData?.email) {
        canonicalUser = {
          id: meData.id,
          name: meData.name || registerData.user.Name,
          email: meData.email,
          roles: Array.isArray(meData.roles) ? meData.roles : [],
        };
      } else {
        // Fallback: Normalize PascalCase response without fabricating roles
        canonicalUser = {
          id: registerData.user.ID,
          name: registerData.user.Name,
          email: registerData.user.Email,
          roles: [],
        };
      }
    } else {
      // Fallback: Normalize PascalCase response without fabricating roles
      canonicalUser = {
        id: registerData.user.ID,
        name: registerData.user.Name,
        email: registerData.user.Email,
        roles: [],
      };
    }
  } catch {
    // If /me call fails transiently, normalize PascalCase response without fabricating roles
    canonicalUser = {
      id: registerData.user.ID,
      name: registerData.user.Name,
      email: registerData.user.Email,
      roles: [],
    };
  }

  const response = NextResponse.json<BffSuccessEnvelope<{ user: User }>>(
    {
      data: {
        user: canonicalUser,
      },
    },
    {
      status: 201,
      headers: {
        "X-Request-ID": requestId,
        "Content-Type": "application/json",
      },
    }
  );

  setAuthCookies(response, {
    accessToken: registerData.access_token,
    refreshToken: registerData.refresh_token,
  });

  return response;
}
