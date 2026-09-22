import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getDonationDetail } from "@/lib/api/authenticated/donations";
import { getPublicCampaign } from "@/lib/api/public/campaigns";
import { Container } from "@/components/layout/container";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { DonationDetailView } from "@/components/donations/donation-detail-view";
import { CareFundApiError } from "@/lib/api/errors";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

interface DonationDetailPageProps {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({
  params,
}: DonationDetailPageProps): Promise<Metadata> {
  const { id } = await params;
  return {
    title: `Donation Details — CareFund`,
    description: `View transaction details and confirmation status for donation ${id}.`,
  };
}

export default async function DonationDetailPage({
  params,
}: DonationDetailPageProps) {
  const { id } = await params;
  const session = await getSession();

  // 1. Authenticated check
  if (session.status !== "AUTHENTICATED" || !session.user) {
    redirect(`/auth/login?from=${encodeURIComponent(`/donations/${id}`)}`);
  }

  // 2. Fetch donation detail
  let donation;
  try {
    donation = await getDonationDetail(id, { cache: "no-store" });
  } catch (err: unknown) {
    if (err instanceof CareFundApiError && err.status === 403) {
      return (
        <main className="py-8 sm:py-12">
          <Container size="md">
            <Alert variant="destructive">
              <AlertTitle>403 Forbidden</AlertTitle>
              <AlertDescription>
                You do not have permission to access this donation. Only the donor, the associated campaign organizer, or an administrator may view this record.
              </AlertDescription>
            </Alert>
            <div className="mt-4">
              <Link
                href="/me/donations"
                className="inline-flex items-center justify-center h-10 px-4 text-sm font-medium rounded-lg border border-border-strong bg-surface-card hover:bg-surface-hover active:bg-surface-active text-text-primary transition-colors"
              >
                Return to My Donations
              </Link>
            </div>
          </Container>
        </main>
      );
    }

    notFound();
  }

  if (!donation) {
    notFound();
  }

  // 3. Best-effort historical campaign resolution
  let campaign = null;
  if (donation.campaignId) {
    try {
      campaign = await getPublicCampaign(donation.campaignId);
    } catch {
      // Historical campaigns in COMPLETED, SUSPENDED, REJECTED, or CANCELLED status
      // return 404 from getPublicCampaign(). Fallback gracefully.
      campaign = null;
    }
  }

  return (
    <main className="py-8 sm:py-12">
      <Container size="md">
        <DonationDetailView donation={donation} campaign={campaign} />
      </Container>
    </main>
  );
}
