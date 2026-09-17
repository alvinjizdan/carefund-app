import * as React from "react";
import { cn } from "@/lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?:
    | "default"
    | "primary"
    | "secondary"
    | "destructive"
    | "warning"
    | "info"
    | "success"
    | "outline";
  size?: "sm" | "md";
}

const variantClasses = {
  default: "bg-surface-muted text-text-primary border border-border-subtle",
  primary: "bg-primary text-primary-fg border-transparent",
  secondary: "bg-surface-card text-text-secondary border border-border-subtle",
  destructive: "bg-destructive text-destructive-fg border-transparent",
  warning: "bg-warning-surface text-warning-text border border-warning-border",
  info: "bg-info-surface text-info-text border border-info-border",
  success: "bg-success-surface text-success-text border border-success-border",
  outline: "bg-transparent text-text-primary border border-border-strong",
};

const sizeClasses = {
  sm: "px-2 py-0.5 text-xs",
  md: "px-2.5 py-1 text-xs font-medium",
};

export const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = "default", size = "md", children, ...props }, ref) => {
    return (
      <span
        ref={ref}
        className={cn(
          "inline-flex items-center gap-1 rounded-full border transition-colors select-none",
          variantClasses[variant],
          sizeClasses[size],
          className
        )}
        {...props}
      >
        {children}
      </span>
    );
  }
);

Badge.displayName = "Badge";
