"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/feedback/alert";
import { submitReviewAction } from "@/lib/actions/campaigns";
import type { CampaignDetail } from "@/lib/api/public/campaigns";

export interface CampaignOwnerActionsProps {
  campaign: CampaignDetail;
  isOwner: boolean;
  className?: string;
}

export function CampaignOwnerActions({
  campaign,
  isOwner,
  className,
}: CampaignOwnerActionsProps) {
  const router = useRouter();
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  if (!isOwner) {
    return null;
  }

  const { status, id } = campaign;
  const canEdit =
    status === "DRAFT" ||
    status === "PENDING_REVIEW" ||
    status === "ACTIVE" ||
    status === "REJECTED";

  const canSubmitReview = status === "DRAFT";

  const handleSubmitReview = async () => {
    if (!canSubmitReview || isSubmitting) return;

    setIsSubmitting(true);
    setErrorMessage(null);

    try {
      const result = await submitReviewAction(id);
      if (!result.ok) {
        switch (result.error.code) {
          case "INVALID_STATE_TRANSITION":
            setErrorMessage(
              "This campaign cannot be submitted for review from its current state."
            );
            break;
          case "FORBIDDEN":
            setErrorMessage(
              "You do not have permission to submit this campaign for review."
            );
            break;
          case "UNAUTHORIZED":
            setErrorMessage("Your session has expired. Please log in again.");
            break;
          default:
            setErrorMessage(
              result.error.message ||
                "Failed to submit campaign for review. Please try again."
            );
        }
      } else {
        router.refresh();
      }
    } catch {
      setErrorMessage("An unexpected network error occurred. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!canEdit && !canSubmitReview) {
    return null;
  }

  return (
    <div className={className}>
      {errorMessage && (
        <Alert variant="destructive" className="mb-3">
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      )}

      <div className="flex flex-wrap items-center gap-3">
        {canEdit && (
          <Button
            variant="outline"
            size="md"
            onClick={() => router.push(`/campaigns/${id}/edit`)}
          >
            Edit Campaign
          </Button>
        )}

        {canSubmitReview && (
          <Button
            variant="primary"
            size="md"
            isLoading={isSubmitting}
            onClick={handleSubmitReview}
          >
            Submit for Review
          </Button>
        )}
      </div>
    </div>
  );
}
