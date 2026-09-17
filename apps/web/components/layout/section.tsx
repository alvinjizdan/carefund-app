import * as React from "react";
import { cn } from "@/lib/utils";

export interface SectionProps extends React.HTMLAttributes<HTMLElement> {
  spacing?: "none" | "sm" | "md" | "lg" | "xl";
  variant?: "ground" | "card" | "muted" | "transparent";
}

const spacingClasses: Record<NonNullable<SectionProps["spacing"]>, string> = {
  none: "py-0",
  sm: "py-6 sm:py-8",
  md: "py-10 sm:py-12",
  lg: "py-12 sm:py-16 lg:py-20",
  xl: "py-16 sm:py-24 lg:py-32",
};

const variantClasses: Record<NonNullable<SectionProps["variant"]>, string> = {
  ground: "bg-surface-ground text-text-primary",
  card: "bg-surface-card text-text-primary",
  muted: "bg-surface-muted text-text-primary",
  transparent: "bg-transparent text-text-primary",
};

export const Section = React.forwardRef<HTMLElement, SectionProps>(
  ({ className, spacing = "md", variant = "transparent", ...props }, ref) => {
    return (
      <section
        ref={ref}
        className={cn(
          "w-full transition-colors",
          spacingClasses[spacing],
          variantClasses[variant],
          className
        )}
        {...props}
      />
    );
  }
);

Section.displayName = "Section";
