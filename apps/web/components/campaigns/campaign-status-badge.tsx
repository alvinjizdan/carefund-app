import * as React from "react";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import type { CampaignStatus } from "@/lib/api/public/campaigns";

export interface CampaignStatusBadgeProps {
  status: CampaignStatus | string;
  size?: BadgeProps["size"];
  className?: string;
}

const statusConfig: Record<
  string,
  { label: string; variant: BadgeProps["variant"] }
> = {
  DRAFT: { label: "Draft", variant: "warning" },
  PENDING_REVIEW: { label: "Pending Review", variant: "info" },
  ACTIVE: { label: "Active", variant: "success" },
  REJECTED: { label: "Rejected", variant: "destructive" },
  SUSPENDED: { label: "Suspended", variant: "destructive" },
  COMPLETED: { label: "Completed", variant: "default" },
  CANCELLED: { label: "Cancelled", variant: "secondary" },
};

export function CampaignStatusBadge({
  status,
  size = "md",
  className,
}: CampaignStatusBadgeProps) {
  const config = statusConfig[status] || {
    label: status,
    variant: "default" as const,
  };

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}
