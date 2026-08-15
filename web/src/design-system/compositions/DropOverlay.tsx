import { type HTMLAttributes, type ReactNode } from "react";
import { UploadCloud } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface DropOverlayProps extends HTMLAttributes<HTMLDivElement> {
  /** When false, returns null — safe to keep in render tree. */
  active: boolean;
  /** Optional override for the centered caption. */
  caption?: ReactNode;
}

/**
 * Absolute-inset drop target indicator. Stays behind pointer events (the
 * real drop handler is on the parent) — purely visual feedback.
 */
export function DropOverlay({
  className,
  active,
  caption = "Drop files to import",
  ...props
}: DropOverlayProps) {
  if (!active) return null;
  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-3.5 z-drop grid place-items-center rounded-md border-2 border-dashed border-primary bg-primary-soft text-foreground animate-fade-in",
        className,
      )}
      {...props}
    >
      <div className="flex flex-col items-center gap-2 text-ui">
        <UploadCloud size={28} className="text-primary" />
        <span className="font-medium">{caption}</span>
      </div>
    </div>
  );
}
