"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { useFormField } from "@/components/forms/form-field";

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {}

/**
 * Native text control for the form system.
 *
 * When rendered inside a `<FormField>`, the control adopts the field's generated
 * `id`, `aria-invalid` state and `aria-describedby` wiring so that the label,
 * helper text and error message are programmatically associated with it. Every
 * value remains overridable per call site: an explicit `id`, `aria-invalid`,
 * `aria-describedby` or `required` prop always wins over the context.
 */
export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      className,
      type = "text",
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
      <input
        ref={ref}
        id={id ?? field?.id}
        type={type}
        disabled={disabled}
        readOnly={readOnly}
        required={required ?? field?.required}
        aria-invalid={ariaInvalid ?? (field?.isInvalid ? true : undefined)}
        aria-describedby={ariaDescribedBy ?? field?.ariaDescribedBy}
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
