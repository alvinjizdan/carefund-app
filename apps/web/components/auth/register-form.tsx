"use client";

/**
 * CareFund Registration Form (Client Component)
 *
 * Submits to the EXISTING same-origin BFF route `POST /api/auth/register`.
 * The Go API is never contacted from the browser and no Go internal URL is referenced.
 *
 * SECURITY INVARIANTS:
 * 1. Only `name`, `email`, and `password` are sent — exactly the three fields the
 *    backend `registerRequest` struct accepts. No user_id / role / owner_id / token.
 * 2. The success body is deliberately NEVER read. The BFF returns only the user
 *    profile; access and refresh tokens arrive exclusively as HttpOnly cookies.
 * 3. Error copy is derived from the HTTP status alone, so no backend error string,
 *    SQL fragment, or internal detail can reach the UI.
 * 4. The post-signup destination is re-sanitized client-side before navigation.
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

/** Fallback destination when `from` is absent or rejected. Matches the login page. */
const DEFAULT_REDIRECT = "/me";

/**
 * Maps a BFF registration HTTP status to safe, user-facing copy.
 *
 * NOTE ON 409: the backend returns `domain.ErrDuplicate` when the INSERT violates
 * the `email CITEXT UNIQUE` constraint (pq 23505), which `RespondError` maps to
 * 409. Registration has no state-transition path, so 409 here unambiguously means
 * the email is already taken, and the backend intentionally distinguishes it.
 *
 * NOTE: this maps by status, not by the backend's `message`, because that message
 * is internal phrasing ("resource already exists") and is not user-facing.
 */
function mapRegisterErrorToMessage(status: number): string {
  switch (status) {
    case 400:
      return "Name, email, and password are required. Please complete all fields.";
    case 409:
      return "This email is already registered. Please sign in instead.";
    case 429:
      return "Too many registration attempts. Please wait a moment and try again.";
    default:
      if (status >= 500) {
        return "Registration is temporarily unavailable. Please try again shortly.";
      }
      return "Registration could not be completed. Please try again.";
  }
}

export interface RegisterFormProps {
  /** Already-sanitized local path supplied by the Server Component. */
  redirectTo?: string;
}

interface RegisterFieldErrors {
  name?: string;
  email?: string;
  password?: string;
}

/**
 * UX-only validation.
 *
 * Deliberately mirrors the backend, which enforces ONLY non-empty values
 * (`RegisterUser` checks `email == "" || password == "" || name == ""`) and hashes
 * with bcrypt without any length or complexity rule. No password policy is invented
 * here, because none exists server-side. Go remains the authority.
 */
function validateRegistration(
  name: string,
  email: string,
  password: string
): RegisterFieldErrors {
  const errors: RegisterFieldErrors = {};
  const trimmedName = name.trim();
  const trimmedEmail = email.trim();

  if (!trimmedName) {
    errors.name = "Full name is required.";
  }

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

export function RegisterForm({ redirectTo }: RegisterFormProps) {
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [fieldErrors, setFieldErrors] = React.useState<RegisterFieldErrors>({});
  const [formError, setFormError] = React.useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = React.useState(false);

  const safeRedirect = React.useMemo(
    () => sanitizeRedirectPath(redirectTo, DEFAULT_REDIRECT),
    [redirectTo]
  );

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    // Duplicate-submission guard.
    if (isSubmitting) {
      return;
    }

    setFormError(null);

    const nextErrors = validateRegistration(name, email, password);
    setFieldErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    setIsSubmitting(true);

    try {
      const res = await fetch("/api/auth/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        // Email is trimmed but never lowercased: the users table stores
        // `email CITEXT UNIQUE`, so case-insensitive matching already exists
        // in the database. No case transformation is applied here.
        body: JSON.stringify({
          name: name.trim(),
          email: email.trim(),
          password,
        }),
        credentials: "same-origin",
      });

      if (!res.ok) {
        setFormError(mapRegisterErrorToMessage(res.status));
        // Preserve name and email; clear the credential.
        setPassword("");
        setIsSubmitting(false);
        return;
      }

      // Registration auto-authenticates: the BFF has already set the HttpOnly
      // session cookies on this response. Success body intentionally not parsed.
      window.location.assign(safeRedirect);
    } catch {
      setFormError(
        "Unable to reach the registration service. Check your connection and try again."
      );
      setPassword("");
      setIsSubmitting(false);
    }
  };

  const nameErrorId = "register-name-error";
  const emailErrorId = "register-email-error";
  const passwordErrorId = "register-password-error";

  return (
    <form onSubmit={handleSubmit} className="space-y-5" noValidate>
      {formError && (
        <Alert variant="destructive">
          <AlertTitle>Unable to create your account</AlertTitle>
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}

      <FormField id="register-name" error={fieldErrors.name} required>
        <FormLabel>Full Name</FormLabel>
        <Input
          id="register-name"
          name="name"
          type="text"
          autoComplete="name"
          placeholder="Your full name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={isSubmitting}
          aria-invalid={Boolean(fieldErrors.name)}
          aria-describedby={fieldErrors.name ? nameErrorId : undefined}
        />
        <FormErrorMessage id={nameErrorId}>{fieldErrors.name}</FormErrorMessage>
      </FormField>

      <FormField id="register-email" error={fieldErrors.email} required>
        <FormLabel>Email Address</FormLabel>
        <Input
          id="register-email"
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

      <FormField id="register-password" error={fieldErrors.password} required>
        <FormLabel>Password</FormLabel>
        <Input
          id="register-password"
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder="Choose a password"
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
        {isSubmitting ? "Creating account…" : "Create Account"}
      </Button>
    </form>
  );
}
