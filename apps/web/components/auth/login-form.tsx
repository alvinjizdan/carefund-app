"use client";

/**
 * CareFund Login Form (Client Component)
 *
 * Submits credentials to the EXISTING same-origin BFF route `POST /api/auth/login`.
 * The Go API is never contacted from the browser, and no Go internal URL is
 * referenced here.
 *
 * SECURITY INVARIANTS:
 * 1. Only `email` and `password` are sent. No user_id / role / owner_id / token.
 * 2. The response body is deliberately NEVER read on success. The BFF returns only
 *    the user profile; access + refresh tokens are delivered exclusively as HttpOnly
 *    cookies by the BFF and are therefore never reachable from JavaScript.
 * 3. Authentication error copy is derived from the HTTP status alone. Backend error
 *    strings are never rendered, so no internal detail can reach the UI.
 * 4. The post-login destination is re-sanitized client-side before navigation
 *    (defence in depth), so an attacker-supplied `from` cannot become an external redirect.
 */

import * as React from "react";
import { Input } from "@/components/forms/input";
import {
  FormField,
  FormLabel,
  FormErrorMessage,
} from "@/components/forms/form-field";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { sanitizeRedirectPath } from "@/lib/auth/redirect";

/** Fallback destination when `from` is absent or rejected by the sanitizer. */
const DEFAULT_REDIRECT = "/me";

/**
 * Maps a BFF login HTTP status to safe, user-facing copy.
 *
 * NOTE ON 404: the Go API intentionally returns `NOT_FOUND` for BOTH an unknown
 * email and an incorrect password (`user_service.Login` returns `domain.ErrNotFound`
 * in both branches, with an explicit anti-enumeration comment). 404 and 401 therefore
 * collapse into ONE indistinguishable message so the UI cannot be used to probe
 * whether an email address is registered.
 */
function mapLoginErrorToMessage(status: number): string {
  switch (status) {
    case 400:
      return "Email and password are required. Please complete both fields.";
    case 401:
    case 404:
      return "Email or password is incorrect. Please try again.";
    case 429:
      return "Too many sign-in attempts. Please wait a moment before trying again.";
    default:
      if (status >= 500) {
        return "Sign-in is temporarily unavailable. Please try again shortly.";
      }
      return "Sign-in could not be completed. Please try again.";
  }
}

export interface LoginFormProps {
  /** Already-sanitized local path supplied by the Server Component. */
  redirectTo?: string;
}

interface LoginFieldErrors {
  email?: string;
  password?: string;
}

/** UX-only validation. Go remains the authentication and validation authority. */
function validateCredentials(email: string, password: string): LoginFieldErrors {
  const errors: LoginFieldErrors = {};
  const trimmedEmail = email.trim();

  if (!trimmedEmail) {
    errors.email = "Email address is required.";
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
    errors.email = "Enter a valid email address.";
  }

  if (!password) {
    errors.password = "Password is required.";
  }

  return errors;
}

export function LoginForm({ redirectTo }: LoginFormProps) {
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [fieldErrors, setFieldErrors] = React.useState<LoginFieldErrors>({});
  const [formError, setFormError] = React.useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = React.useState(false);

  // Re-sanitize at the point of use. Never navigate to a destination that the
  // sanitizer has not approved, even if this component is reused elsewhere.
  const safeRedirect = React.useMemo(
    () => sanitizeRedirectPath(redirectTo, DEFAULT_REDIRECT),
    [redirectTo]
  );

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    // Duplicate-submission guard: ignore re-entry while a request is in flight.
    if (isSubmitting) {
      return;
    }

    setFormError(null);

    const nextErrors = validateCredentials(email, password);
    setFieldErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    setIsSubmitting(true);

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({ email: email.trim(), password }),
        credentials: "same-origin",
      });

      if (!res.ok) {
        setFormError(mapLoginErrorToMessage(res.status));
        // Retain the email so the user does not retype it; clear the credential.
        setPassword("");
        setIsSubmitting(false);
        return;
      }

      // Success body intentionally not parsed — no token is ever exposed to JS.
      // Full document navigation guarantees the destination Server Component
      // renders against the freshly-set HttpOnly cookies.
      window.location.assign(safeRedirect);
    } catch {
      setFormError(
        "Unable to reach the authentication service. Check your connection and try again."
      );
      setPassword("");
      setIsSubmitting(false);
    }
  };

  const emailErrorId = "login-email-error";
  const passwordErrorId = "login-password-error";

  return (
    <form onSubmit={handleSubmit} className="space-y-5" noValidate>
      {formError && (
        <Alert variant="destructive">
          <AlertTitle>Unable to sign in</AlertTitle>
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}

      <FormField id="login-email" error={fieldErrors.email} required>
        <FormLabel>Email Address</FormLabel>
        <Input
          id="login-email"
          name="email"
          type="email"
          autoComplete="email"
          inputMode="email"
          placeholder="you@example.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={isSubmitting}
          aria-invalid={Boolean(fieldErrors.email)}
          aria-describedby={fieldErrors.email ? emailErrorId : undefined}
        />
        <FormErrorMessage id={emailErrorId}>{fieldErrors.email}</FormErrorMessage>
      </FormField>

      <FormField id="login-password" error={fieldErrors.password} required>
        <FormLabel>Password</FormLabel>
        <Input
          id="login-password"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder="••••••••"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={isSubmitting}
          aria-invalid={Boolean(fieldErrors.password)}
          aria-describedby={fieldErrors.password ? passwordErrorId : undefined}
        />
        <FormErrorMessage id={passwordErrorId}>
          {fieldErrors.password}
        </FormErrorMessage>
      </FormField>

      <Button
        type="submit"
        variant="primary"
        size="lg"
        className="w-full"
        isLoading={isSubmitting}
        disabled={isSubmitting}
      >
        {isSubmitting ? "Signing in…" : "Sign In"}
      </Button>
    </form>
  );
}
