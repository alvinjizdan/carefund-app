import { notFound } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getCampaignForViewer } from "@/lib/api/public/campaigns";
import { getCategories } from "@/lib/api/public/categories";
import { Container } from "@/components/layout/container";
import { CampaignDetailView } from "@/components/campaigns/campaign-detail-view";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

interface CampaignPageProps {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({
  params,
}: CampaignPageProps): Promise<Metadata> {
  const { id } = await params;
  try {
    const campaign = await getCampaignForViewer(id);
    return {
      title: `${campaign.title} — CareFund`,
      description: campaign.description.slice(0, 160),
    };
  } catch {
    return {
      title: "Campaign Details — CareFund",
    };
  }
}

export default async function CampaignDetailPage({ params }: CampaignPageProps) {
  const { id } = await params;
  const session = await getSession();

  let campaign;
  try {
    // getCampaignForViewer forwards authenticated session cookies server-side
    // allowing creator/admin access to private lifecycle states and rejectionReason
    campaign = await getCampaignForViewer(id);
  } catch (err: unknown) {
    // Non-active campaign viewed by unauthorized user returns 404 (NOT_FOUND)
    notFound();
  }

  if (!campaign) {
    notFound();
  }

  // Resolve category name from public categories
  let categoryName: string | undefined;
  try {
    const categories = await getCategories();
    const matched = categories.find((cat) => cat.id === campaign.categoryId);
    if (matched) {
      categoryName = matched.name;
    }
  } catch {
    // Fallback gracefully if categories fail to load
  }

  return (
    <main className="py-8 sm:py-12">
      <Container size="2xl">
        <CampaignDetailView
          campaign={campaign}
          categoryName={categoryName}
          currentUser={session.user}
        />
      </Container>
    </main>
  );
}
