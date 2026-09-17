import * as React from "react";
import { cn } from "@/lib/utils";

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", disabled, readOnly, required, ...props }, ref) => {
    return (
      <input
        ref={ref}
        type={type}
        disabled={disabled}
        readOnly={readOnly}
        required={required}
        className={cn(
          "flex h-10 w-full rounded-lg border border-border-strong bg-surface-card px-3 py-2 text-sm text-text-primary transition-colors",
          "placeholder:text-text-muted",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "read-only:cursor-default read-only:bg-surface-muted",
          "aria-invalid:border-destructive aria-invalid:focus-visible:ring-destructive",
          "file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-text-primary",
          className
        )}
        {...props}
      />
    );
  }
);

Input.displayName = "Input";
