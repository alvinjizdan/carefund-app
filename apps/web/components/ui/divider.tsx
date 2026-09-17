import * as React from "react";
import { cn } from "@/lib/utils";

export interface DividerProps extends React.HTMLAttributes<HTMLDivElement> {
  orientation?: "horizontal" | "vertical";
  label?: string;
}

export const Divider = React.forwardRef<HTMLDivElement, DividerProps>(
  ({ orientation = "horizontal", label, className, ...props }, ref) => {
    if (orientation === "vertical") {
      return (
        <div
          ref={ref}
          role="separator"
          aria-orientation="vertical"
          className={cn(
            "inline-block h-full min-h-[1em] w-px self-stretch bg-border-subtle",
            className
          )}
          {...props}
        />
      );
    }

    if (label) {
      return (
        <div
          ref={ref}
          role="separator"
          aria-orientation="horizontal"
          className={cn(
            "relative flex w-full items-center justify-center my-4",
            className
          )}
          {...props}
        >
          <div className="absolute inset-0 flex items-center" aria-hidden="true">
            <div className="w-full border-t border-border-subtle" />
          </div>
          <span className="relative bg-surface-card px-3 text-xs font-medium text-text-muted uppercase tracking-wider">
            {label}
          </span>
        </div>
      );
    }

    return (
      <hr
        role="separator"
        aria-orientation="horizontal"
        className={cn(
          "w-full border-0 border-t border-border-subtle my-4",
          className
        )}
        {...props}
      />
    );
  }
);

Divider.displayName = "Divider";
