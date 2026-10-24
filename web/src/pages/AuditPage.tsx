import { useMemo } from "react";
import { useSearchParams } from "react-router";

import { AuditList } from "@/components/AuditList";
import { Shell } from "@/components/Shell";
import { UrlPager } from "@/components/UrlPager";
import { Breadcrumb, BreadcrumbItem, SearchInput, Toolbar } from "@/design-system";
import { cn } from "@/design-system/utils";
import { useLibraryAudit } from "@/hooks/useDataQueries";
import { ACTION_FILTERS } from "@/lib/auditFilters";
import { parsePaging, withPage } from "@/lib/libraryFilters";

/**
 * Library-scoped audit log, paged. The chip bar filters by action type and
 * the search input by user name, email or action type, both on the server;
 * changing either starts again from page 1.
 */
export function AuditPage() {
  const [search, setSearch] = useSearchParams();
  const action = search.get("action") ?? "";
  const q = search.get("q") ?? "";
  const { page, pageSize } = useMemo(() => parsePaging(search), [search]);

  const {
    data: entries,
    total,
    loading,
    placeholder,
    error,
  } = useLibraryAudit({
    action_type: action || undefined,
    q: q || undefined,
    limit: pageSize,
    offset: (page - 1) * pageSize,
  });

  const setParam = (key: string, value: string) => {
    const next = withPage(search, 1);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearch(next, { replace: true });
  };

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={loading ? "…" : `${total}`}>
              <BreadcrumbItem current>Audit log</BreadcrumbItem>
            </Breadcrumb>
            <div className="ml-auto flex items-center gap-2">
              <SearchInput
                value={q}
                onChange={(e) => setParam("q", e.target.value)}
                placeholder="Filter by user…"
                aria-label="Filter audit log by user"
              />
            </div>
          </Toolbar>
          <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
            {ACTION_FILTERS.map((f) => (
              <button
                key={f.value}
                type="button"
                onClick={() => setParam("action", f.value)}
                className={cn(
                  "h-[22px] rounded-full border px-2.5 text-label transition-colors duration-fast ease-standard",
                  action === f.value
                    ? "border-primary bg-primary-soft text-foreground"
                    : "border-border bg-card text-muted-foreground hover:text-foreground",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
          <div className="mx-auto w-full max-w-[960px] p-3">
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-ui text-subtle-foreground">
                Loading…
              </div>
            ) : (
              <AuditList entries={entries} linkTargets />
            )}
            <div className="mt-2 flex justify-end">
              <UrlPager
                aria-label="Audit log pages"
                total={total}
                settled={!loading && !placeholder && !error}
              />
            </div>
          </div>
        </>
      }
    />
  );
}
