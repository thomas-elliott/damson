import { type HTMLAttributes } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/design-system/primitives/Select";
import { cn } from "@/design-system/utils";

export interface PagerProps extends Omit<HTMLAttributes<HTMLElement>, "onChange"> {
  /** 1-based. */
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
  /** The page size selector shows only when both of these and `pageSizes` are set. */
  pageSize?: number;
  pageSizes?: readonly number[];
  onPageSizeChange?: (size: number) => void;
}

/**
 * Prev / next with page numbers, and an optional page size selector. The
 * numbers show the first and last page and a window around the current one;
 * the current page carries `aria-current="page"`.
 */
export function Pager({
  page,
  pageCount,
  onPageChange,
  pageSize,
  pageSizes,
  onPageSizeChange,
  className,
  "aria-label": ariaLabel = "Pagination",
  ...props
}: PagerProps) {
  const count = Math.max(1, pageCount);
  return (
    <nav
      aria-label={ariaLabel}
      className={cn("flex items-center gap-1 text-label text-subtle-foreground", className)}
      {...props}
    >
      <IconButton
        aria-label="Previous page"
        size="sm"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft />
      </IconButton>
      <ul className="flex items-center gap-0.5">
        {pageItems(page, count).map((item, i) =>
          item === "gap" ? (
            <li key={`gap-${i}`} aria-hidden className="px-1">
              …
            </li>
          ) : (
            <li key={item}>
              <Button
                variant={item === page ? "secondary" : "ghost"}
                size="sm"
                className="min-w-7 px-1.5 tabular-nums"
                aria-label={`Page ${item}`}
                aria-current={item === page ? "page" : undefined}
                data-active={item === page ? "true" : undefined}
                onClick={() => onPageChange(item)}
              >
                {item}
              </Button>
            </li>
          ),
        )}
      </ul>
      <IconButton
        aria-label="Next page"
        size="sm"
        disabled={page >= count}
        onClick={() => onPageChange(page + 1)}
      >
        <ChevronRight />
      </IconButton>
      <span className="ml-1.5 tabular-nums">
        Page {page} of {count}
      </span>
      {pageSize != null && pageSizes && onPageSizeChange && (
        <Select value={String(pageSize)} onValueChange={(v) => onPageSizeChange(Number(v))}>
          <SelectTrigger aria-label="Page size" className="ml-2.5">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {pageSizes.map((s) => (
              <SelectItem key={s} value={String(s)}>
                {s} per page
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </nav>
  );
}

// pageItems lists the page numbers to show: the first, the last, and one
// either side of the current page, with a gap wherever pages are skipped. A
// gap that would hide a single page shows that page instead.
function pageItems(page: number, count: number): (number | "gap")[] {
  const shown = [...new Set([1, page - 1, page, page + 1, count])]
    .filter((n) => n >= 1 && n <= count)
    .sort((a, b) => a - b);
  const out: (number | "gap")[] = [];
  let prev = 0;
  for (const n of shown) {
    if (n - prev === 2) out.push(prev + 1);
    else if (n - prev > 2) out.push("gap");
    out.push(n);
    prev = n;
  }
  return out;
}
