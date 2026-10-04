import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getCategories } from "@/lib/api/public/categories";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { CampaignForm } from "@/components/campaigns/campaign-form";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Create a Campaign — CareFund",
  description: "Start a new transparent fundraising campaign on CareFund. Your campaign is saved as a draft before it is submitted for review.",
};

export default async function NewCampaignPage() {
  const session = await getSession();

  // 1. Unauthenticated: redirect to the existing login route with a return path.
  //     The login flow then completes via the backend and restores the session.
  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent("/campaigns/new")}`);
  }

  // 2. Public category taxonomy. If unavailable, the form still renders but the
  //    user is shown a safe, user-facing error and cannot submit.
  let categories: Awaited<ReturnType<typeof getCategories>> = [];
  let categoriesError = false;

  try {
    categories = await getCategories();
  } catch {
    categoriesError = true;
  }

  return (
    <main className="py-8 sm:py-12">
      <Container size="lg">
        <PageHeader
          title="Create a New Campaign"
          description="Fill in the details below to start a transparent fundraising campaign. Once saved, it remains a draft until you submit it for review."
        />

        <div className="mt-6 space-y-6">
          {/* Category fetch failure: safe, user-facing error. */}
          {categoriesError && (
            <Alert variant="destructive">
              <AlertTitle>Categories unavailable</AlertTitle>
              <AlertDescription>
                We could not load campaign categories at this time. Please try
                refreshing the page. You can still write your campaign text, but
                you will need to select a category before submitting.
              </AlertDescription>
            </Alert>
          )}

          <CampaignForm mode="create" categories={categories} />
        </div>
      </Container>
    </main>
  );
}
