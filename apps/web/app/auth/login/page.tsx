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
import { LoginForm } from "@/components/auth/login-form";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

/**
 * Canonical login destination when no valid `from` is supplied.
 * Matches the existing authenticated landing page used by F3.5.
 */
const DEFAULT_REDIRECT = "/me";

export const metadata: Metadata = {
  title: "Sign In — CareFund",
  description:
    "Sign in to your CareFund account to donate, start a campaign, or track your contributions.",
};

interface LoginPageProps {
  searchParams: Promise<{ from?: string | string[] }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const params = await searchParams;

  // `from` may arrive repeated; only the first value is considered.
  const requested =
    Array.isArray(params?.from) ? params.from[0] : params?.from;

  // Sanitize BEFORE any use. Rejects absolute URLs, protocol-relative URLs,
  // scheme prefixes, backslash escapes, and control-character payloads.
  const redirectTo = sanitizeRedirectPath(requested, DEFAULT_REDIRECT);

  // Reuse the existing session layer. No second session mechanism is introduced.
  // SESSION_ERROR deliberately does NOT block the form: sign-in must remain
  // available when the session lookup hits a transient upstream failure.
  const session = await getSession();
  if (session.status === "AUTHENTICATED" && session.user) {
    redirect(redirectTo);
  }

  return (
    <main className="flex flex-1 items-center justify-center py-8 sm:py-12">
      <Container size="sm">
        <div className="mx-auto w-full max-w-md">
          <div className="mb-6 text-center">
            <p className="text-sm font-semibold uppercase tracking-wider text-primary">
              CareFund
            </p>
            <h1 className="mt-2 text-2xl font-bold tracking-tight text-text-primary sm:text-3xl">
              Sign in to your account
            </h1>
            <p className="mt-2 text-sm text-text-secondary">
              Access your donations, campaigns, and contribution history.
            </p>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Sign In</CardTitle>
              <CardDescription>
                Enter the email address and password associated with your
                CareFund account.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <LoginForm redirectTo={redirectTo} />
            </CardContent>
          </Card>

          {/*
            TODO (F3.7.2): registration entry point.

            The BFF route `POST /api/auth/register` already exists, but no
            registration PAGE exists yet. Per scope, no link is rendered here
            because a link to a non-existent route would 404. Add
            "Belum memiliki akun? Daftar" once `app/auth/register/page.tsx` lands.
          */}
        </div>
      </Container>
    </main>
  );
}
