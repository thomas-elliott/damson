import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface ActivityRowProps extends HTMLAttributes<HTMLDivElement> {
  /** ISO or display timestamp. */
  timestamp: ReactNode;
  /** Actor name / email / system. */
  actor: ReactNode;
  /** Short verb phrase ("pinned", "updated licence"). */
  action: ReactNode;
  /** Target of the action — asset name, project, etc. */
  target?: ReactNode;
}

/**
 * Audit-log / activity timeline row. Grid layout matches the mockup's
 * `.activity-row` so columns line up across many rows. The timestamp never
 * wraps: its track is 8rem, which fits `formatAuditTimestamp`'s output, and
 * grows to the content only when a locale's timestamp is longer. In a row
 * narrower than 28rem the action moves to a second line.
 */
export function ActivityRow({
  className,
  timestamp,
  actor,
  action,
  target,
  ...props
}: ActivityRowProps) {
  return (
    <div
      className={cn(
        "@container border-b border-border px-3.5 py-2 text-list-row last:border-b-0",
        className,
      )}
      {...props}
    >
      {/* Narrower than 28rem (a phone, or the detail panel) the action takes
          its own line under the timestamp and actor rather than truncating
          to nothing. */}
      <div className="grid grid-cols-[minmax(max-content,8rem)_140px_1fr] items-baseline gap-x-3 gap-y-0.5 @max-md:grid-cols-[minmax(max-content,8rem)_1fr]">
        <span className="whitespace-nowrap text-mono tabular-nums text-subtle-foreground">
          {timestamp}
        </span>
        <span className="truncate text-muted-foreground">{actor}</span>
        <span className="truncate text-muted-foreground @max-md:col-span-2">
          <span className="text-subtle-foreground">{action}</span>
          {target && (
            <>
              {" "}
              <span className="font-medium text-foreground">{target}</span>
            </>
          )}
        </span>
      </div>
    </div>
  );
}
