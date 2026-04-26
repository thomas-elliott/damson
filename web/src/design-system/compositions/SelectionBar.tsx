import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface SelectionBarProps extends HTMLAttributes<HTMLDivElement> {
  /** Number of selected items. */
  count: number;
  /** Right-side actions. */
  actions?: ReactNode;
  /** Optional dismiss button content on the far right. */
  onDismiss?: () => void;
}

/**
 * Floating pill that appears over the main content when the user has
 * multiple items selected. Absolute-positioned so it sits over a grid/list
 * without shifting layout. Auto-hides when count is 0.
 */
export function SelectionBar({
  className,
  count,
  actions,
  children,
  ...props
}: SelectionBarProps) {
  if (count <= 0) return null;
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "absolute bottom-4 left-1/2 z-selbar flex -translate-x-1/2 items-center gap-2.5 rounded-pill border border-line-2 bg-surface-2 py-1.5 pl-3.5 pr-1.5 text-ui text-text shadow-elevated",
        "animate-pop-up",
        className,
      )}
      {...props}
    >
      <span className="font-medium tabular-nums">{count} selected</span>
      {children}
      {actions && <div className="flex items-center gap-1">{actions}</div>}
    </div>
  );
}
