import Link from "next/link";
import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Account Overview — CareFund",
  description: "View your CareFund account profile and navigation hub.",
};

function getRoleBadgeVariant(
  role: string
): "destructive" | "info" | "secondary" {
  switch (role.toUpperCase()) {
    case "ADMIN":
      return "destructive";
    case "CREATOR":
      return "info";
    default:
      return "secondary";
  }
}

export default async function MePage() {
  const session = await getSession();

  // 1. Session error handling (5xx / network failure)
  if (session.status === "SESSION_ERROR") {
    return (
      <main className="py-8 sm:py-12">
        <Container size="lg">
          <PageHeader
            title="Account Overview"
            description="Manage your profile information and access account features."
          />
          <Alert variant="destructive">
            <AlertTitle>Service Unavailable</AlertTitle>
            <AlertDescription>
              Unable to load your account information at this time. Please try
              refreshing the page.
            </AlertDescription>
          </Alert>
        </Container>
      </main>
    );
  }

  // 2. Unauthenticated redirect with return path
  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent("/me")}`);
  }

  const user = session.user;

  return (
    <main className="py-8 sm:py-12">
      <Container size="lg">
        <PageHeader
          title="Account Overview"
          description="Manage your profile information and access account features."
        />

        <div className="space-y-8">
          {/* Read-Only Profile Details Card */}
          <Card>
            <CardHeader>
              <CardTitle className="text-xl">Profile Details</CardTitle>
              <CardDescription>
                Your verified CareFund identity and platform roles.
              </CardDescription>
            </CardHeader>

            <CardContent>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-6 text-sm">
                <div>
                  <dt className="text-xs font-medium text-text-muted">
                    Full Name
                  </dt>
                  <dd className="mt-1 text-base font-medium text-text-primary">
                    {user.name || "—"}
                  </dd>
                </div>

                <div>
                  <dt className="text-xs font-medium text-text-muted">
                    Email Address
                  </dt>
                  <dd className="mt-1 text-base font-medium text-text-primary">
                    {user.email}
                  </dd>
                </div>

                <div>
                  <dt className="text-xs font-medium text-text-muted">
                    User ID
                  </dt>
                  <dd className="mt-1 font-mono text-xs text-text-secondary truncate">
                    {user.id}
                  </dd>
                </div>

                <div>
                  <dt className="text-xs font-medium text-text-muted">
                    Platform Roles
                  </dt>
                  <dd className="mt-1.5 flex flex-wrap gap-1.5">
                    {user.roles && user.roles.length > 0 ? (
                      user.roles.map((role) => (
                        <Badge
                          key={role}
                          variant={getRoleBadgeVariant(role)}
                          size="sm"
                        >
                          {role}
                        </Badge>
                      ))
                    ) : (
                      <Badge variant="secondary" size="sm">
                        USER
                      </Badge>
                    )}
                  </dd>
                </div>
              </dl>
            </CardContent>

            <CardFooter className="pt-3 border-t border-border-subtle text-xs text-text-muted">
              Role permissions and account privileges are governed
              authoritatively by CareFund backend services.
            </CardFooter>
          </Card>

          {/* Navigation Hubs */}
          <div className="space-y-4">
            <h2 className="text-lg font-semibold tracking-tight text-text-primary">
              Account Hub
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {/* Hub 1: Donation History */}
              <Card className="flex flex-col justify-between">
                <CardHeader className="space-y-2">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-muted text-primary">
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={1.5}
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
                      />
                    </svg>
                  </div>
                  <CardTitle className="text-base">Donation History</CardTitle>
                  <CardDescription className="text-xs leading-relaxed">
                    View your past donations, track ongoing confirmations, and
                    access transaction details.
                  </CardDescription>
                </CardHeader>
                <CardFooter className="pt-0">
                  <Link href="/me/donations" className="w-full">
                    <Button variant="outline" size="sm" className="w-full">
                      View Donations
                    </Button>
                  </Link>
                </CardFooter>
              </Card>

              {/* Hub 2: Start a Campaign */}
              <Card className="flex flex-col justify-between">
                <CardHeader className="space-y-2">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-muted text-primary">
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={1.5}
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M12 4.5v15m7.5-7.5h-15"
                      />
                    </svg>
                  </div>
                  <CardTitle className="text-base">Start a Campaign</CardTitle>
                  <CardDescription className="text-xs leading-relaxed">
                    Create a new transparent medical or social crowdfunding
                    campaign to seek community funding.
                  </CardDescription>
                </CardHeader>
                <CardFooter className="pt-0">
                  <Link href="/campaigns/new" className="w-full">
                    <Button variant="outline" size="sm" className="w-full">
                      Create Campaign
                    </Button>
                  </Link>
                </CardFooter>
              </Card>

              {/* Hub 3: Explore Campaigns */}
              <Card className="flex flex-col justify-between">
                <CardHeader className="space-y-2">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-muted text-primary">
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={1.5}
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
                      />
                    </svg>
                  </div>
                  <CardTitle className="text-base">Explore Campaigns</CardTitle>
                  <CardDescription className="text-xs leading-relaxed">
                    Discover and support active verified crowdfunding initiatives
                    across various cause categories.
                  </CardDescription>
                </CardHeader>
                <CardFooter className="pt-0">
                  <Link href="/explore" className="w-full">
                    <Button variant="outline" size="sm" className="w-full">
                      Browse Campaigns
                    </Button>
                  </Link>
                </CardFooter>
              </Card>
            </div>
          </div>
        </div>
      </Container>
    </main>
  );
}
