import { type HTMLAttributes, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface BreadcrumbProps extends HTMLAttributes<HTMLElement> {
  /** Count shown in the trailing mono subtext. E.g. "254 assets". */
  count?: ReactNode;
  children: ReactNode;
}

/** Breadcrumb ancestry text + optional count suffix. */
export function Breadcrumb({ className, count, children, ...props }: BreadcrumbProps) {
  return (
    <nav aria-label="Breadcrumb" className={cn("flex items-center gap-2", className)} {...props}>
      <div className="flex items-center gap-1.5 text-ui font-medium text-foreground">
        {children}
      </div>
      {count !== undefined && (
        <span className="text-mono tabular-nums text-subtle-foreground">{count}</span>
      )}
    </nav>
  );
}

export interface BreadcrumbItemProps extends HTMLAttributes<HTMLElement> {
  current?: boolean;
  /**
   * Render the crumb as the page's `h1`. Set it on the current crumb of a
   * page whose body has no `h1` of its own; it looks the same either way.
   */
  heading?: boolean;
}

/** Single crumb — display-only, no link semantics built in. */
export function BreadcrumbItem({
  className,
  current,
  heading,
  children,
  ...props
}: BreadcrumbItemProps) {
  const Tag = heading ? "h1" : "span";
  return (
    <Tag
      aria-current={current ? "page" : undefined}
      className={cn("text-muted-foreground aria-[current=page]:text-foreground", className)}
      {...props}
    >
      {children}
    </Tag>
  );
}

export function BreadcrumbSeparator() {
  return <ChevronRight aria-hidden size={12} className="text-faint-foreground" />;
}
