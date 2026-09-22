import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getUserDonations } from "@/lib/api/authenticated/donations";
import { getPublicCampaign } from "@/lib/api/public/campaigns";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import {
  DonationHistoryList,
  type ResolvedCampaignInfo,
} from "@/components/donations/donation-history-list";
import { CareFundApiError } from "@/lib/api/errors";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "My Donations — CareFund",
  description: "View and manage your donation history and transaction status.",
};

interface MeDonationsPageProps {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}

export default async function MeDonationsPage({ searchParams }: MeDonationsPageProps) {
  const session = await getSession();

  // 1. Authenticated check
  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent("/me/donations")}`);
  }

  // 2. Parse pagination query parameters
  const resolvedParams = await searchParams;
  const rawPage = resolvedParams.page;
  let page = 1;
  if (typeof rawPage === "string") {
    const parsed = parseInt(rawPage, 10);
    if (Number.isInteger(parsed) && parsed > 0) {
      page = parsed;
    }
  }

  const limit = 10;
  const offset = (page - 1) * limit;

  // 3. Fetch user donations
  let history;
  try {
    history = await getUserDonations({ limit, offset, cache: "no-store" });
  } catch (err: unknown) {
    if (err instanceof CareFundApiError && err.status === 401) {
      redirect(`/auth/login?from=${encodeURIComponent("/me/donations")}`);
    }

    return (
      <main className="py-8 sm:py-12">
        <Container size="lg">
          <PageHeader
            title="My Donations"
            description="View your past donations, track ongoing confirmations, and access transaction details."
          />
          <Alert variant="destructive">
            <AlertTitle>Failed to load donations</AlertTitle>
            <AlertDescription>
              Unable to load your donation history at this time. Please try refreshing the page.
            </AlertDescription>
          </Alert>
        </Container>
      </main>
    );
  }

  // 4. Best-effort parallel resolution of campaign metadata
  const uniqueCampaignIds = Array.from(
    new Set(history.items.map((item) => item.campaignId).filter(Boolean))
  );

  const campaignsMap: Record<string, ResolvedCampaignInfo | null> = {};
  await Promise.all(
    uniqueCampaignIds.map(async (cid) => {
      try {
        const campaign = await getPublicCampaign(cid);
        campaignsMap[cid] = {
          id: campaign.id,
          title: campaign.title,
        };
      } catch {
        campaignsMap[cid] = null;
      }
    })
  );

  return (
    <main className="py-8 sm:py-12">
      <Container size="lg">
        <PageHeader
          title="My Donations"
          description="View your past donations, track ongoing confirmations, and access transaction details."
        />
        <DonationHistoryList
          donations={history.items}
          campaignsMap={campaignsMap}
          currentPage={page}
          pageSize={limit}
          hasMore={history.hasMore}
        />
      </Container>
    </main>
  );
}
