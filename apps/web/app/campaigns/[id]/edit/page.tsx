import Link from "next/link";
import { redirect, notFound } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getCampaignForViewer } from "@/lib/api/public/campaigns";
import { getCategories } from "@/lib/api/public/categories";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { Button } from "@/components/ui/button";
import { CampaignForm } from "@/components/campaigns/campaign-form";

export const dynamic = "force-dynamic";

interface EditCampaignPageProps {
  params: Promise<{ id: string }>;
}

export const metadata = {
  title: "Edit Campaign — CareFund",
  description: "Update campaign details and story.",
};

export default async function EditCampaignPage({ params }: EditCampaignPageProps) {
  const { id } = await params;
  const session = await getSession();

  // 1. Authenticated check
  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent(`/campaigns/${id}/edit`)}`);
  }

  // 2. Fetch campaign with viewer context
  let campaign;
  try {
    campaign = await getCampaignForViewer(id);
  } catch {
    notFound();
  }

  if (!campaign) {
    notFound();
  }

  // 3. Strict Owner Authorization Check
  if (session.user.id !== campaign.ownerId) {
    return (
      <main className="py-8 sm:py-12">
        <Container size="md">
          <Alert variant="destructive">
            <AlertTitle>403 Forbidden</AlertTitle>
            <AlertDescription>
              You do not have permission to edit this campaign. Only the campaign creator can modify campaign details.
            </AlertDescription>
          </Alert>
          <div className="mt-4">
            <Link
              href={`/campaigns/${id}`}
              className="inline-flex items-center justify-center h-10 px-4 text-sm font-medium rounded-lg border border-border-strong bg-surface-card hover:bg-surface-hover active:bg-surface-active text-text-primary transition-colors"
            >
              Return to Campaign
            </Link>
          </div>
        </Container>
      </main>
    );
  }

  // 4. Terminal State Gating
  if (campaign.status === "COMPLETED" || campaign.status === "CANCELLED") {
    return (
      <main className="py-8 sm:py-12">
        <Container size="md">
          <Alert variant="warning">
            <AlertTitle>Campaign Is Read-Only</AlertTitle>
            <AlertDescription>
              This campaign is currently in {campaign.status} status and cannot be edited.
            </AlertDescription>
          </Alert>
          <div className="mt-4">
            <Link
              href={`/campaigns/${id}`}
              className="inline-flex items-center justify-center h-10 px-4 text-sm font-medium rounded-lg border border-border-strong bg-surface-card hover:bg-surface-hover active:bg-surface-active text-text-primary transition-colors"
            >
              Return to Campaign
            </Link>
          </div>
        </Container>
      </main>
    );
  }

  // 5. Load categories
  const categories = await getCategories();

  return (
    <main className="py-8 sm:py-12">
      <Container size="lg">
        <PageHeader
          title="Edit Campaign"
          description={`Updating details for "${campaign.title}".`}
          action={
            <Link
              href={`/campaigns/${id}`}
              className="inline-flex items-center justify-center h-8 px-3 text-xs font-medium rounded-md border border-border-strong bg-surface-card hover:bg-surface-hover active:bg-surface-active text-text-primary transition-colors"
            >
              View Campaign
            </Link>
          }
        />

        <div className="mt-6 rounded-xl border border-border-strong bg-surface-card p-6 sm:p-8 shadow-sm">
          <CampaignForm
            mode="edit"
            categories={categories}
            initialData={campaign}
            isRejected={campaign.status === "REJECTED"}
          />
        </div>
      </Container>
    </main>
  );
}
