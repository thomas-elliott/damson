import { forwardRef, type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";
import { ProgressBar } from "@/design-system/primitives/ProgressBar";

export interface QueueItemProps extends HTMLAttributes<HTMLDivElement> {
  /** Thumbnail content (icon, mini-thumb, etc.). */
  thumb?: ReactNode;
  name: string;
  /** Short secondary line (detected type, size, timestamps). */
  sub?: ReactNode;
  /** 0–100 import progress. Omit when already-resolved. */
  progress?: number;
  /** Actions or status indicators on the right. */
  actions?: ReactNode;
  /** Extra block under the meta line — description snippets, inline notes. */
  detail?: ReactNode;
  /** Thumbnail edge length in px. Defaults to 56. */
  thumbSize?: number;
}

/**
 * Single row in the import queue view. Fixed 56px thumbnail, flexible meta,
 * optional progress bar on the bottom. Actions sit on the right.
 */
export const QueueItem = forwardRef<HTMLDivElement, QueueItemProps>(function QueueItem(
  { className, thumb, name, sub, progress, actions, detail, thumbSize = 56, ...props },
  ref,
) {
  return (
    <div
      ref={ref}
      className={cn(
        "flex gap-3.5 rounded-md border border-line bg-panel p-3.5 transition-colors duration-std ease hover:border-line-2",
        detail ? "items-start" : "items-center",
        className,
      )}
      {...props}
    >
      {thumb && (
        <div
          style={{ width: thumbSize, height: thumbSize }}
          className="grid shrink-0 place-items-center overflow-hidden rounded-sm border border-line bg-surface text-text-3"
        >
          {thumb}
        </div>
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="truncate text-list-row font-medium text-text">{name}</div>
        {sub && <div className="flex items-center gap-2 text-small text-text-3">{sub}</div>}
        {detail}
        {typeof progress === "number" && (
          <ProgressBar value={progress} aria-label={`${Math.round(progress)}% uploaded`} />
        )}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-1">{actions}</div>}
    </div>
  );
});
