import { useEffect, useState } from "react";

import { SearchInput, cn } from "@/design-system";
import { useAssets, type AssetRow } from "@/hooks/useDataQueries";

export const PICKER_PAGE_SIZE = 25;
const SEARCH_DELAY_MS = 250;

/**
 * Finds an asset anywhere in the library by searching the server as the user
 * types. It lists the first page of matches; when there are more, it says so
 * and asks for a narrower search rather than paging.
 */
export function AssetPicker({
  selectedId,
  onSelect,
  excludeId,
  placeholder = "Search assets",
  "aria-label": ariaLabel,
}: {
  selectedId: number | null;
  onSelect: (asset: AssetRow) => void;
  /** An asset never offered, such as the merge target. */
  excludeId?: number;
  placeholder?: string;
  "aria-label": string;
}) {
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setSearch(q.trim()), SEARCH_DELAY_MS);
    return () => clearTimeout(t);
  }, [q]);

  const {
    data: assets,
    total,
    loading,
    placeholder: stale,
  } = useAssets({ q: search || undefined }, true, { limit: PICKER_PAGE_SIZE, offset: 0 });
  const candidates = assets.filter((a) => a.id !== excludeId);

  const rows = () => {
    if (loading || stale)
      return <li className="px-2 py-3 text-label text-faint-foreground">Searching…</li>;
    if (candidates.length === 0)
      return <li className="px-2 py-3 text-label text-faint-foreground">No matching assets</li>;
    return candidates.map((a) => (
      <li key={a.id}>
        <button
          type="button"
          aria-pressed={selectedId === a.id}
          onClick={() => onSelect(a)}
          className={cn(
            "w-full truncate rounded-md px-2 py-1.5 text-left text-label text-muted-foreground transition-colors duration-fast ease-standard hover:bg-muted",
            selectedId === a.id && "bg-muted text-foreground",
          )}
        >
          {a.name}
        </button>
      </li>
    ));
  };

  return (
    <div className="flex flex-col gap-1.5">
      <SearchInput
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder={placeholder}
        aria-label={ariaLabel}
        autoFocus
      />
      <ul
        aria-label={`${ariaLabel} results`}
        className="flex max-h-64 flex-col gap-0.5 overflow-y-auto"
      >
        {rows()}
      </ul>
      {!loading && !stale && total > assets.length && (
        <p className="text-label text-subtle-foreground">
          Showing the first {assets.length} of {total} matches. Type more of the name to narrow the
          search.
        </p>
      )}
    </div>
  );
}
