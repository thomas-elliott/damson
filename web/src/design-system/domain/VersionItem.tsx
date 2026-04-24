import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface VersionItemProps extends HTMLAttributes<HTMLDivElement> {
  /** Version identifier (e.g. "1.12.3"). */
  version: string;
  date: ReactNode;
  /** File count or other right-aligned metadata. */
  meta?: ReactNode;
  notes?: ReactNode;
  /** Highlights as the currently-pinned version. */
  current?: boolean;
  onSelect?: () => void;
}

const baseClasses =
  "flex w-full items-start gap-2.5 rounded-sm border-l-2 border-transparent px-2 py-2 text-left text-xs-plus transition-colors duration-std ease";
const currentClasses = "border-accent bg-accent-tint";

/**
 * Timeline entry in an asset's version history. Current version gets an
 * accent left bar and tint background. Renders as a button when `onSelect`
 * is set, a div otherwise.
 */
export function VersionItem({
  className,
  version,
  date,
  meta,
  notes,
  current,
  onSelect,
  ...props
}: VersionItemProps) {
  const body = (
    <>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm-plus font-medium text-text">
            {version}
          </span>
          {current && (
            <span className="text-2xs uppercase tracking-wider text-accent">
              Current
            </span>
          )}
        </div>
        {notes && <div className="mt-0.5 text-text-2">{notes}</div>}
      </div>
      <div className="flex flex-col items-end gap-0.5 text-right">
        <span className="font-mono text-xs tabular-nums text-text-3">
          {date}
        </span>
        {meta && (
          <span className="font-mono text-xs tabular-nums text-text-3">
            {meta}
          </span>
        )}
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
          "cursor-pointer hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
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
