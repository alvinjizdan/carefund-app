import Link from "next/link";
import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { sanitizeRedirectPath } from "@/lib/auth/redirect";
import { Container } from "@/components/layout/container";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { RegisterForm } from "@/components/auth/register-form";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

/**
 * Post-registration destination when no valid `from` is supplied.
 * Mirrors the login page default.
 */
const DEFAULT_REDIRECT = "/me";

export const metadata: Metadata = {
  title: "Create Account — CareFund",
  description:
    "Create a CareFund account to donate, start a crowdfunding campaign, or track your contributions.",
};

interface RegisterPageProps {
  searchParams: Promise<{ from?: string | string[] }>;
}

export default async function RegisterPage({ searchParams }: RegisterPageProps) {
  const params = await searchParams;

  // `from` may arrive repeated; only the first value is considered.
  const requested = Array.isArray(params?.from) ? params.from[0] : params?.from;

  // Sanitize BEFORE any use. Rejects absolute URLs, protocol-relative URLs,
  // scheme prefixes, backslash escapes, and control-character payloads.
  const redirectTo = sanitizeRedirectPath(requested, DEFAULT_REDIRECT);

  // Reuse the existing session layer. No second session mechanism is introduced.
  // SESSION_ERROR does not block the form, so signup stays available during a
  // transient session-lookup failure.
  const session = await getSession();
  if (session.status === "AUTHENTICATED" && session.user) {
    redirect(redirectTo);
  }

  // Carry the sanitized destination through so a user who arrived from a protected
  // page returns there after signing in, without re-sanitizing at the destination.
  const signInHref = `/auth/login?from=${encodeURIComponent(redirectTo)}`;

  return (
    <main className="flex flex-1 items-center justify-center py-8 sm:py-12">
      <Container size="sm">
        <div className="mx-auto w-full max-w-md">
          <div className="mb-6 text-center">
            <p className="text-sm font-semibold uppercase tracking-wider text-primary">
              CareFund
            </p>
            <h1 className="mt-2 text-2xl font-bold tracking-tight text-text-primary sm:text-3xl">
              Create your account
            </h1>
            <p className="mt-2 text-sm text-text-secondary">
              Join CareFund to donate, start a campaign, or track your impact.
            </p>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Create Account</CardTitle>
              <CardDescription>
                All new accounts are registered with the DONOR role. Campaign and
                administrative privileges are granted by CareFund administrators.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <RegisterForm redirectTo={redirectTo} />
            </CardContent>
          </Card>

          <p className="mt-6 text-center text-sm text-text-secondary">
            Already have an account?{" "}
            <Link
              href={signInHref}
              className="font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card"
            >
              Sign in
            </Link>
          </p>
        </div>
      </Container>
    </main>
  );
}
