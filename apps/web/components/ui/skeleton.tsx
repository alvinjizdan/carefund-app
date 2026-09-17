import * as React from "react";
import { cn } from "@/lib/utils";

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {}

export const Skeleton = React.forwardRef<HTMLDivElement, SkeletonProps>(
  ({ className, ...props }, ref) => {
    return (
      <div
        ref={ref}
        aria-hidden="true"
        className={cn(
          "animate-pulse rounded-md bg-surface-muted motion-reduce:animate-none",
          className
        )}
        {...props}
      />
    );
  }
);

Skeleton.displayName = "Skeleton";
