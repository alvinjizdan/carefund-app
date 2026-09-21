"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/forms/textarea";
import { Label } from "@/components/forms/label";
import { Alert, AlertDescription } from "@/components/feedback/alert";
import {
  approveCampaignAction,
  rejectCampaignAction,
  suspendCampaignAction,
} from "@/lib/actions/campaigns";
import type { CampaignDetail } from "@/lib/api/public/campaigns";

export interface CampaignAdminActionsProps {
  campaign: CampaignDetail;
  isAdmin: boolean;
  className?: string;
}

export function CampaignAdminActions({
  campaign,
  isAdmin,
  className,
}: CampaignAdminActionsProps) {
  const router = useRouter();
  const { id, status } = campaign;

  // Mutation states
  const [isApproving, setIsApproving] = React.useState(false);
  const [isRejecting, setIsRejecting] = React.useState(false);
  const [isSuspending, setIsSuspending] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  // Dialog states
  const [isRejectModalOpen, setIsRejectModalOpen] = React.useState(false);
  const [rejectionReason, setRejectionReason] = React.useState("");
  const [rejectionError, setRejectionError] = React.useState<string | null>(null);

  const [isSuspendModalOpen, setIsSuspendModalOpen] = React.useState(false);

  if (!isAdmin) {
    return null;
  }

  const canApprove = status === "PENDING_REVIEW" || status === "SUSPENDED";
  const canReject = status === "PENDING_REVIEW";
  const canSuspend = status === "ACTIVE";

  if (!canApprove && !canReject && !canSuspend) {
    return null;
  }

  const mapBackendError = (code: string, defaultMessage: string): string => {
    switch (code) {
      case "INVALID_REQUEST":
        return "The request was invalid. Please verify the submitted data.";
      case "UNAUTHORIZED":
        return "Your session has expired. Please log in again.";
      case "FORBIDDEN":
        return "You do not have administrative privileges to perform this action.";
      case "NOT_FOUND":
        return "The campaign could not be found.";
      case "INVALID_STATE_TRANSITION":
        return "This lifecycle transition is not permitted from the current campaign status.";
      case "TOO_MANY_REQUESTS":
        return "Too many requests. Please wait a moment before trying again.";
      default:
        return defaultMessage || "An internal error occurred. Please try again.";
    }
  };

  const handleApprove = async () => {
    if (!canApprove || isApproving) return;

    setIsApproving(true);
    setErrorMessage(null);

    try {
      const result = await approveCampaignAction(id);
      if (!result.ok) {
        setErrorMessage(
          mapBackendError(
            result.error.code,
            result.error.message || "Failed to approve campaign."
          )
        );
      } else {
        router.refresh();
      }
    } catch {
      setErrorMessage("An unexpected network error occurred while approving.");
    } finally {
      setIsApproving(false);
    }
  };

  const handleOpenRejectModal = () => {
    setRejectionReason("");
    setRejectionError(null);
    setIsRejectModalOpen(true);
  };

  const handleConfirmReject = async () => {
    const trimmedReason = rejectionReason.trim();

    // Frontend UX validation: minimum 10 meaningful characters
    if (trimmedReason.length < 10) {
      setRejectionError("Please provide a meaningful rejection reason of at least 10 characters.");
      return;
    }

    setIsRejecting(true);
    setRejectionError(null);

    try {
      const result = await rejectCampaignAction(id, trimmedReason);
      if (!result.ok) {
        setRejectionError(
          mapBackendError(
            result.error.code,
            result.error.message || "Failed to reject campaign."
          )
        );
      } else {
        setIsRejectModalOpen(false);
        router.refresh();
      }
    } catch {
      setRejectionError("An unexpected network error occurred while rejecting.");
    } finally {
      setIsRejecting(false);
    }
  };

  const handleConfirmSuspend = async () => {
    setIsSuspending(true);
    setErrorMessage(null);

    try {
      const result = await suspendCampaignAction(id);
      if (!result.ok) {
        setErrorMessage(
          mapBackendError(
            result.error.code,
            result.error.message || "Failed to suspend campaign."
          )
        );
      } else {
        setIsSuspendModalOpen(false);
        router.refresh();
      }
    } catch {
      setErrorMessage("An unexpected network error occurred while suspending.");
    } finally {
      setIsSuspending(false);
    }
  };

  return (
    <div className={className}>
      <div className="rounded-lg border border-border-strong bg-surface-muted p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 className="text-sm font-semibold text-text-primary">
              Admin Moderation
            </h3>
            <p className="text-xs text-text-secondary">
              Review and manage the lifecycle state of this campaign initiative.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {canApprove && (
              <Button
                variant="primary"
                size="sm"
                isLoading={isApproving}
                onClick={handleApprove}
              >
                {status === "SUSPENDED" ? "Reactivate Campaign" : "Approve Campaign"}
              </Button>
            )}

            {canReject && (
              <Button
                variant="destructive"
                size="sm"
                onClick={handleOpenRejectModal}
              >
                Reject Campaign
              </Button>
            )}

            {canSuspend && (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setIsSuspendModalOpen(true)}
              >
                Suspend Campaign
              </Button>
            )}
          </div>
        </div>

        {errorMessage && (
          <Alert variant="destructive" className="mt-3">
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        )}
      </div>

      {/* Reject Confirmation Modal */}
      <Dialog open={isRejectModalOpen} onOpenChange={setIsRejectModalOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reject Campaign</DialogTitle>
            <DialogDescription>
              Please provide the creator with a clear explanation of why this campaign does not meet platform guidelines.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="admin-rejection-reason">
                Rejection Reason <span className="text-destructive">*</span>
              </Label>
              <Textarea
                id="admin-rejection-reason"
                rows={4}
                value={rejectionReason}
                onChange={(e) => setRejectionReason(e.target.value)}
                placeholder="Explain the reason for rejection (minimum 10 characters)..."
                disabled={isRejecting}
                aria-invalid={Boolean(rejectionError)}
              />
              <p className="text-xs text-text-muted">
                Minimum 10 characters. This reason will be visible to the campaign creator.
              </p>
            </div>

            {rejectionError && (
              <Alert variant="destructive">
                <AlertDescription>{rejectionError}</AlertDescription>
              </Alert>
            )}
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              size="md"
              onClick={() => setIsRejectModalOpen(false)}
              disabled={isRejecting}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="md"
              isLoading={isRejecting}
              onClick={handleConfirmReject}
            >
              Confirm Rejection
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Suspend Confirmation Modal */}
      <Dialog open={isSuspendModalOpen} onOpenChange={setIsSuspendModalOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Suspend Campaign</DialogTitle>
            <DialogDescription>
              Are you sure you want to suspend this campaign? While suspended, donations will be paused immediately and the campaign will not appear in the public discovery feed.
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant="outline"
              size="md"
              onClick={() => setIsSuspendModalOpen(false)}
              disabled={isSuspending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="md"
              isLoading={isSuspending}
              onClick={handleConfirmSuspend}
            >
              Confirm Suspension
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
