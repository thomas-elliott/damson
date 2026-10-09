import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { SlidersHorizontal, UploadCloud } from "lucide-react";

import { AddByLocationButton } from "@/components/AddByLocationDialog";
import { AssetGrid } from "@/components/AssetGrid";
import { BulkAssetBar } from "@/components/BulkAssetBar";
import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";
import { DropZone, useDropZoneOpen } from "@/components/DropZone";
import { FileBrowser } from "@/components/filebrowser/FileBrowser";
import { LibraryFilterSidebar } from "@/components/LibraryFilterSidebar";
import { LibraryPicker } from "@/components/LibraryPicker";
import { Shell } from "@/components/Shell";
import {
  Badge,
  Button,
  EmptyState,
  SearchInput,
  Toolbar,
  ToolbarSpacer,
  isBrowsable,
} from "@/design-system";
import { FolderOpen, Library as LibraryIcon } from "lucide-react";
import { AssetSortMenu, DEFAULT_ASSET_SORT } from "@/components/AssetSortMenu";
import { useAssets, useMe, type AssetFilters, type AssetRow } from "@/hooks/useDataQueries";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  activeFilterCount,
  parseAssetFilters,
  readFiltersOpen,
  writeFiltersOpen,
} from "@/lib/libraryFilters";

export function LibraryPage({ browse = false }: { browse?: boolean } = {}) {
  const { data: me } = useMe();
  const params = useParams();
  const navigate = useNavigate();
  const [search, setSearch] = useSearchParams();
  const hasLibrary = !!me?.current_library;

  const assetId = params.assetId ? Number(params.assetId) : null;
  const browsing = browse && assetId != null;

  const filters: AssetFilters = useMemo(() => parseAssetFilters(search), [search]);
  const filterCount = activeFilterCount(filters);

  const [filtersOpen, setFiltersOpen] = useState(readFiltersOpen);
  const toggleFilters = () => {
    const next = !filtersOpen;
    setFiltersOpen(next);
    writeFiltersOpen(next);
  };

  const { data: assets, total, loading } = useAssets(filters, hasLibrary);

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
    const params = new URLSearchParams(search);
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
  // swallow `/` from the file search.
  const focusSearch = (e: KeyboardEvent) => {
    if (stateRef.current.browsing) return;
    e.preventDefault();
    searchInputRef.current?.focus();
  };
  const moveSelection = (delta: number) => {
    const s = stateRef.current;
    if (s.browsing) return;
    moveByOffset(s.assets, s.assetId, delta, s.onOpenAsset);
  };
  useKeyboardShortcuts(
    useMemo(
      () => [
        { key: "/", handler: focusSearch },
        { key: "k", meta: true, handler: focusSearch },
        { key: "k", ctrl: true, handler: focusSearch },
        {
          key: "Escape",
          handler: () => {
            const s = stateRef.current;
            if (s.browsing) return;
            if (s.assetId != null) s.onCloseDetail();
          },
        },
        {
          key: "ArrowRight",
          handler: (e) => {
            e.preventDefault();
            moveSelection(1);
          },
        },
        {
          key: "ArrowLeft",
          handler: (e) => {
            e.preventDefault();
            moveSelection(-1);
          },
        },
        {
          key: "ArrowDown",
          handler: (e) => {
            e.preventDefault();
            moveSelection(columnsRef.current);
          },
        },
        {
          key: "ArrowUp",
          handler: (e) => {
            e.preventDefault();
            moveSelection(-columnsRef.current);
          },
        },
        {
          key: "Enter",
          handler: () => {
            const s = stateRef.current;
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
      sidebar={lib && filtersOpen ? <LibraryFilterSidebar filters={filters} /> : undefined}
      main={
        lib ? (
          <DropZone>
            <Toolbar>
              <LibraryPicker current={lib} />
              <span className="shrink-0 text-label tabular-nums text-subtle-foreground">
                {total} assets
              </span>
              <Button
                variant="secondary"
                pressed={filtersOpen}
                aria-expanded={filtersOpen}
                leadingIcon={<SlidersHorizontal size={12} />}
                onClick={toggleFilters}
                className={filterCount > 0 ? "border-primary/50 text-foreground" : undefined}
              >
                Filter
                {filterCount > 0 && (
                  <Badge tone="accent" shape="pill">
                    {filterCount}
                  </Badge>
                )}
              </Button>
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
                  const next = new URLSearchParams(search);
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
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-ui text-subtle-foreground">
                Loading assets…
              </div>
            ) : (
              <AssetGrid
                assets={assets}
                onOpen={onOpenAsset}
                selectedId={assetId}
                selectedIds={selectedIds}
                onToggleSelect={toggleSelect}
                onColumnsChange={onColumnsChange}
              />
            )}
            {selectedList.length > 0 && (
              <BulkAssetBar assetIds={selectedList} onClear={clearSelection} />
            )}
            <AssetDetailPanel assetId={assetId} onClose={onCloseDetail} onBrowse={onBrowse} />
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
