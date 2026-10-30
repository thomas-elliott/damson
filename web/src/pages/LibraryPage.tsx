import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { SearchX, UploadCloud } from "lucide-react";

import { ActiveFilterChips } from "@/components/ActiveFilterChips";
import { AddByLocationButton } from "@/components/AddByLocationDialog";
import { AssetGrid } from "@/components/AssetGrid";
import { BulkAssetBar } from "@/components/BulkAssetBar";
import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";
import { DropZone, useDropZoneOpen } from "@/components/DropZone";
import { FileBrowser } from "@/components/filebrowser/FileBrowser";
import { LibraryFiltersButton } from "@/components/LibraryFilters";
import { LibraryPicker } from "@/components/LibraryPicker";
import { LibraryTagsButton } from "@/components/LibraryTagsButton";
import { Shell } from "@/components/Shell";
import { UrlPager } from "@/components/UrlPager";
import {
  Button,
  EmptyState,
  ErrorState,
  isBrowsable,
  LoadingState,
  SearchInput,
  Toolbar,
  ToolbarSpacer,
} from "@/design-system";
import { FolderOpen, Library as LibraryIcon } from "lucide-react";
import { AssetSortMenu, DEFAULT_ASSET_SORT } from "@/components/AssetSortMenu";
import {
  useAssets,
  useLibrarySettings,
  useMe,
  type AssetFilters,
  type AssetRow,
} from "@/hooks/useDataQueries";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  activeFilterCount,
  parseAssetFilters,
  parsePaging,
  withPage,
  withoutFilters,
  withoutPaging,
} from "@/lib/libraryFilters";

export function LibraryPage({ browse = false }: { browse?: boolean } = {}) {
  const { data: me } = useMe();
  const { data: settings, loading: settingsLoading } = useLibrarySettings(me?.current_library?.id);
  const params = useParams();
  const navigate = useNavigate();
  const [search, setSearch] = useSearchParams();
  const hasLibrary = !!me?.current_library;

  const assetId = params.assetId ? Number(params.assetId) : null;
  const browsing = browse && assetId != null;

  const filters: AssetFilters = useMemo(() => parseAssetFilters(search), [search]);
  const tagPaths = useMemo(() => filters.tag_path ?? [], [filters.tag_path]);
  // Filtered at all, search included: an empty result is then "no match",
  // not an empty library.
  const narrowed = activeFilterCount(filters) > 0 || tagPaths.length > 0 || !!filters.q;
  const [tagsOpen, setTagsOpen] = useState(false);
  // The detail panel floats over the grid; the grid pads itself out from under
  // it so its cards reflow beside the panel.
  const [panelWidth, setPanelWidth] = useState(0);

  const { page, pageSize } = useMemo(() => parsePaging(search), [search]);
  const {
    data: assets,
    total,
    loading,
    placeholder,
    error,
    failed,
    reload,
  } = useAssets(filters, hasLibrary, { limit: pageSize, offset: (page - 1) * pageSize });

  // The last settled total for this search and filter, so a failed page
  // change still shows the pager.
  const totalScope = withoutPaging(search);
  const [knownTotal, setKnownTotal] = useState({ scope: totalScope, total: 0 });
  const settledTotal = !loading && !placeholder && !failed ? total : null;
  if (
    settledTotal != null &&
    (knownTotal.scope !== totalScope || knownTotal.total !== settledTotal)
  ) {
    setKnownTotal({ scope: totalScope, total: settledTotal });
  }
  const pagerTotal = failed ? (knownTotal.scope === totalScope ? knownTotal.total : 0) : total;

  const [selectedIds, setSelectedIds] = useState<Set<number>>(() => new Set());
  // Reset the selection when the search/filter scope changes — a render-time
  // reset rather than an effect so it lands before the grid paints.
  const scopeKey = search.toString();
  const [prevScopeKey, setPrevScopeKey] = useState(scopeKey);
  if (prevScopeKey !== scopeKey) {
    setPrevScopeKey(scopeKey);
    setSelectedIds(new Set());
  }
  const toggleSelect = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);
  const clearSelection = useCallback(() => setSelectedIds(new Set()), []);
  const selectedList = useMemo(
    () => assets.filter((a) => selectedIds.has(a.id)).map((a) => a.id),
    [assets, selectedIds],
  );

  const onSearchChange = (next: string) => {
    const params = withPage(search, 1);
    if (next) params.set("q", next);
    else params.delete("q");
    setSearch(params, { replace: true });
  };

  const suffix = search.toString() ? `?${search}` : "";
  const onOpenAsset = (id: number) => {
    navigate(`/library/${id}${suffix}`);
  };
  const onCloseDetail = () => {
    navigate(`/library${suffix}`);
  };
  const onBrowse = (id: number) => {
    navigate(`/library/${id}/browse${suffix}`);
  };

  const searchInputRef = useRef<HTMLInputElement>(null);

  // Per UI §5.3: `/` and `cmd+k` focus the library search; arrow keys move
  // selection in the grid; enter opens the highlighted asset; escape closes
  // the panel. We keep the bindings array stable across renders by reading
  // current state through refs inside the handlers — re-creating the array
  // every time `assets` refetches caused useKeyboardShortcuts to rebind on
  // every paint.
  const stateRef = useRef({ assets, assetId, onOpenAsset, onCloseDetail, browsing });
  // The grid reports how many cards a row shows; Arrow Up/Down move by it.
  const columnsRef = useRef(1);
  const onColumnsChange = useCallback((columns: number) => {
    columnsRef.current = columns;
  }, []);
  useEffect(() => {
    stateRef.current = { assets, assetId, onOpenAsset, onCloseDetail, browsing };
  });

  // In browse mode the FileBrowser owns the keyboard (Esc/arrows); the grid's
  // shortcuts must stand down so they don't fight over Escape/arrow keys or
  // swallow `/` from the file search. So must they for a key that went to a
  // popover or dialog (the tag picker, a menu): its Enter, Escape and arrows
  // are its own.
  const standDown = (e: KeyboardEvent) => stateRef.current.browsing || inOverlay(e);
  const focusSearch = (e: KeyboardEvent) => {
    if (standDown(e)) return;
    e.preventDefault();
    searchInputRef.current?.focus();
  };
  const moveSelection = (e: KeyboardEvent, delta: () => number) => {
    if (standDown(e)) return;
    e.preventDefault();
    const s = stateRef.current;
    moveByOffset(s.assets, s.assetId, delta(), s.onOpenAsset);
  };
  useKeyboardShortcuts(
    useMemo(
      () => [
        { key: "/", handler: focusSearch },
        { key: "k", meta: true, handler: focusSearch },
        { key: "k", ctrl: true, handler: focusSearch },
        {
          key: "t",
          handler: (e) => {
            if (standDown(e)) return;
            e.preventDefault();
            setTagsOpen(true);
          },
        },
        {
          key: "Escape",
          handler: (e) => {
            const s = stateRef.current;
            if (standDown(e)) return;
            if (s.assetId != null) s.onCloseDetail();
          },
        },
        { key: "ArrowRight", handler: (e) => moveSelection(e, () => 1) },
        { key: "ArrowLeft", handler: (e) => moveSelection(e, () => -1) },
        { key: "ArrowDown", handler: (e) => moveSelection(e, () => columnsRef.current) },
        { key: "ArrowUp", handler: (e) => moveSelection(e, () => -columnsRef.current) },
        {
          key: "Enter",
          handler: (e) => {
            const s = stateRef.current;
            if (standDown(e)) return;
            if (s.assetId == null && s.assets[0]) s.onOpenAsset(s.assets[0].id);
          },
        },
      ],
      [],
    ),
  );

  if (!me) return null;

  if (browse && assetId != null) {
    return <Shell main={<FileBrowser assetId={assetId} onExit={onCloseDetail} />} />;
  }

  const selectedAsset = assetId != null ? assets.find((a) => a.id === assetId) : undefined;
  const canBrowseSelected = !!selectedAsset && isBrowsable(selectedAsset.file_count);

  const lib = me.current_library;

  return (
    <Shell
      main={
        lib ? (
          <DropZone>
            <Toolbar>
              {/* The library picker names the page visually. */}
              <h1 className="sr-only">Library</h1>
              <LibraryPicker current={lib} />
              {/* Until the first page settles the hook's total is 0, which is
                  wrong rather than absent; show the pager's total otherwise, so
                  a failed page change keeps the last count. */}
              <span className="shrink-0 text-label tabular-nums text-subtle-foreground">
                {loading ? "…" : pagerTotal} assets
              </span>
              <LibraryTagsButton selected={tagPaths} open={tagsOpen} onOpenChange={setTagsOpen} />
              <LibraryFiltersButton filters={filters} />
              <div className="min-w-[220px] max-w-[360px] flex-1">
                <SearchInput
                  ref={searchInputRef}
                  value={search.get("q") ?? ""}
                  onChange={(e) => onSearchChange(e.currentTarget.value)}
                  placeholder="Search assets ( / )"
                  aria-label="Search assets"
                />
              </div>
              <AssetSortMenu
                value={filters.sort}
                onChange={(sort) => {
                  const next = withPage(search, 1);
                  if (sort === DEFAULT_ASSET_SORT) next.delete("sort");
                  else next.set("sort", sort);
                  setSearch(next, { replace: true });
                }}
              />
              <ToolbarSpacer />
              {canBrowseSelected && assetId != null && (
                <Button
                  variant="secondary"
                  leadingIcon={<FolderOpen size={12} />}
                  onClick={() => onBrowse(assetId)}
                >
                  Browse files
                </Button>
              )}
              <AddByLocationButton onOpenAsset={onOpenAsset} />
              <UploadButton />
            </Toolbar>
            <ActiveFilterChips filters={filters} />
            {/* The selection bar and the detail panel float over the grid's
                area, not the whole column, so the bar clears the pager and the
                panel leaves the toolbar's actions uncovered. */}
            <div className="relative flex min-h-0 flex-1 flex-col">
              {/* Assets wait for the settings so the grid first draws them at the
                library's density; if the settings fail it draws at medium. The
                error and the empty library need no density, so they do not wait. */}
              {loading || (settingsLoading && !failed && assets.length > 0) ? (
                <LoadingState />
              ) : failed ? (
                <ErrorState className="flex-1" title="Couldn't load assets" onRetry={reload} />
              ) : assets.length === 0 && narrowed ? (
                <EmptyState
                  className="flex-1"
                  icon={<SearchX size={28} />}
                  title="No assets match"
                  description="Nothing in this library matches the search, tags and filters together. Loosen one, or clear them."
                  action={
                    <Button
                      variant="secondary"
                      onClick={() => {
                        const next = withoutFilters(search);
                        next.delete("q");
                        setSearch(next, { replace: true });
                      }}
                    >
                      Clear search and filters
                    </Button>
                  }
                />
              ) : (
                <AssetGrid
                  // A new page starts scrolled to the top.
                  key={`${page}/${pageSize}`}
                  reservedRight={panelWidth}
                  assets={assets}
                  onOpen={onOpenAsset}
                  selectedId={assetId}
                  selectedIds={selectedIds}
                  onToggleSelect={toggleSelect}
                  onColumnsChange={onColumnsChange}
                  density={settings?.grid_density_default}
                />
              )}
              {selectedList.length > 0 && (
                <BulkAssetBar assetIds={selectedList} onClear={clearSelection} />
              )}
              <AssetDetailPanel
                assetId={assetId}
                onClose={onCloseDetail}
                onBrowse={onBrowse}
                onReservedWidthChange={setPanelWidth}
              />
            </div>
            {!loading && (
              <UrlPager
                aria-label="Library pages"
                total={pagerTotal}
                query={{ loading, placeholder, error }}
              />
            )}
          </DropZone>
        ) : (
          <EmptyState
            icon={<LibraryIcon size={32} />}
            title="No library access yet"
            description={
              <>
                You're signed in as{" "}
                <span className="font-mono text-muted-foreground">{me.email}</span> but you haven't
                been added to a library. Ask a Damson administrator to invite you.
              </>
            }
          />
        )
      }
    />
  );
}

// inOverlay reports whether a key went to something layered over the page.
function inOverlay(e: KeyboardEvent) {
  const target = e.target as Element | null;
  return !!target?.closest?.('[role="dialog"], [role="menu"], [role="listbox"]');
}

// moveByOffset shifts the highlighted asset by `delta` and pushes the new id
// through the supplied open handler so the URL routing stays the source of
// truth. Bounded so wrap-around doesn't accidentally jump from the start
// to the end.
function moveByOffset(
  assets: AssetRow[],
  selected: number | null,
  delta: number,
  open: (id: number) => void,
) {
  if (assets.length === 0) return;
  const i = selected != null ? assets.findIndex((a) => a.id === selected) : -1;
  let next = i < 0 ? 0 : i + delta;
  if (next < 0) next = 0;
  if (next >= assets.length) next = assets.length - 1;
  const target = assets[next];
  if (target) open(target.id);
}

function UploadButton() {
  const dz = useDropZoneOpen();
  return (
    <Button
      variant="primary"
      leadingIcon={<UploadCloud size={12} />}
      onClick={() => dz?.open()}
      disabled={!dz}
    >
      Upload
    </Button>
  );
}
