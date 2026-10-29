import { type HTMLAttributes } from "react";

import { cn } from "@/design-system/utils";

export interface PageContentProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * `full` fills the main column; `wide` (lists, logs) and `narrow` (forms)
   * cap the line length at `--container-content-wide` / `-narrow`.
   */
  width?: "full" | "wide" | "narrow";
}

/**
 * The scrolling content column under a page's Toolbar. It owns the padding
 * and the max width, so every page's content starts on the toolbar's 14px
 * edge; a capped column stays left-aligned for the same reason. Children
 * stack in a flex column, so an EmptyState, ErrorState or LoadingState
 * fills the space.
 */
export function PageContent({ width = "full", className, children, ...props }: PageContentProps) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div
        className={cn(
          "flex min-h-full flex-col p-3.5",
          width === "wide" && "max-w-content-wide",
          width === "narrow" && "max-w-content-narrow",
          className,
        )}
        {...props}
      >
        {children}
      </div>
    </div>
  );
}
