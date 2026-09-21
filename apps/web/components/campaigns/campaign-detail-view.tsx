"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { CampaignStatusBadge } from "@/components/campaigns/campaign-status-badge";
import { CampaignStatusBanner } from "@/components/campaigns/campaign-status-banner";
import { CampaignOwnerActions } from "@/components/campaigns/campaign-owner-actions";
import { CampaignAdminActions } from "@/components/campaigns/campaign-admin-actions";
import { DonationCheckoutModal } from "@/components/donations/donation-checkout-modal";
import { formatIDR } from "@/lib/donations/validation";
import type { CampaignDetail } from "@/lib/api/public/campaigns";
import type { User } from "@/lib/auth/types";

export interface CampaignDetailViewProps {
  campaign: CampaignDetail;
  categoryName?: string;
  currentUser: User | null;
}

export function CampaignDetailView({
  campaign,
  categoryName,
  currentUser,
}: CampaignDetailViewProps) {
  const router = useRouter();
  const [isDonationModalOpen, setIsDonationModalOpen] = React.useState(false);

  const isOwner = Boolean(currentUser && currentUser.id === campaign.ownerId);
  const isAdmin = Boolean(currentUser?.roles?.includes("ADMIN"));
  const isActive = campaign.status === "ACTIVE";

  // Financial progress percentage calculation (capped visually at 100% for progress bar)
  const percentFunded =
    campaign.targetAmount > 0
      ? Math.min(
          100,
          Math.floor((campaign.currentAmount / campaign.targetAmount) * 100)
        )
      : 0;

  const actualPercent =
    campaign.targetAmount > 0
      ? ((campaign.currentAmount / campaign.targetAmount) * 100).toFixed(1)
      : "0";

  // Format dates for presentation
  const formatDateDisplay = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString("id-ID", {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
    } catch {
      return isoString;
    }
  };

  return (
    <div className="space-y-8">
      {/* Campaign Status Banner */}
      <CampaignStatusBanner
        status={campaign.status}
        rejectionReason={campaign.rejectionReason}
      />

      {/* Admin Moderation Bar (Admin only) */}
      <CampaignAdminActions campaign={campaign} isAdmin={isAdmin} />

      {/* Header Section */}
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2 text-xs font-medium text-text-muted">
          {categoryName && (
            <span className="rounded-md bg-surface-muted px-2.5 py-1 text-text-secondary">
              {categoryName}
            </span>
          )}
          <CampaignStatusBadge status={campaign.status} size="sm" />
          <span>•</span>
          <span>Created on {formatDateDisplay(campaign.createdAt)}</span>
        </div>

        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <h1 className="text-2xl font-bold tracking-tight text-text-primary sm:text-3xl lg:text-4xl max-w-3xl">
            {campaign.title}
          </h1>

          {/* Owner Actions */}
          <CampaignOwnerActions campaign={campaign} isOwner={isOwner} />
        </div>
      </div>

      {/* Main Grid: Overview & Donation */}
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Story / Description (2 cols on large screen) */}
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-xl">About this campaign</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="prose prose-sm dark:prose-invert max-w-none text-text-secondary leading-relaxed whitespace-pre-line">
                {campaign.description}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Campaign Details</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 text-sm">
                <div>
                  <dt className="text-xs text-text-muted font-medium">Scheduled Start</dt>
                  <dd className="mt-1 text-text-primary font-semibold">
                    {formatDateDisplay(campaign.startAt)}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-text-muted font-medium">Scheduled End</dt>
                  <dd className="mt-1 text-text-primary font-semibold">
                    {formatDateDisplay(campaign.endAt)}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-text-muted font-medium">Platform Category</dt>
                  <dd className="mt-1 text-text-primary">
                    {categoryName || campaign.categoryId}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-text-muted font-medium">Campaign Slug</dt>
                  <dd className="mt-1 font-mono text-xs text-text-secondary truncate">
                    {campaign.slug}
                  </dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>

        {/* Financial Progress & Donation Card (1 col on large screen) */}
        <div className="space-y-6">
          <Card className="sticky top-6">
            <CardHeader>
              <div className="space-y-1">
                <span className="text-xs uppercase tracking-wider text-text-muted font-medium">
                  Fundraising Progress
                </span>
                <div className="text-2xl font-bold text-text-primary">
                  {formatIDR(campaign.currentAmount)}
                </div>
                <div className="text-xs text-text-secondary">
                  raised of {formatIDR(campaign.targetAmount)} target ({actualPercent}%)
                </div>
              </div>
            </CardHeader>

            <CardContent className="space-y-6">
              {/* Progress Bar */}
              <div
                className="w-full h-3 rounded-full bg-surface-muted overflow-hidden"
                role="progressbar"
                aria-valuenow={percentFunded}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-label={`Fundraising progress: ${actualPercent}%`}
              >
                <div
                  className="h-full bg-primary transition-all duration-500 rounded-full"
                  style={{ width: `${percentFunded}%` }}
                />
              </div>

              {/* Donation Action */}
              {isActive ? (
                <div className="space-y-2">
                  <Button
                    variant="primary"
                    size="lg"
                    className="w-full"
                    onClick={() => setIsDonationModalOpen(true)}
                  >
                    Donate Now
                  </Button>
                  <p className="text-center text-xs text-text-muted">
                    Secure checkout powered by Midtrans Payment Gateway
                  </p>
                </div>
              ) : (
                <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 text-center">
                  <p className="text-xs text-text-muted">
                    Donations are only accepted when the campaign is active.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Donation Checkout Modal (active when donation button clicked) */}
      {isActive && (
        <DonationCheckoutModal
          campaignId={campaign.id}
          campaignTitle={campaign.title}
          isOpen={isDonationModalOpen}
          onOpenChange={setIsDonationModalOpen}
          onDonationSuccess={() => {
            router.refresh();
          }}
        />
      )}
    </div>
  );
}
