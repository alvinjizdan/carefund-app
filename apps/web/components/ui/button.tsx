import * as React from "react";
import { cn } from "@/lib/utils";
import { Spinner } from "@/components/ui/spinner";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?:
    | "primary"
    | "secondary"
    | "outline"
    | "ghost"
    | "destructive"
    | "link";
  size?: "sm" | "md" | "lg" | "icon";
  isLoading?: boolean;
}

const variantClasses = {
  primary:
    "bg-primary hover:bg-primary-hover active:bg-primary-active text-primary-fg shadow-sm",
  secondary:
    "bg-surface-muted hover:bg-surface-hover active:bg-surface-active text-text-primary",
  outline:
    "border border-border-strong bg-surface-card hover:bg-surface-hover active:bg-surface-active text-text-primary",
  ghost:
    "bg-transparent hover:bg-surface-hover active:bg-surface-active text-text-primary",
  destructive:
    "bg-destructive hover:bg-destructive-hover active:bg-destructive-active text-destructive-fg shadow-sm",
  link: "text-primary underline-offset-4 hover:underline p-0 h-auto bg-transparent",
};

const sizeClasses = {
  sm: "h-8 px-3 text-xs rounded-md gap-1.5",
  md: "h-10 px-4 text-sm rounded-lg gap-2",
  lg: "h-12 px-6 text-base rounded-lg gap-2.5",
  icon: "h-10 w-10 p-0 rounded-lg justify-center",
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = "primary",
      size = "md",
      isLoading = false,
      disabled = false,
      children,
      type = "button",
      ...props
    },
    ref
  ) => {
    const isInactive = disabled || isLoading;

    return (
      <button
        ref={ref}
        type={type}
        disabled={isInactive}
        aria-busy={isLoading ? true : undefined}
        className={cn(
          "inline-flex items-center justify-center font-medium transition-colors select-none",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          variantClasses[variant],
          sizeClasses[size],
          className
        )}
        {...props}
      >
        {isLoading && <Spinner size="sm" aria-hidden="true" />}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
