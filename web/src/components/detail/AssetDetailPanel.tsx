import { useEffect, useRef, useState } from "react";
import { Maximize2 } from "lucide-react";

import { isNotFound } from "@/api";
import { Button, DetailPanel, EmptyState, ErrorState } from "@/design-system";
import { useAssetDetail, type AssetDetailView } from "@/hooks/useDataQueries";

import { AssetGallery } from "./AssetGallery";
import { AssetStatusChips, GetItSection, TagsSection } from "./AssetSections";
import { ClampedDescription, LicenceSummary } from "./OverviewSection";

const WIDTH_KEY = "damson.detail.w";
const DEFAULT_WIDTH = 440;
const MIN_WIDTH = 320;
const MOBILE_QUERY = "(max-width: 640px)";

function readStoredWidth(): number {
  if (typeof window === "undefined") return DEFAULT_WIDTH;
  const raw = window.localStorage.getItem(WIDTH_KEY);
  const n = raw ? Number(raw) : NaN;
  if (Number.isFinite(n) && n >= MIN_WIDTH) return n;
  return DEFAULT_WIDTH;
}

/**
 * The quick look an asset opens into from the library: where it is and how to
 * get it first, then enough to recognise it. Editing, versions, files and
 * housekeeping live on the package page, one click away.
 */
export function AssetDetailPanel({
  assetId,
  onClose,
  onOpenPackage,
  onReservedWidthChange,
}: {
  assetId: number | null;
  onClose: () => void;
  /** Open the asset's full package page. */
  onOpenPackage?: (id: number) => void;
  /**
   * Reports how much horizontal space the open panel covers (0 when closed or
   * fullscreen), so a page whose content reaches the right edge can pad itself
   * out of the way instead of being overlapped.
   */
  onReservedWidthChange?: (width: number) => void;
}) {
  const { data: asset, loading, error, failed, reload } = useAssetDetail(assetId);
  const [width, setWidth] = useState<number>(readStoredWidth);
  const [autoFullscreen, setAutoFullscreen] = useState(false);
  const [maxWidth, setMaxWidth] = useState<number>(1024);
  const asideRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(WIDTH_KEY, String(width));
  }, [width]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia(MOBILE_QUERY);
    const update = () => setAutoFullscreen(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  // Track the panel's positioned ancestor so the user can't drag past
  // the available main-area width.
  useEffect(() => {
    const aside = asideRef.current;
    if (!aside) return;
    const parent = aside.offsetParent as HTMLElement | null;
    if (!parent) return;
    const update = () => setMaxWidth(parent.clientWidth);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(parent);
    return () => observer.disconnect();
  }, [assetId]);

  // Below the mobile breakpoint the panel takes the screen; above it, the full
  // view of an asset is its package page rather than a wider panel.
  const isFullscreen = autoFullscreen;
  // Clamp to the available space at render time (window resize, sidebar
  // widened) while preserving the user's chosen width in state, so it springs
  // back when room reappears.
  const effectiveWidth = Math.min(Math.max(MIN_WIDTH, width), maxWidth);
  const reservedWidth = assetId == null || isFullscreen ? 0 : effectiveWidth;

  useEffect(() => {
    onReservedWidthChange?.(reservedWidth);
  }, [reservedWidth, onReservedWidthChange]);

  if (assetId == null) return null;

  return (
    <DetailPanel
      ref={asideRef}
      onClose={onClose}
      title={asset?.name ?? (loading ? "Loading…" : "Asset")}
      variant="overlay"
      resizable
      width={effectiveWidth}
      onWidthChange={setWidth}
      minWidth={MIN_WIDTH}
      maxWidth={maxWidth}
      fullscreen={isFullscreen}
    >
      {asset ? (
        <QuickLook
          asset={asset}
          onOpenPackage={onOpenPackage ? () => onOpenPackage(asset.id) : undefined}
        />
      ) : failed && isNotFound(error) ? (
        <EmptyState title="Asset not found" description="It may have been deleted." />
      ) : failed ? (
        <ErrorState title="Couldn't load this asset" onRetry={reload} />
      ) : null}
    </DetailPanel>
  );
}

function QuickLook({
  asset,
  onOpenPackage,
}: {
  asset: AssetDetailView;
  onOpenPackage?: () => void;
}) {
  return (
    <div className="@container flex flex-col gap-3 p-3">
      {/* The name lives in the panel header. */}
      <AssetStatusChips asset={asset} />

      <GetItSection asset={asset} />

      {onOpenPackage && (
        <Button variant="primary" leadingIcon={<Maximize2 size={12} />} onClick={onOpenPackage}>
          Open package page
        </Button>
      )}

      <AssetGallery images={asset.images} />

      {asset.description.trim() ? (
        <ClampedDescription>{asset.description}</ClampedDescription>
      ) : (
        <p className="text-label text-subtle-foreground">No description yet.</p>
      )}

      <TagsSection asset={asset} />

      <div className="flex items-center justify-between gap-2 border-t border-border pt-3">
        <span className="text-label text-muted-foreground">Licence</span>
        <LicenceSummary licence={asset.licence ?? null} />
      </div>
    </div>
  );
}
