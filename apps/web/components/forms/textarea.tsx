import * as React from "react";
import { cn } from "@/lib/utils";

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, disabled, readOnly, required, ...props }, ref) => {
    return (
      <textarea
        ref={ref}
        disabled={disabled}
        readOnly={readOnly}
        required={required}
        className={cn(
          "flex min-h-[80px] w-full rounded-lg border border-border-strong bg-surface-card px-3 py-2 text-sm text-text-primary transition-colors",
          "placeholder:text-text-muted",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "read-only:cursor-default read-only:bg-surface-muted",
          "aria-invalid:border-destructive aria-invalid:focus-visible:ring-destructive",
          className
        )}
        {...props}
      />
    );
  }
);

Textarea.displayName = "Textarea";
