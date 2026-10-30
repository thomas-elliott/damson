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
  "flex w-full items-start gap-2.5 rounded-sm border-l-2 border-transparent px-2 py-2 text-left text-label transition-colors duration-fast ease-standard";
const currentClasses = "border-primary bg-primary-soft";

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
              <span className="font-mono text-list-row font-medium text-foreground">
                {storeVersion}
              </span>
              <span className="text-overline text-subtle-foreground">{version}</span>
            </>
          ) : (
            <span className="font-mono text-list-row font-medium text-foreground">{version}</span>
          )}
          {current && <span className="text-overline text-primary-text">Current</span>}
          {action && <span className="ml-auto">{action}</span>}
        </div>
        {notes && <div className="mt-0.5 text-muted-foreground">{notes}</div>}
      </div>
      <div className="flex flex-col items-end gap-0.5 text-right">
        <span className="text-mono tabular-nums text-subtle-foreground">{date}</span>
        {meta && <span className="text-mono tabular-nums text-subtle-foreground">{meta}</span>}
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
          "cursor-pointer hover:bg-muted focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
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
