import { useMemo } from "react";
import { useSearchParams } from "react-router";

import { AuditList } from "@/components/AuditList";
import { Shell } from "@/components/Shell";
import { Breadcrumb, BreadcrumbItem, SearchInput, Toolbar } from "@/design-system";
import { cn } from "@/design-system/utils";
import { useLibraryAudit } from "@/hooks/useDataQueries";

const ACTION_FILTERS = [
  { label: "All", value: "" },
  { label: "Imports", value: "asset.create" },
  { label: "Updates", value: "asset.update" },
  { label: "Reviews", value: "asset.review" },
  { label: "Licences", value: "asset.licence.update" },
  { label: "Tags", value: "asset.tag.add" },
  { label: "Projects", value: "project.update" },
];

/**
 * Library-scoped audit log. Filterable by action type via the chip bar +
 * by free-text user/target via the search input (server-side
 * filtering for action_type; the search input narrows by user_email
 * client-side because the audit endpoint doesn't accept a free-text
 * query — small enough at v1 row volumes).
 */
export function AuditPage() {
  const [search, setSearch] = useSearchParams();
  const action = search.get("action") ?? "";
  const q = search.get("q") ?? "";

  const filters = useMemo(
    () => (action ? { action_type: action, limit: 200 } : { limit: 200 }),
    [action],
  );
  const { data: entries, total, loading } = useLibraryAudit(filters);

  const filtered = useMemo(() => {
    if (!q) return entries;
    const needle = q.toLowerCase();
    return entries.filter(
      (e) =>
        (e.user_email ?? "").toLowerCase().includes(needle) ||
        (e.user_display_name ?? "").toLowerCase().includes(needle) ||
        e.action_type.toLowerCase().includes(needle),
    );
  }, [entries, q]);

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(search);
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
          <div className="flex flex-wrap items-center gap-2 border-b border-line px-3 py-2">
            {ACTION_FILTERS.map((f) => (
              <button
                key={f.value}
                type="button"
                onClick={() => setParam("action", f.value)}
                className={cn(
                  "h-[22px] rounded-pill border px-2.5 text-small transition-colors duration-std ease",
                  action === f.value
                    ? "border-accent bg-accent-tint text-text"
                    : "border-line bg-panel text-text-2 hover:text-text",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
          <div className="mx-auto w-full max-w-[960px] p-3">
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-ui text-text-3">
                Loading…
              </div>
            ) : (
              <AuditList entries={filtered} linkTargets />
            )}
          </div>
        </>
      }
    />
  );
}
