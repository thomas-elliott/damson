import { useEffect } from "react";
import { useSearchParams } from "react-router";

import { Pager } from "@/design-system";
import { DEFAULT_PAGE_SIZE, PAGE_SIZES, parsePaging, withPage } from "@/lib/libraryFilters";

/**
 * A pager whose page and page size live in the URL's `page` and `page_size`,
 * as `parsePaging` reads them. Paging pushes history entries, so back and
 * forward step through pages; a new page size starts again from page 1.
 *
 * A page past the end (a stale link, or entries gone) moves to the last
 * page once `settled` says `total` is the real total for this URL, not a
 * placeholder carried over from the previous one.
 */
export function UrlPager({
  total,
  settled,
  "aria-label": ariaLabel,
}: {
  total: number;
  settled: boolean;
  "aria-label": string;
}) {
  const [search, setSearch] = useSearchParams();
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
      page={Math.min(page, pageCount)}
      pageCount={pageCount}
      onPageChange={(next) => setSearch(withPage(search, next))}
      pageSize={pageSize}
      pageSizes={PAGE_SIZES}
      onPageSizeChange={onPageSizeChange}
    />
  );
}
