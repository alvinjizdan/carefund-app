"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { useFormField } from "@/components/forms/form-field";

export interface SelectProps
  extends React.SelectHTMLAttributes<HTMLSelectElement> {
  wrapperClassName?: string;
}

/**
 * Native select for the form system.
 *
 * Mirrors `<Input>`: inside a `<FormField>` it inherits the field `id`,
 * `aria-invalid` state and `aria-describedby` wiring, while explicit props
 * passed by the call site always take precedence. The decorative chevron stays
 * `aria-hidden`, so it is never announced.
 */
export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  (
    {
      className,
      wrapperClassName,
      disabled,
      required,
      id,
      "aria-describedby": ariaDescribedBy,
      "aria-invalid": ariaInvalid,
      children,
      ...props
    },
    ref
  ) => {
    const field = useFormField();

    return (
      <div className={cn("relative w-full", wrapperClassName)}>
        <select
          ref={ref}
          id={id ?? field?.id}
          disabled={disabled}
          required={required ?? field?.required}
          aria-invalid={ariaInvalid ?? (field?.isInvalid ? true : undefined)}
          aria-describedby={ariaDescribedBy ?? field?.ariaDescribedBy}
          className={cn(
            "flex h-10 w-full appearance-none rounded-lg border border-border-strong bg-surface-card px-3 py-2 pr-9 text-sm text-text-primary transition-colors",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card",
            "disabled:cursor-not-allowed disabled:opacity-50",
            "aria-invalid:border-destructive aria-invalid:focus-visible:ring-destructive",
            className
          )}
          {...props}
        >
          {children}
        </select>
        <span
          className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-text-secondary"
          aria-hidden="true"
        >
          <svg
            className="h-4 w-4"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fillRule="evenodd"
              d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
              clipRule="evenodd"
            />
          </svg>
        </span>
      </div>
    );
  }
);

Select.displayName = "Select";
