import { NextRequest, NextResponse } from "next/server";
import { serverEnv } from "@/lib/server/env";
import { CF_ACCESS_TOKEN_COOKIE } from "@/lib/auth/constants";
import { getOrGenerateRequestId } from "@/lib/server/correlation";
import { createErrorResponse, forwardBackendError } from "@/lib/server/api-error";
import { BackendMeData, BffSuccessEnvelope, User } from "@/lib/auth/types";

export async function GET(req: NextRequest) {
  const requestId = getOrGenerateRequestId(req);

  const accessToken = req.cookies.get(CF_ACCESS_TOKEN_COOKIE)?.value;
  if (!accessToken) {
    return createErrorResponse(
      401,
      "UNAUTHORIZED",
      "Not authenticated",
      requestId
    );
  }

  let backendRes: Response;
  try {
    backendRes = await fetch(`${serverEnv.goApiInternalUrl}/me`, {
      method: "GET",
      headers: {
        "Authorization": `Bearer ${accessToken}`,
        "Accept": "application/json",
        "X-Request-ID": requestId,
      },
    });
  } catch {
    return createErrorResponse(
      504,
      "TIMEOUT",
      "Unable to communicate with user service",
      requestId
    );
  }

  if (!backendRes.ok) {
    return forwardBackendError(backendRes, requestId);
  }

  let rawJson: { data?: BackendMeData };
  try {
    rawJson = await backendRes.json();
  } catch {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "Invalid response format from user service",
      requestId
    );
  }

  const meData = rawJson?.data;
  if (!meData?.id || !meData?.email) {
    return createErrorResponse(
      500,
      "INTERNAL_ERROR",
      "User service returned incomplete profile data",
      requestId
    );
  }

  const canonicalUser: User = {
    id: meData.id,
    name: meData.name,
    email: meData.email,
    roles: Array.isArray(meData.roles) ? meData.roles : [],
  };

  return NextResponse.json<BffSuccessEnvelope<{ user: User }>>(
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
}
