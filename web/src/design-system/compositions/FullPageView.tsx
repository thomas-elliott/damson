import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";
import { ScrollArea } from "@/design-system/primitives/ScrollArea";

export interface FullPageViewProps extends HTMLAttributes<HTMLDivElement> {
  header: ReactNode;
  preview: ReactNode;
  side: ReactNode;
  /** Right-side panel width, defaults to 440px. */
  sideWidth?: number;
}

/**
 * Full-page view: shared header, then grid of (preview | side panel).
 * Preview is center-aligned with padding and max-width; side panel scrolls
 * independently.
 */
export function FullPageView({
  className,
  header,
  preview,
  side,
  sideWidth = 440,
  ...props
}: FullPageViewProps) {
  return (
    <div
      className={cn("flex h-full flex-col bg-bg", className)}
      {...props}
    >
      <header className="flex shrink-0 items-center gap-2 border-b border-line bg-bg px-3.5 py-2.5">
        {header}
      </header>
      <div
        className="grid min-h-0 flex-1"
        style={{ gridTemplateColumns: `minmax(0,1fr) ${sideWidth}px` }}
      >
        <ScrollArea>
          <div className="grid place-items-center p-10">
            <div className="w-full max-w-[640px]">{preview}</div>
          </div>
        </ScrollArea>
        <ScrollArea className="border-l border-line bg-panel">
          <div className="px-3.5 py-3">{side}</div>
        </ScrollArea>
      </div>
    </div>
  );
}
