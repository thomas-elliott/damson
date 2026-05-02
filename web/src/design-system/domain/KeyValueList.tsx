import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface KeyValueEntry {
  key: ReactNode;
  value: ReactNode;
  /** Use mono font for the value (hashes, paths, SPDX, sizes). */
  mono?: boolean;
}

export interface KeyValueListProps extends HTMLAttributes<HTMLElement> {
  entries: KeyValueEntry[];
  /** Width of the label column (CSS value). Defaults to 90px. */
  labelWidth?: string;
}

/**
 * Two-column definition list. Labels on the left in text-3, values on the
 * right in text-2. Matches the mockup's `.kv` pattern used in detail panels.
 */
export function KeyValueList({
  className,
  entries,
  labelWidth = "90px",
  ...props
}: KeyValueListProps) {
  return (
    <dl
      className={cn("grid gap-x-3 gap-y-1.5 text-small", className)}
      style={{ gridTemplateColumns: `${labelWidth} 1fr` }}
      {...props}
    >
      {entries.map((entry, i) => (
        <div key={i} className="contents">
          <dt className="text-text-3">{entry.key}</dt>
          <dd className={cn("m-0 break-words text-text-2", entry.mono && "font-mono")}>
            {entry.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
