import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface ToolbarProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

/**
 * Horizontal toolbar — 42px min height, border-bottom, gap-2.5 between
 * children. Compose Breadcrumb / chips / ToolbarSpacer / buttons inside.
 */
export function Toolbar({ className, children, ...props }: ToolbarProps) {
  return (
    <div
      role="toolbar"
      className={cn(
        // Wraps rather than overflowing, so every action stays reachable on
        // a narrow screen.
        "flex min-h-[42px] shrink-0 flex-wrap items-center gap-x-2.5 gap-y-1.5 border-b border-border bg-background px-3.5 py-1",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function ToolbarSpacer() {
  return <div className="flex-1" aria-hidden />;
}
