import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { FilePlus2 } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { AssetCard, Checkbox, EmptyState, cn, formatBytes } from "@/design-system";
import type { AssetRow } from "@/hooks/useDataQueries";
import { useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";

// The grid auto-fills as many cells as fit (min 180px wide), recomputed on
// resize. Row height is measured per row (measureElement) since the square
// thumbnail makes it track the responsive cell width; the constants below only
// seed the pre-measurement estimate.
const MIN_CELL_WIDTH = 180;
const FOOTER_HEIGHT = 44; // name + meta line under the square thumbnail.
const GAP = 12;

/**
 * Library grid — virtualised with @tanstack/react-virtual so multi-thousand
 * asset libraries don't render every card to the DOM. Each visible row is a
 * CSS subgrid of N cards; N is derived from the container width so the grid
 * still feels responsive.
 */
export function AssetGrid({
  assets,
  onOpen,
  selectedId,
  selectedIds,
  onToggleSelect,
}: {
  assets: AssetRow[];
  onOpen?: (id: number) => void;
  selectedId?: number | null;
  selectedIds?: Set<number>;
  onToggleSelect?: (id: number) => void;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const { columns, cellWidth } = useGridMetrics(scrollRef);

  const rows = useMemo(() => {
    const out: AssetRow[][] = [];
    for (let i = 0; i < assets.length; i += columns) {
      out.push(assets.slice(i, i + columns));
    }
    return out;
  }, [assets, columns]);

  const estimateRef = useRef(MIN_CELL_WIDTH + FOOTER_HEIGHT + GAP);
  estimateRef.current = (cellWidth || MIN_CELL_WIDTH) + FOOTER_HEIGHT + GAP;

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => estimateRef.current,
    measureElement: (el) => el.getBoundingClientRect().height,
    overscan: 4,
  });

  // Keep the keyboard-selected card in view. We compute the row that
  // contains `selectedId` and ask the virtualiser to scroll to it.
  useEffect(() => {
    if (selectedId == null) return;
    const i = assets.findIndex((a) => a.id === selectedId);
    if (i < 0) return;
    virtualizer.scrollToIndex(Math.floor(i / columns), { align: "auto" });
  }, [selectedId, assets, columns, virtualizer]);

  return (
    <div ref={scrollRef} className="flex-1 overflow-auto p-3.5">
      {assets.length === 0 ? (
        <EmptyState
          className="h-full"
          icon={<FilePlus2 size={28} />}
          title="No assets yet"
          description="Drop .unitypackage files, sprites, audio, or fonts onto this view — or click to select. Damson extracts, hashes, and licence-tags each import."
        />
      ) : (
        <div style={{ height: virtualizer.getTotalSize(), position: "relative", width: "100%" }}>
          {virtualizer.getVirtualItems().map((vRow) => {
            const row = rows[vRow.index];
            return (
              <div
                key={vRow.key}
                data-index={vRow.index}
                ref={virtualizer.measureElement}
                className="grid"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${vRow.start}px)`,
                  gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                  columnGap: GAP,
                  paddingBottom: GAP,
                }}
              >
                {row.map((a) => (
                  <GridCard
                    key={a.id}
                    asset={a}
                    selected={selectedId === a.id}
                    onOpen={onOpen}
                    selectionActive={!!selectedIds && selectedIds.size > 0}
                    checked={selectedIds?.has(a.id) ?? false}
                    onToggleSelect={onToggleSelect}
                  />
                ))}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// GridCard renders one asset tile, fetching its server-generated thumbnail as
// an authenticated blob (the /api/thumbs endpoint needs a bearer token, so a
// plain <img src> won't do). While the blob loads — or when the asset has no
// thumbnail — AssetCard falls back to its procedural placeholder.
function GridCard({
  asset: a,
  selected,
  onOpen,
  selectionActive,
  checked,
  onToggleSelect,
}: {
  asset: AssetRow;
  selected: boolean;
  onOpen?: (id: number) => void;
  selectionActive?: boolean;
  checked?: boolean;
  onToggleSelect?: (id: number) => void;
}) {
  const imageUrl = useBlobUrl(
    a.thumbnail_object_key ? `/api/thumbs/${a.thumbnail_object_key}` : undefined,
  );
  return (
    <div className="group relative">
      {onToggleSelect && (
        <div
          className={cn(
            "absolute left-2 top-2 z-10 transition-opacity",
            checked || selectionActive ? "opacity-100" : "opacity-0 group-hover:opacity-100",
          )}
        >
          <Checkbox
            checked={checked}
            onCheckedChange={() => onToggleSelect(a.id)}
            aria-label={`Select ${a.name}`}
          />
        </div>
      )}
      <AssetCard
        name={a.name}
        data-asset-id={a.id}
        imageUrl={imageUrl}
        selected={selected || checked}
        onSelect={() => onOpen?.(a.id)}
        sub={
          <>
            <span className="font-mono">{formatBytes(a.size_bytes)}</span>
            {a.file_count > 1 && (
              <>
                <span className="text-text-4">·</span>
                <span className="font-mono">{a.file_count} files</span>
              </>
            )}
          </>
        }
        needsReview={a.needs_review}
        updateAvailable={a.update_available}
      />
    </div>
  );
}

// useGridMetrics tracks the number of cards per row and the resulting cell
// width based on the scroll container's measured width. Recomputes on resize so
// density toggles and window resizes both work. The cell width lets the caller
// estimate row height (thumbnails are square) before the row is measured.
function useGridMetrics(ref: React.RefObject<HTMLDivElement | null>): {
  columns: number;
  cellWidth: number;
} {
  const [metrics, setMetrics] = useState({ columns: 4, cellWidth: MIN_CELL_WIDTH });
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const compute = () => {
      if (!el.isConnected) return;
      const w = el.clientWidth - 28; // subtract the p-3.5 (14px each side).
      const columns = Math.max(1, Math.floor((w + GAP) / (MIN_CELL_WIDTH + GAP)));
      const cellWidth = (w - (columns - 1) * GAP) / columns;
      setMetrics((prev) =>
        prev.columns === columns && prev.cellWidth === cellWidth ? prev : { columns, cellWidth },
      );
    };
    compute();
    const ro = new ResizeObserver(compute);
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return metrics;
}
