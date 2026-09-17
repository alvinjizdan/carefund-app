import * as React from "react";
import { cn } from "@/lib/utils";

export type AlertVariant =
  | "default"
  | "info"
  | "success"
  | "warning"
  | "destructive";

export interface AlertProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: AlertVariant;
  icon?: React.ReactNode;
}

const variantClasses: Record<AlertVariant, string> = {
  default: "bg-surface-muted text-text-primary border-border-subtle",
  info: "bg-info-surface text-info-text border-info-border",
  success: "bg-success-surface text-success-text border-success-border",
  warning: "bg-warning-surface text-warning-text border-warning-border",
  destructive: "bg-destructive text-destructive-fg border-destructive-hover",
};

const roleMap: Record<AlertVariant, "alert" | "status"> = {
  destructive: "alert",
  warning: "alert",
  info: "status",
  success: "status",
  default: "status",
};

const ariaLiveMap: Record<AlertVariant, "assertive" | "polite"> = {
  destructive: "assertive",
  warning: "assertive",
  info: "polite",
  success: "polite",
  default: "polite",
};

export const Alert = React.forwardRef<HTMLDivElement, AlertProps>(
  (
    {
      className,
      variant = "default",
      icon,
      role,
      "aria-live": ariaLive,
      children,
      ...props
    },
    ref
  ) => {
    const resolvedRole = role ?? roleMap[variant];
    const resolvedAriaLive = ariaLive ?? ariaLiveMap[variant];

    return (
      <div
        ref={ref}
        role={resolvedRole}
        aria-live={resolvedAriaLive}
        className={cn(
          "relative w-full rounded-lg border p-4 text-sm transition-colors",
          variantClasses[variant],
          icon && "flex items-start gap-3",
          className
        )}
        {...props}
      >
        {icon && (
          <span className="shrink-0 mt-0.5 text-current" aria-hidden="true">
            {icon}
          </span>
        )}
        <div className="flex-1 space-y-1">{children}</div>
      </div>
    );
  }
);

Alert.displayName = "Alert";

export interface AlertTitleProps
  extends React.HTMLAttributes<HTMLHeadingElement> {}

export const AlertTitle = React.forwardRef<HTMLHeadingElement, AlertTitleProps>(
  ({ className, children, ...props }, ref) => {
    return (
      <h5
        ref={ref}
        className={cn("font-semibold leading-none tracking-tight", className)}
        {...props}
      >
        {children}
      </h5>
    );
  }
);

AlertTitle.displayName = "AlertTitle";

export interface AlertDescriptionProps
  extends React.HTMLAttributes<HTMLParagraphElement> {}

export const AlertDescription = React.forwardRef<
  HTMLParagraphElement,
  AlertDescriptionProps
>(({ className, children, ...props }, ref) => {
  return (
    <div
      ref={ref}
      className={cn("text-sm leading-relaxed [&_p]:leading-relaxed", className)}
      {...props}
    >
      {children}
    </div>
  );
});

AlertDescription.displayName = "AlertDescription";
