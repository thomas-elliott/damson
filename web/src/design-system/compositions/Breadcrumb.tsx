import { type HTMLAttributes, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface BreadcrumbProps extends HTMLAttributes<HTMLElement> {
  /** Count shown in the trailing mono subtext. E.g. "254 assets". */
  count?: ReactNode;
  children: ReactNode;
}

/** Breadcrumb ancestry text + optional count suffix. */
export function Breadcrumb({
  className,
  count,
  children,
  ...props
}: BreadcrumbProps) {
  return (
    <nav
      aria-label="Breadcrumb"
      className={cn("flex items-center gap-2", className)}
      {...props}
    >
      <div className="flex items-center gap-1.5 text-sm-plus font-medium text-text">
        {children}
      </div>
      {count !== undefined && (
        <span className="font-mono text-xs tabular-nums text-text-3">
          {count}
        </span>
      )}
    </nav>
  );
}

export interface BreadcrumbItemProps extends HTMLAttributes<HTMLSpanElement> {
  current?: boolean;
}

/** Single crumb — display-only, no link semantics built in. */
export function BreadcrumbItem({
  className,
  current,
  children,
  ...props
}: BreadcrumbItemProps) {
  return (
    <span
      aria-current={current ? "page" : undefined}
      className={cn(
        "text-text-2 aria-[current=page]:text-text",
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export function BreadcrumbSeparator() {
  return (
    <ChevronRight aria-hidden size={12} className="text-text-4" />
  );
}
