"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { useFormField } from "@/components/forms/form-field";

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

/**
 * Native multiline control for the form system.
 *
 * Mirrors `<Input>`: inside a `<FormField>` it inherits the field `id`,
 * `aria-invalid` state and `aria-describedby` wiring, while explicit props
 * passed by the call site always take precedence.
 */
export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  (
    {
      className,
      disabled,
      readOnly,
      required,
      id,
      "aria-describedby": ariaDescribedBy,
      "aria-invalid": ariaInvalid,
      ...props
    },
    ref
  ) => {
    const field = useFormField();

    return (
      <textarea
        ref={ref}
        id={id ?? field?.id}
        disabled={disabled}
        readOnly={readOnly}
        required={required ?? field?.required}
        aria-invalid={ariaInvalid ?? (field?.isInvalid ? true : undefined)}
        aria-describedby={ariaDescribedBy ?? field?.ariaDescribedBy}
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
