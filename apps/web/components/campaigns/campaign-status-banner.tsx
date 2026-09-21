import * as React from "react";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import type { CampaignStatus } from "@/lib/api/public/campaigns";

export interface CampaignStatusBannerProps {
  status: CampaignStatus | string;
  rejectionReason?: string | null;
  className?: string;
}

export function CampaignStatusBanner({
  status,
  rejectionReason,
  className,
}: CampaignStatusBannerProps) {
  switch (status) {
    case "DRAFT":
      return (
        <Alert variant="warning" className={className}>
          <AlertTitle>Draft Campaign</AlertTitle>
          <AlertDescription>
            This campaign is currently in draft mode. You can edit campaign details. Submit it for review when you are ready for administrative evaluation.
          </AlertDescription>
        </Alert>
      );

    case "PENDING_REVIEW":
      return (
        <Alert variant="info" className={className}>
          <AlertTitle>Under Administrative Review</AlertTitle>
          <AlertDescription>
            This campaign has been submitted and is under review by CareFund administrators. Public donations will open once approved.
          </AlertDescription>
        </Alert>
      );

    case "REJECTED":
      return (
        <Alert variant="destructive" className={className}>
          <AlertTitle>Campaign Rejected</AlertTitle>
          <AlertDescription>
            {rejectionReason ? (
              <p className="mb-1 font-medium">
                Reason: {rejectionReason}
              </p>
            ) : null}
            <p>
              This campaign was rejected and cannot be resubmitted through the current API version.
            </p>
          </AlertDescription>
        </Alert>
      );

    case "SUSPENDED":
      return (
        <Alert variant="destructive" className={className}>
          <AlertTitle>Campaign Suspended</AlertTitle>
          <AlertDescription>
            This campaign has been temporarily suspended by an administrator. Donations and public interactions are paused.
          </AlertDescription>
        </Alert>
      );

    case "COMPLETED":
      return (
        <Alert variant="default" className={className}>
          <AlertTitle>Campaign Completed</AlertTitle>
          <AlertDescription>
            This campaign has reached its end or target and is now completed. It is preserved in read-only mode.
          </AlertDescription>
        </Alert>
      );

    case "CANCELLED":
      return (
        <Alert variant="default" className={className}>
          <AlertTitle>Campaign Cancelled</AlertTitle>
          <AlertDescription>
            This campaign was cancelled. It is preserved in read-only mode.
          </AlertDescription>
        </Alert>
      );

    case "ACTIVE":
    default:
      return null;
  }
}
