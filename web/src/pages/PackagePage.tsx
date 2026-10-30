import { useCallback, useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { isNotFound } from "@/api";
import { FileBrowser } from "@/components/filebrowser/FileBrowser";
import { PackageSummary } from "@/components/detail/PackageSummary";
import { Shell } from "@/components/Shell";
import { Button, EmptyState, ErrorState, IconButton, LoadingState, Toolbar } from "@/design-system";
import { useAssetDetail, useAssets, useMe } from "@/hooks/useDataQueries";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { parseAssetFilters, parsePaging, withPage } from "@/lib/libraryFilters";
import { edges, packageNav, type Neighbour, type PackageNav } from "@/lib/packageNav";

/**
 * The full page for one asset: everything about it on the left, its files on
 * the right, stacked on a narrow screen. A reference asset has no files, so it
 * gets the left column alone. The URL query is the library's, so next and
 * previous walk the results the asset was opened from and Back returns to them.
 */
export function PackagePage() {
  const params = useParams();
  const assetId = Number(params.assetId);
  const [search] = useSearchParams();
  const navigate = useNavigate();
  const { data: me } = useMe();
  const { data: asset, loading, error, failed, reload } = useAssetDetail(assetId);
  const nav = usePackageNav(assetId, search, !!me?.current_library);

  const query = search.toString();
  const toLibrary = useCallback(
    () => navigate(`/library/${assetId}${query ? `?${query}` : ""}`),
    [navigate, assetId, query],
  );
  const toNeighbour = useCallback(
    (n: Neighbour | null) => {
      if (!n) return;
      const q = withPage(search, n.page).toString();
      navigate(`/library/${n.id}/package${q ? `?${q}` : ""}`);
    },
    [navigate, search],
  );
  const step = useCallback(
    (delta: -1 | 1) => toNeighbour(delta < 0 ? nav.prev : nav.next),
    [toNeighbour, nav.prev, nav.next],
  );

  // The file browser handles the keys itself, so a reference asset's page,
  // which has none, binds them here.
  const isReference = !!asset?.is_reference;
  useKeyboardShortcuts(
    useMemo(
      () =>
        isReference
          ? [
              { key: "ArrowLeft", handler: () => step(-1) },
              { key: "ArrowRight", handler: () => step(1) },
              { key: "Escape", handler: toLibrary },
            ]
          : [],
      [isReference, step, toLibrary],
    ),
  );

  if (!me) return null;

  return (
    <Shell
      main={
        <div className="flex h-full min-h-0 flex-col">
          <PackageHeader
            name={asset?.name ?? (loading ? "Loading…" : "Asset")}
            nav={nav}
            onBack={toLibrary}
            onStep={step}
          />
          {asset ? (
            asset.is_reference ? (
              <div className="min-h-0 flex-1 overflow-y-auto">
                <div className="mx-auto w-full max-w-2xl">
                  <PackageSummary key={asset.id} asset={asset} onDeleted={toLibrary} />
                </div>
              </div>
            ) : (
              // Side by side from lg; below it the page scrolls as one column
              // and the files take most of a screen of their own.
              <div className="flex min-h-0 flex-1 flex-col overflow-y-auto lg:flex-row lg:overflow-hidden">
                <div className="shrink-0 border-border bg-card lg:w-[380px] lg:overflow-y-auto lg:border-r">
                  <PackageSummary key={asset.id} asset={asset} onDeleted={toLibrary} />
                </div>
                <section
                  aria-label="Files"
                  className="h-[85dvh] shrink-0 border-t border-border lg:h-auto lg:min-w-0 lg:flex-1 lg:border-t-0"
                >
                  <FileBrowser
                    key={asset.id}
                    assetId={asset.id}
                    onExit={toLibrary}
                    onStepPackage={step}
                  />
                </section>
              </div>
            )
          ) : failed && isNotFound(error) ? (
            <EmptyState title="Asset not found" description="It may have been deleted." />
          ) : failed ? (
            <ErrorState title="Couldn't load this asset" onRetry={reload} />
          ) : (
            <LoadingState className="flex-1" />
          )}
        </div>
      }
    />
  );
}

function PackageHeader({
  name,
  nav,
  onBack,
  onStep,
}: {
  name: string;
  nav: PackageNav;
  onBack: () => void;
  onStep: (delta: -1 | 1) => void;
}) {
  return (
    <Toolbar>
      <Button variant="ghost" size="sm" leadingIcon={<ChevronLeft size={12} />} onClick={onBack}>
        Library
      </Button>
      <h1 className="min-w-0 flex-1 truncate text-subhead-semibold text-foreground">{name}</h1>
      {nav.position != null && (
        <div className="flex items-center gap-1">
          <span className="text-label tabular-nums text-subtle-foreground">
            {nav.position} of {nav.total}
          </span>
          <IconButton
            aria-label="Previous package"
            title="Previous package (←)"
            size="sm"
            disabled={!nav.prev}
            onClick={() => onStep(-1)}
          >
            <ChevronLeft />
          </IconButton>
          <IconButton
            aria-label="Next package"
            title="Next package (→)"
            size="sm"
            disabled={!nav.next}
            onClick={() => onStep(1)}
          >
            <ChevronRight />
          </IconButton>
        </div>
      )}
    </Toolbar>
  );
}

// usePackageNav reads the library page the asset was opened from, and the
// page either side of it only while the asset sits on that edge. The queries
// share the library page's cache keys, so arriving from the library costs no
// request.
function usePackageNav(assetId: number, search: URLSearchParams, enabled: boolean): PackageNav {
  const filters = useMemo(() => parseAssetFilters(search), [search]);
  const { page, pageSize } = useMemo(() => parsePaging(search), [search]);
  const offset = (page - 1) * pageSize;

  const current = useAssets(filters, enabled, { limit: pageSize, offset });
  // A placeholder is the last page's rows while this one loads: not this page.
  const currentIds = settledIds(current);
  const { needsBefore, needsAfter } = edges(assetId, page, pageSize, current.total, currentIds);
  const before = useAssets(filters, enabled && needsBefore, {
    limit: pageSize,
    offset: offset - pageSize,
  });
  const after = useAssets(filters, enabled && needsAfter, {
    limit: pageSize,
    offset: offset + pageSize,
  });

  return packageNav({
    assetId,
    page,
    pageSize,
    total: current.total,
    current: currentIds,
    before: settledIds(before),
    after: settledIds(after),
  });
}

function settledIds(q: { data: { id: number }[]; loading: boolean; placeholder: boolean }) {
  return q.loading || q.placeholder ? null : q.data.map((a) => a.id);
}
