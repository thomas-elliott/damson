import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { FilePlus2 } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { AssetCard, EmptyState, LicencePill, formatBytes } from "@/design-system";
import type { AssetRow } from "@/hooks/useDataQueries";
import { useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";

// Cell sizing matches the Tailwind class `minmax(180px, 1fr)` used previously
// — the virtualiser needs concrete pixel sizes to estimate row height. The
// grid auto-fills as many cells as fit, so we recompute on resize.
const MIN_CELL_WIDTH = 180;
const CELL_HEIGHT = 220; // card body + footer; tuned to AssetCard's CSS.
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
}: {
  assets: AssetRow[];
  onOpen?: (id: number) => void;
  selectedId?: number | null;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const columns = useColumnCount(scrollRef);

  const rows = useMemo(() => {
    const out: AssetRow[][] = [];
    for (let i = 0; i < assets.length; i += columns) {
      out.push(assets.slice(i, i + columns));
    }
    return out;
  }, [assets, columns]);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => CELL_HEIGHT + GAP,
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
                data-row-index={vRow.index}
                className="grid"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${vRow.start}px)`,
                  gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                  gap: GAP,
                }}
              >
                {row.map((a) => (
                  <GridCard key={a.id} asset={a} selected={selectedId === a.id} onOpen={onOpen} />
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
}: {
  asset: AssetRow;
  selected: boolean;
  onOpen?: (id: number) => void;
}) {
  const imageUrl = useBlobUrl(
    a.thumbnail_object_key ? `/api/thumbs/${a.thumbnail_object_key}` : undefined,
  );
  return (
    <AssetCard
      name={a.name}
      data-asset-id={a.id}
      imageUrl={imageUrl}
      selected={selected}
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
      trailing={
        a.licence_spdx ? (
          <LicencePill status={a.licence_status ?? "unk"} spdx={a.licence_spdx} />
        ) : undefined
      }
      needsReview={a.needs_review}
    />
  );
}

// useColumnCount tracks the number of cards per row based on the scroll
// container's measured width. Recomputes on resize so density toggles and
// window resizes both work.
function useColumnCount(ref: React.RefObject<HTMLDivElement | null>): number {
  const [cols, setCols] = useState(4);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const compute = () => {
      if (!el.isConnected) return;
      const w = el.clientWidth;
      const next = Math.max(1, Math.floor((w + GAP) / (MIN_CELL_WIDTH + GAP)));
      setCols(next);
    };
    compute();
    const ro = new ResizeObserver(compute);
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return cols;
}
