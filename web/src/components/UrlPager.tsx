import { useEffect } from "react";
import { useSearchParams } from "react-router";

import { Pager, cn } from "@/design-system";
import { DEFAULT_PAGE_SIZE, PAGE_SIZES, parsePaging, withPage } from "@/lib/libraryFilters";

/**
 * A pager whose page and page size live in the URL's `page` and `page_size`,
 * as `parsePaging` reads them. Paging pushes history entries, so back and
 * forward step through pages; a new page size starts again from page 1.
 *
 * A page past the end (a stale link, or entries gone) moves to the last
 * page once `query` has settled, so `total` is the real total for this URL,
 * not a placeholder carried over from the previous one or a failed fetch.
 *
 * It renders as the bar pinned under a page's content, the same on every
 * page: mount it as the last child of the page's flex column, after the
 * scrolling content rather than inside it.
 */
export function UrlPager({
  total,
  query,
  className,
  "aria-label": ariaLabel,
}: {
  total: number;
  query: { loading: boolean; placeholder: boolean; error: unknown };
  className?: string;
  "aria-label": string;
}) {
  const [search, setSearch] = useSearchParams();
  const settled = !query.loading && !query.placeholder && !query.error;
  const { page, pageSize } = parsePaging(search);
  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  useEffect(() => {
    if (!settled || page <= pageCount) return;
    setSearch(withPage(search, pageCount), { replace: true });
  }, [settled, page, pageCount, search, setSearch]);

  if (total === 0) return null;

  const onPageSizeChange = (size: number) => {
    const next = withPage(search, 1);
    if (size === DEFAULT_PAGE_SIZE) next.delete("page_size");
    else next.set("page_size", String(size));
    setSearch(next);
  };

  return (
    <Pager
      aria-label={ariaLabel}
      className={cn(
        "shrink-0 justify-end border-t border-border bg-background px-3.5 py-1.5",
        className,
      )}
      page={Math.min(page, pageCount)}
      pageCount={pageCount}
      onPageChange={(next) => setSearch(withPage(search, next))}
      pageSize={pageSize}
      pageSizes={PAGE_SIZES}
      onPageSizeChange={onPageSizeChange}
    />
  );
}
