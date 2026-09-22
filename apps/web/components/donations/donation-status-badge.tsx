import * as React from "react";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import type { DonationStatus } from "@/lib/api/authenticated/types";

export interface DonationStatusBadgeProps {
  status: DonationStatus | string;
  size?: BadgeProps["size"];
  className?: string;
}

const donationStatusConfig: Record<
  string,
  { label: string; variant: BadgeProps["variant"] }
> = {
  PENDING: { label: "Awaiting confirmation", variant: "warning" },
  PAID: { label: "Confirmed", variant: "success" },
  FAILED: { label: "Payment failed", variant: "destructive" },
  EXPIRED: { label: "Payment expired", variant: "secondary" },
  CANCELLED: { label: "Cancelled", variant: "secondary" },
  REFUNDED: { label: "Refunded", variant: "info" },
  PARTIALLY_REFUNDED: { label: "Partially refunded", variant: "warning" },
};

export function DonationStatusBadge({
  status,
  size = "md",
  className,
}: DonationStatusBadgeProps) {
  const config = donationStatusConfig[status] || {
    label: status,
    variant: "default" as const,
  };

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}
