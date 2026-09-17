import * as React from "react";
import { cn } from "@/lib/utils";

export interface StackProps extends React.HTMLAttributes<HTMLDivElement> {
  direction?: "vertical" | "horizontal";
  gap?: "none" | "xs" | "sm" | "md" | "lg" | "xl";
  align?: "start" | "center" | "end" | "stretch" | "baseline";
  justify?: "start" | "center" | "end" | "between" | "around";
  wrap?: boolean;
}

const directionClasses: Record<NonNullable<StackProps["direction"]>, string> = {
  vertical: "flex flex-col",
  horizontal: "flex flex-row",
};

const gapClasses: Record<NonNullable<StackProps["gap"]>, string> = {
  none: "gap-0",
  xs: "gap-1",
  sm: "gap-2",
  md: "gap-4",
  lg: "gap-6",
  xl: "gap-8",
};

const alignClasses: Record<NonNullable<StackProps["align"]>, string> = {
  start: "items-start",
  center: "items-center",
  end: "items-end",
  stretch: "items-stretch",
  baseline: "items-baseline",
};

const justifyClasses: Record<NonNullable<StackProps["justify"]>, string> = {
  start: "justify-start",
  center: "justify-center",
  end: "justify-end",
  between: "justify-between",
  around: "justify-around",
};

export const Stack = React.forwardRef<HTMLDivElement, StackProps>(
  (
    {
      className,
      direction = "vertical",
      gap = "md",
      align,
      justify,
      wrap = false,
      ...props
    },
    ref
  ) => {
    return (
      <div
        ref={ref}
        className={cn(
          directionClasses[direction],
          gapClasses[gap],
          align && alignClasses[align],
          justify && justifyClasses[justify],
          wrap && "flex-wrap",
          className
        )}
        {...props}
      />
    );
  }
);

Stack.displayName = "Stack";
