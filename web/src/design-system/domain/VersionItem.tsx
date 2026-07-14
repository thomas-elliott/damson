import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface VersionItemProps extends HTMLAttributes<HTMLDivElement> {
  /** Internal sequence label (e.g. "v3"). */
  version: string;
  /** Store-facing version (e.g. "1.1"), shown alongside the internal label. */
  storeVersion?: string;
  date: ReactNode;
  /** File count or other right-aligned metadata. */
  meta?: ReactNode;
  notes?: ReactNode;
  /** Highlights as the currently-pinned version. */
  current?: boolean;
  /** Right-aligned slot in the header row (e.g. an edit control). */
  action?: ReactNode;
  onSelect?: () => void;
}

const baseClasses =
  "flex w-full items-start gap-2.5 rounded-sm border-l-2 border-transparent px-2 py-2 text-left text-small transition-colors duration-std ease";
const currentClasses = "border-accent bg-accent-tint";

/**
 * Timeline entry in an asset's version history. Current version gets an
 * accent left bar and tint background. Renders as a button when `onSelect`
 * is set, a div otherwise.
 */
export function VersionItem({
  className,
  version,
  storeVersion,
  date,
  meta,
  notes,
  current,
  action,
  onSelect,
  ...props
}: VersionItemProps) {
  const body = (
    <>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          {storeVersion ? (
            <>
              <span className="font-mono text-list-row font-medium text-text">{storeVersion}</span>
              <span className="text-label text-text-3">{version}</span>
            </>
          ) : (
            <span className="font-mono text-list-row font-medium text-text">{version}</span>
          )}
          {current && <span className="text-label text-accent">Current</span>}
          {action && <span className="ml-auto">{action}</span>}
        </div>
        {notes && <div className="mt-0.5 text-text-2">{notes}</div>}
      </div>
      <div className="flex flex-col items-end gap-0.5 text-right">
        <span className="text-mono tabular-nums text-text-3">{date}</span>
        {meta && <span className="text-mono tabular-nums text-text-3">{meta}</span>}
      </div>
    </>
  );

  if (onSelect) {
    return (
      <button
        type="button"
        onClick={onSelect}
        className={cn(
          baseClasses,
          "cursor-pointer hover:bg-surface focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring",
          current && currentClasses,
          className,
        )}
      >
        {body}
      </button>
    );
  }

  return (
    <div className={cn(baseClasses, current && currentClasses, className)} {...props}>
      {body}
    </div>
  );
}
