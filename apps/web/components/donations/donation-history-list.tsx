import * as React from "react";
import Link from "next/link";
import { Card, CardHeader, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/feedback/empty-state";
import { DonationStatusBadge } from "./donation-status-badge";
import { formatIDR } from "@/lib/donations/validation";
import type { DonationDetail } from "@/lib/api/authenticated/types";

export interface ResolvedCampaignInfo {
  id: string;
  title: string;
}

export interface DonationHistoryListProps {
  donations: DonationDetail[];
  campaignsMap: Record<string, ResolvedCampaignInfo | null>;
  currentPage: number;
  pageSize: number;
  hasMore: boolean;
}

function formatDate(isoString: string): string {
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) {
      return isoString;
    }
    return d.toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return isoString;
  }
}

export function DonationHistoryList({
  donations,
  campaignsMap,
  currentPage,
  pageSize,
  hasMore,
}: DonationHistoryListProps) {
  if (donations.length === 0) {
    return (
      <EmptyState
        title="No donations yet"
        description="You haven't made any donations yet. Support a verified cause you care about today."
        icon={
          <svg
            className="w-6 h-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M21 8.25c0-2.485-2.099-4.5-4.688-4.5-1.935 0-3.597 1.126-4.312 2.733-.715-1.607-2.377-2.733-4.313-2.733C5.1 3.75 3 5.765 3 8.25c0 7.22 9 12 9 12s9-4.78 9-12z"
            />
          </svg>
        }
        action={
          <Link href="/">
            <Button variant="primary">Explore Campaigns</Button>
          </Link>
        }
      />
    );
  }

  const startRecord = (currentPage - 1) * pageSize + 1;
  const endRecord = startRecord + donations.length - 1;

  return (
    <div className="space-y-6">
      <div className="space-y-4">
        {donations.map((donation) => {
          const campaign = campaignsMap[donation.campaignId];

          return (
            <Card key={donation.id} className="transition-shadow hover:shadow-md">
              <CardHeader className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 pb-3">
                <div className="space-y-1">
                  {campaign ? (
                    <Link
                      href={`/campaigns/${donation.campaignId}`}
                      className="text-base sm:text-lg font-semibold text-text-primary hover:text-primary transition-colors line-clamp-1"
                    >
                      {campaign.title}
                    </Link>
                  ) : (
                    <div>
                      <span className="text-base sm:text-lg font-medium text-text-secondary italic">
                        Completed or Archived Campaign
                      </span>
                      <span className="block text-xs font-mono text-text-muted mt-0.5">
                        ID: {donation.campaignId}
                      </span>
                    </div>
                  )}
                  <p className="text-xs text-text-secondary">
                    {formatDate(donation.createdAt)}
                  </p>
                </div>
                <div className="shrink-0">
                  <DonationStatusBadge status={donation.status} size="sm" />
                </div>
              </CardHeader>

              <CardContent className="py-2">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <div>
                    <span className="text-xs text-text-secondary block">Amount</span>
                    <span className="text-xl sm:text-2xl font-bold text-text-primary">
                      {formatIDR(donation.amount)}
                    </span>
                  </div>

                  <div>
                    {donation.isAnonymous ? (
                      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-text-secondary bg-surface-muted px-2.5 py-1 rounded-full">
                        <svg
                          className="w-3.5 h-3.5 text-text-muted"
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
                        Anonymous Donation
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-text-secondary bg-surface-muted px-2.5 py-1 rounded-full">
                        Public Donation
                      </span>
                    )}
                  </div>
                </div>

                {donation.message && (
                  <div className="mt-3 rounded-lg bg-surface-muted/60 p-3 text-sm text-text-secondary italic border border-border-subtle/50">
                    &ldquo;{donation.message}&rdquo;
                  </div>
                )}
              </CardContent>

              <CardFooter className="pt-3 border-t border-border-subtle flex items-center justify-between">
                <span className="text-xs font-mono text-text-muted">
                  Ref: {donation.id.slice(0, 8)}...
                </span>
                <Link href={`/donations/${donation.id}`}>
                  <Button variant="outline" size="sm">
                    View Details
                  </Button>
                </Link>
              </CardFooter>
            </Card>
          );
        })}
      </div>

      {/* Pagination Controls */}
      <div className="flex flex-col sm:flex-row items-center justify-between gap-4 pt-2">
        <p className="text-xs sm:text-sm text-text-secondary">
          Showing <span className="font-medium text-text-primary">{startRecord}</span> to{" "}
          <span className="font-medium text-text-primary">{endRecord}</span>
        </p>

        <div className="flex items-center gap-2">
          {currentPage > 1 ? (
            <Link href={`/me/donations?page=${currentPage - 1}`}>
              <Button variant="outline" size="sm">
                Previous
              </Button>
            </Link>
          ) : (
            <Button variant="outline" size="sm" disabled>
              Previous
            </Button>
          )}

          <span className="text-xs sm:text-sm font-medium text-text-secondary px-2">
            Page {currentPage}
          </span>

          {hasMore ? (
            <Link href={`/me/donations?page=${currentPage + 1}`}>
              <Button variant="outline" size="sm">
                Next
              </Button>
            </Link>
          ) : (
            <Button variant="outline" size="sm" disabled>
              Next
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
