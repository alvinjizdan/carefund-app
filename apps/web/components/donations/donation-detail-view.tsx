"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { DonationStatusBadge } from "./donation-status-badge";
import { formatIDR } from "@/lib/donations/validation";
import type { DonationDetail } from "@/lib/api/authenticated/types";
import type { PublicCampaign } from "@/lib/api/public/campaigns";

export interface DonationDetailViewProps {
  donation: DonationDetail;
  campaign: PublicCampaign | null;
}

function formatDate(isoString: string): string {
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) {
      return isoString;
    }
    return d.toLocaleDateString("en-US", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return isoString;
  }
}

export function DonationDetailView({
  donation,
  campaign,
}: DonationDetailViewProps) {
  const router = useRouter();
  const [isRefreshing, startTransition] = React.useTransition();

  const handleRefresh = () => {
    startTransition(() => {
      router.refresh();
    });
  };

  const isPending = donation.status === "PENDING";

  return (
    <div className="space-y-6">
      {/* Back Link */}
      <div>
        <Link
          href="/me/donations"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-text-secondary hover:text-text-primary transition-colors"
        >
          <svg
            className="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
            aria-hidden="true"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
          </svg>
          Back to My Donations
        </Link>
      </div>

      {/* Pending Status Alert with Single Manual Refresh Action */}
      {isPending && (
        <Alert variant="warning">
          <AlertTitle>Awaiting Payment Confirmation</AlertTitle>
          <AlertDescription>
            <p className="mt-1">
              This donation is currently awaiting confirmation from the payment gateway. If you have already completed payment via Midtrans, click below to check for an updated status.
            </p>
            <div className="mt-3">
              <Button
                variant="outline"
                size="sm"
                onClick={handleRefresh}
                isLoading={isRefreshing}
              >
                Refresh Status
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      {/* Main Donation Overview Card */}
      <Card>
        <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border-subtle pb-6">
          <div className="space-y-1">
            <span className="text-xs font-mono uppercase tracking-wider text-text-muted">
              Donation Summary
            </span>
            <div className="text-2xl sm:text-3xl font-bold text-text-primary">
              {formatIDR(donation.amount)}
            </div>
          </div>
          <div className="shrink-0">
            <DonationStatusBadge status={donation.status} size="md" />
          </div>
        </CardHeader>

        <CardContent className="pt-6 space-y-6">
          {/* Associated Campaign Section */}
          <div className="rounded-lg border border-border-subtle p-4 bg-surface-muted/30 space-y-3">
            <div className="text-xs font-semibold uppercase tracking-wider text-text-secondary">
              Associated Campaign
            </div>
            {campaign ? (
              <div className="space-y-2">
                <Link
                  href={`/campaigns/${campaign.id}`}
                  className="text-lg font-semibold text-text-primary hover:text-primary transition-colors block"
                >
                  {campaign.title}
                </Link>
                <p className="text-sm text-text-secondary line-clamp-2">
                  {campaign.description}
                </p>
                <div className="pt-2">
                  <Link href={`/campaigns/${campaign.id}`}>
                    <Button variant="outline" size="sm">
                      View Campaign
                    </Button>
                  </Link>
                </div>
              </div>
            ) : (
              <div className="space-y-1">
                <div className="text-base font-medium text-text-secondary italic">
                  Completed or Archived Campaign
                </div>
                <p className="text-xs text-text-muted">
                  This campaign is either completed, suspended, or not publicly accessible.
                </p>
                <span className="block text-xs font-mono text-text-muted pt-1">
                  Campaign ID: {donation.campaignId}
                </span>
              </div>
            )}
          </div>

          {/* Donor Message */}
          {donation.message && (
            <div className="space-y-2">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-secondary">
                Personal Message
              </div>
              <blockquote className="rounded-lg border-l-4 border-primary/60 bg-surface-muted/40 p-3 text-sm italic text-text-primary">
                &ldquo;{donation.message}&rdquo;
              </blockquote>
            </div>
          )}

          {/* Privacy & Anonymity Preference */}
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-wider text-text-secondary">
              Privacy Preference
            </div>
            <div className="flex items-center gap-2 text-sm text-text-secondary">
              {donation.isAnonymous ? (
                <>
                  <svg
                    className="w-4 h-4 text-warning"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                    aria-hidden="true"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
                    />
                  </svg>
                  <span>
                    <strong className="font-semibold text-text-primary">Anonymous Donation:</strong> You chose to hide your identity on the public campaign page.
                  </span>
                </>
              ) : (
                <>
                  <svg
                    className="w-4 h-4 text-success"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                    aria-hidden="true"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z"
                    />
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                    />
                  </svg>
                  <span>
                    <strong className="font-semibold text-text-primary">Public Donation:</strong> Your donation appears publicly under your name.
                  </span>
                </>
              )}
            </div>
          </div>

          {/* Technical Transaction Details */}
          <div className="space-y-3 pt-4 border-t border-border-subtle">
            <div className="text-xs font-semibold uppercase tracking-wider text-text-secondary">
              Transaction Details
            </div>
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
              <div className="rounded-md bg-surface-muted/50 p-3">
                <dt className="text-xs text-text-secondary font-medium">Donation Reference ID</dt>
                <dd className="font-mono text-xs text-text-primary break-all mt-1 select-all">
                  {donation.id}
                </dd>
              </div>

              <div className="rounded-md bg-surface-muted/50 p-3">
                <dt className="text-xs text-text-secondary font-medium">Created At</dt>
                <dd className="text-xs text-text-primary mt-1">
                  {formatDate(donation.createdAt)}
                </dd>
              </div>

              <div className="rounded-md bg-surface-muted/50 p-3">
                <dt className="text-xs text-text-secondary font-medium">Last Updated</dt>
                <dd className="text-xs text-text-primary mt-1">
                  {formatDate(donation.updatedAt)}
                </dd>
              </div>

              <div className="rounded-md bg-surface-muted/50 p-3">
                <dt className="text-xs text-text-secondary font-medium">Campaign Reference ID</dt>
                <dd className="font-mono text-xs text-text-primary break-all mt-1 select-all">
                  {donation.campaignId}
                </dd>
              </div>
            </dl>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
