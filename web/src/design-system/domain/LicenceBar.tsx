import { type HTMLAttributes } from "react";

import { cn } from "@/design-system/utils";
import type { LicenceStatus } from "@/design-system/tokens";

export interface LicenceCounts {
  ok: number;
  warn: number;
  err: number;
  unk: number;
}

export interface LicenceBarProps extends HTMLAttributes<HTMLDivElement> {
  counts: LicenceCounts;
  /** Visual height of the bar. */
  size?: "sm" | "md";
  /** Render an accessible label summarising the split. */
  "aria-label"?: string;
}

const SEG_COLOR: Record<LicenceStatus, string> = {
  ok: "bg-ok",
  warn: "bg-warn",
  err: "bg-err",
  unk: "bg-unk",
};

/**
 * Stacked proportional bar — one segment per licence status. Used in
 * project/library headers to visualise licence compliance at a glance.
 * Order: ok, warn, err, unk (matches mockup .lic-bar).
 */
export function LicenceBar({
  className,
  counts,
  size = "sm",
  "aria-label": ariaLabel,
  ...props
}: LicenceBarProps) {
  const total = counts.ok + counts.warn + counts.err + counts.unk;
  const segs: Array<{ status: LicenceStatus; count: number }> = [
    { status: "ok", count: counts.ok },
    { status: "warn", count: counts.warn },
    { status: "err", count: counts.err },
    { status: "unk", count: counts.unk },
  ];
  return (
    <div
      role="img"
      aria-label={
        ariaLabel ??
        `${counts.ok} compliant, ${counts.warn} review, ${counts.err} violation, ${counts.unk} unknown`
      }
      className={cn(
        "flex w-full overflow-hidden rounded-full bg-line",
        size === "sm" ? "h-[3px]" : "h-1",
        className,
      )}
      {...props}
    >
      {total > 0 &&
        segs.map(
          (seg) =>
            seg.count > 0 && (
              <div
                key={seg.status}
                className={cn("h-full", SEG_COLOR[seg.status])}
                style={{ width: `${(seg.count / total) * 100}%` }}
              />
            ),
        )}
    </div>
  );
}
