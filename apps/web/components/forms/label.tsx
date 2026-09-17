import * as React from "react";
import { cn } from "@/lib/utils";

export interface LabelProps
  extends React.LabelHTMLAttributes<HTMLLabelElement> {
  required?: boolean;
}

export const Label = React.forwardRef<HTMLLabelElement, LabelProps>(
  ({ className, required = false, children, ...props }, ref) => {
    return (
      <label
        ref={ref}
        className={cn(
          "inline-flex items-center text-sm font-medium text-text-primary leading-none select-none",
          "peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
          className
        )}
        {...props}
      >
        {children}
        {required && (
          <span className="ml-1 inline-flex items-center">
            <span aria-hidden="true" className="text-destructive font-bold">
              *
            </span>
            <span className="sr-only">(required)</span>
          </span>
        )}
      </label>
    );
  }
);

Label.displayName = "Label";
