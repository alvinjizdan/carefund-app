import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getCategories } from "@/lib/api/public/categories";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import { CampaignForm } from "@/components/campaigns/campaign-form";

export const dynamic = "force-dynamic";

export const metadata = {
  title: "Create Campaign — CareFund",
  description: "Start a new verified fundraising campaign on CareFund.",
};

export default async function NewCampaignPage() {
  const session = await getSession();

  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent("/campaigns/new")}`);
  }

  const categories = await getCategories();

  return (
    <main className="py-8 sm:py-12">
      <Container size="lg">
        <PageHeader
          title="Create a New Campaign"
          description="Fill out the details below to launch your transparent fundraising initiative. Once submitted, it will be saved as a draft for your review."
        />

        <div className="mt-6 rounded-xl border border-border-strong bg-surface-card p-6 sm:p-8 shadow-sm">
          <CampaignForm mode="create" categories={categories} />
        </div>
      </Container>
    </main>
  );
}
