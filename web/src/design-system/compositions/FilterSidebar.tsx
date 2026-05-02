import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";
import { ScrollArea } from "@/design-system/primitives/ScrollArea";

export interface FilterSidebarProps extends HTMLAttributes<HTMLElement> {
  /** Stuck to the top — typically SearchInput or similar. */
  header?: ReactNode;
  /** Stuck to the bottom — typically actions, summary. */
  footer?: ReactNode;
  children: ReactNode;
}

/**
 * Frame for a scrollable filter/nav sidebar: sticky header on top (search
 * box, library switcher), scrollable body of Sections, optional footer.
 */
export function FilterSidebar({
  className,
  header,
  footer,
  children,
  ...props
}: FilterSidebarProps) {
  return (
    <aside className={cn("flex h-full flex-col", className)} aria-label="Filters" {...props}>
      {header && <div className="shrink-0 border-b border-line px-3 py-2">{header}</div>}
      <ScrollArea className="min-h-0 flex-1">
        <div className="py-1">{children}</div>
      </ScrollArea>
      {footer && <div className="shrink-0 border-t border-line px-3 py-2">{footer}</div>}
    </aside>
  );
}

export interface FilterRowProps extends HTMLAttributes<HTMLButtonElement> {
  icon?: ReactNode;
  label: ReactNode;
  count?: number | string;
  active?: boolean;
  /** Show a leading colored status dot. */
  dot?: ReactNode;
  disabled?: boolean;
}

/**
 * Single filter row — icon/dot + label + count. Active state matches the
 * mockup's accent-tint highlight.
 */
export function FilterRow({
  className,
  icon,
  label,
  count,
  active,
  dot,
  disabled,
  ...props
}: FilterRowProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      aria-pressed={active}
      className={cn(
        "flex w-full items-center gap-2 rounded-sm px-2 py-1 text-ui text-text-2 transition-colors duration-std ease",
        "hover:bg-surface hover:text-text",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring",
        "aria-[pressed=true]:bg-accent-tint aria-[pressed=true]:text-text",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      {dot}
      {icon && <span className="grid place-items-center text-text-3">{icon}</span>}
      <span className="flex-1 text-left">{label}</span>
      {count !== undefined && <span className="text-mono tabular-nums text-text-3">{count}</span>}
    </button>
  );
}
