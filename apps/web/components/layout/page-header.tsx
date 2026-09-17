import * as React from "react";
import { cn } from "@/lib/utils";

export interface PageHeaderProps
  extends React.HTMLAttributes<HTMLDivElement> {
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
}

export const PageHeader = React.forwardRef<HTMLDivElement, PageHeaderProps>(
  ({ className, title, description, action, children, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-col gap-4 pb-6 md:flex-row md:items-center md:justify-between md:pb-8",
          className
        )}
        {...props}
      >
        <div className="space-y-1.5">
          <h1 className="text-2xl font-bold tracking-tight text-text-primary sm:text-3xl">
            {title}
          </h1>
          {description && (
            <p className="text-sm text-text-secondary sm:text-base">
              {description}
            </p>
          )}
          {children}
        </div>
        {action && (
          <div className="flex shrink-0 items-center gap-2 pt-2 md:pt-0">
            {action}
          </div>
        )}
      </div>
    );
  }
);

PageHeader.displayName = "PageHeader";
