import * as React from "react";
import { cn } from "@/lib/utils";

export interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  title: string;
  description?: string;
  icon?: React.ReactNode;
  action?: React.ReactNode;
}

export const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  (
    { className, title, description, icon, action, children, ...props },
    ref
  ) => {
    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-col items-center justify-center rounded-xl border border-dashed border-border-strong bg-surface-card px-6 py-12 text-center",
          className
        )}
        {...props}
      >
        {icon && (
          <div
            className="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-surface-muted text-text-secondary"
            aria-hidden="true"
          >
            {icon}
          </div>
        )}
        <h3 className="text-base font-semibold text-text-primary sm:text-lg">
          {title}
        </h3>
        {description && (
          <p className="mt-1.5 max-w-sm text-sm text-text-secondary">
            {description}
          </p>
        )}
        {children && <div className="mt-4">{children}</div>}
        {action && <div className="mt-6">{action}</div>}
      </div>
    );
  }
);

EmptyState.displayName = "EmptyState";
