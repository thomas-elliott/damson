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
 * `.activity-row` so columns line up across many rows.
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
        "grid items-baseline gap-3 border-b border-line px-3.5 py-2 text-xs-plus last:border-b-0",
        className,
      )}
      style={{ gridTemplateColumns: "96px 140px 1fr auto" }}
      {...props}
    >
      <span className="font-mono text-xs tabular-nums text-text-3">
        {timestamp}
      </span>
      <span className="truncate text-text-2">{actor}</span>
      <span className="truncate text-text-2">
        <span className="text-text-3">{action}</span>
        {target && (
          <>
            {" "}
            <span className="font-medium text-text">{target}</span>
          </>
        )}
      </span>
    </div>
  );
}
