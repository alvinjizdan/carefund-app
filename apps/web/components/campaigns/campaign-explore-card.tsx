import * as React from "react";
import Link from "next/link";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatIDR } from "@/lib/donations/validation";
import type { PublicCampaign } from "@/lib/api/public/campaigns";

export interface CampaignExploreCardProps {
  campaign: PublicCampaign;
  categoryName?: string;
}

export function CampaignExploreCard({
  campaign,
  categoryName,
}: CampaignExploreCardProps) {
  // Integer-safe visual percentage calculation clamped to 0..100%
  const percentFunded =
    campaign.targetAmount > 0
      ? Math.min(
          100,
          Math.max(
            0,
            Math.floor((campaign.currentAmount / campaign.targetAmount) * 100)
          )
        )
      : 0;

  const displayPercent =
    campaign.targetAmount > 0
      ? ((campaign.currentAmount / campaign.targetAmount) * 100).toFixed(1)
      : "0";

  const formatDateDisplay = (isoString: string) => {
    try {
      const d = new Date(isoString);
      if (isNaN(d.getTime())) return isoString;
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
    <Card className="flex flex-col h-full overflow-hidden transition-all duration-200 hover:shadow-md hover:border-border-strong">
      {/* Design System Cover Placeholder */}
      <div
        className="relative h-44 w-full bg-gradient-to-br from-surface-muted to-surface-card border-b border-border-subtle flex flex-col items-center justify-center p-4 text-center select-none"
        aria-hidden="true"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-surface-card shadow-sm border border-border-subtle text-primary mb-2">
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
        </div>
        <span className="text-xs font-medium text-text-muted">
          CareFund Verified Campaign
        </span>
      </div>

      <CardHeader className="p-5 pb-3 space-y-2">
        <div className="flex items-center justify-between gap-2">
          {categoryName ? (
            <Badge variant="secondary" size="sm">
              {categoryName}
            </Badge>
          ) : (
            <Badge variant="outline" size="sm">
              General
            </Badge>
          )}
          <Badge variant="success" size="sm">
            Active
          </Badge>
        </div>

        <CardTitle className="text-lg leading-snug line-clamp-2">
          <Link
            href={`/campaigns/${campaign.id}`}
            className="hover:text-primary transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus rounded"
          >
            {campaign.title}
          </Link>
        </CardTitle>
      </CardHeader>

      <CardContent className="p-5 pt-0 flex-1 flex flex-col justify-between space-y-4">
        <p className="text-sm text-text-secondary line-clamp-2 leading-relaxed">
          {campaign.description}
        </p>

        <div className="space-y-2 pt-2 border-t border-border-subtle">
          <div className="flex items-baseline justify-between text-sm">
            <span className="font-semibold text-text-primary">
              {formatIDR(campaign.currentAmount)}
            </span>
            <span className="text-xs text-text-muted">
              target {formatIDR(campaign.targetAmount)}
            </span>
          </div>

          {/* Accessible progress bar */}
          <div
            className="w-full h-2 rounded-full bg-surface-muted overflow-hidden"
            role="progressbar"
            aria-valuenow={percentFunded}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Fundraising progress: ${displayPercent}% of ${formatIDR(campaign.targetAmount)}`}
          >
            <div
              className="h-full bg-primary transition-all duration-300 rounded-full"
              style={{ width: `${percentFunded}%` }}
            />
          </div>

          <div className="flex items-center justify-between text-xs text-text-muted pt-1">
            <span>{displayPercent}% funded</span>
            {campaign.endAt ? (
              <span>Ends {formatDateDisplay(campaign.endAt)}</span>
            ) : null}
          </div>
        </div>
      </CardContent>

      <CardFooter className="p-5 pt-0">
        <Link href={`/campaigns/${campaign.id}`} className="w-full">
          <Button variant="outline" size="sm" className="w-full">
            View Campaign
          </Button>
        </Link>
      </CardFooter>
    </Card>
  );
}
