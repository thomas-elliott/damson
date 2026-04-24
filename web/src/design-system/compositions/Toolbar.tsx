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
        "flex min-h-[42px] shrink-0 items-center gap-2.5 border-b border-line bg-bg px-3.5",
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
