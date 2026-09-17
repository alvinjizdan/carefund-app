"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { Label, type LabelProps } from "@/components/forms/label";

export interface FormFieldContextValue {
  id: string;
  helperId: string;
  errorId: string;
  error?: string;
  required?: boolean;
  isInvalid: boolean;
  hasHelper?: boolean;
  setHasHelper?: (has: boolean) => void;
  ariaDescribedBy?: string;
}

const FormFieldContext = React.createContext<FormFieldContextValue | null>(null);

export function useFormField(): FormFieldContextValue | null {
  return React.useContext(FormFieldContext);
}

export interface FormFieldProps extends React.HTMLAttributes<HTMLDivElement> {
  id?: string;
  error?: string;
  required?: boolean;
  hasHelper?: boolean;
}

export const FormField = React.forwardRef<HTMLDivElement, FormFieldProps>(
  ({ className, id: propId, error, required = false, hasHelper: propHasHelper, children, ...props }, ref) => {
    const generatedId = React.useId();
    const id = propId ?? generatedId;
    const helperId = `${id}-helper`;
    const errorId = `${id}-error`;
    const [hasHelperState, setHasHelperState] = React.useState(false);
    const hasHelper = propHasHelper ?? hasHelperState;
    const isInvalid = Boolean(error);

    const ariaDescribedBy = [
      hasHelper ? helperId : undefined,
      isInvalid ? errorId : undefined,
    ]
      .filter(Boolean)
      .join(" ") || undefined;

    const contextValue = React.useMemo<FormFieldContextValue>(
      () => ({
        id,
        helperId,
        errorId,
        error,
        required,
        isInvalid,
        hasHelper,
        setHasHelper: setHasHelperState,
        ariaDescribedBy,
      }),
      [id, helperId, errorId, error, required, isInvalid, hasHelper, ariaDescribedBy]
    );

    return (
      <FormFieldContext.Provider value={contextValue}>
        <div
          ref={ref}
          className={cn("flex flex-col gap-1.5", className)}
          {...props}
        >
          {children}
        </div>
      </FormFieldContext.Provider>
    );
  }
);

FormField.displayName = "FormField";

export interface FormLabelProps extends LabelProps {}

export const FormLabel = React.forwardRef<HTMLLabelElement, FormLabelProps>(
  ({ htmlFor, required, children, ...props }, ref) => {
    const field = useFormField();
    const resolvedHtmlFor = htmlFor ?? field?.id;
    const resolvedRequired = required ?? field?.required;

    return (
      <Label
        ref={ref}
        htmlFor={resolvedHtmlFor}
        required={resolvedRequired}
        {...props}
      >
        {children}
      </Label>
    );
  }
);

FormLabel.displayName = "FormLabel";

export interface FormHelperTextProps
  extends React.HTMLAttributes<HTMLParagraphElement> {}

export const FormHelperText = React.forwardRef<
  HTMLParagraphElement,
  FormHelperTextProps
>(({ className, id, children, ...props }, ref) => {
  const field = useFormField();
  const setHasHelper = field?.setHasHelper;

  React.useEffect(() => {
    setHasHelper?.(true);
    return () => setHasHelper?.(false);
  }, [setHasHelper]);

  const resolvedId = id ?? field?.helperId;

  return (
    <p
      ref={ref}
      id={resolvedId}
      className={cn("text-xs text-text-secondary", className)}
      {...props}
    >
      {children}
    </p>
  );
});

FormHelperText.displayName = "FormHelperText";

export interface FormErrorMessageProps
  extends React.HTMLAttributes<HTMLParagraphElement> {
  message?: string;
}

export const FormErrorMessage = React.forwardRef<
  HTMLParagraphElement,
  FormErrorMessageProps
>(({ className, id, message, children, ...props }, ref) => {
  const field = useFormField();
  const resolvedId = id ?? field?.errorId;
  const content = message ?? children ?? field?.error;

  if (!content) {
    return null;
  }

  return (
    <p
      ref={ref}
      id={resolvedId}
      role="alert"
      aria-live="assertive"
      className={cn("text-xs font-medium text-destructive", className)}
      {...props}
    >
      {content}
    </p>
  );
});

FormErrorMessage.displayName = "FormErrorMessage";
