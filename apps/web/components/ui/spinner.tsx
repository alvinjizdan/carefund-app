import * as React from "react";
import { cn } from "@/lib/utils";

export interface SpinnerProps extends React.HTMLAttributes<HTMLSpanElement> {
  size?: "sm" | "md" | "lg";
  label?: string;
}

const sizeClasses = {
  sm: "h-4 w-4 border-2",
  md: "h-6 w-6 border-2",
  lg: "h-8 w-8 border-3",
};

export const Spinner = React.forwardRef<HTMLSpanElement, SpinnerProps>(
  ({ size = "md", label = "Loading...", className, ...props }, ref) => {
    return (
      <span
        ref={ref}
        role="status"
        aria-live="polite"
        className={cn("inline-flex items-center justify-center", className)}
        {...props}
      >
        <span
          className={cn(
            "animate-spin rounded-full border-current border-t-transparent motion-reduce:animate-none",
            sizeClasses[size]
          )}
          aria-hidden="true"
        />
        <span className="sr-only">{label}</span>
      </span>
    );
  }
);

Spinner.displayName = "Spinner";
