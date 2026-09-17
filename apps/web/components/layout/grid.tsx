import * as React from "react";
import { cn } from "@/lib/utils";

export interface GridProps extends React.HTMLAttributes<HTMLDivElement> {
  cols?: 1 | 2 | 3 | 4 | 6 | 12 | "auto";
  gap?: "none" | "xs" | "sm" | "md" | "lg" | "xl";
}

const colClasses: Record<NonNullable<GridProps["cols"]>, string> = {
  1: "grid-cols-1",
  2: "grid-cols-1 sm:grid-cols-2",
  3: "grid-cols-1 sm:grid-cols-2 lg:grid-cols-3",
  4: "grid-cols-1 sm:grid-cols-2 lg:grid-cols-4",
  6: "grid-cols-2 sm:grid-cols-3 lg:grid-cols-6",
  12: "grid-cols-12",
  auto: "grid-cols-[repeat(auto-fit,minmax(250px,1fr))]",
};

const gapClasses: Record<NonNullable<GridProps["gap"]>, string> = {
  none: "gap-0",
  xs: "gap-1",
  sm: "gap-2",
  md: "gap-4",
  lg: "gap-6",
  xl: "gap-8",
};

export const Grid = React.forwardRef<HTMLDivElement, GridProps>(
  ({ className, cols = 3, gap = "md", ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn("grid", colClasses[cols], gapClasses[gap], className)}
        {...props}
      />
    );
  }
);

Grid.displayName = "Grid";
