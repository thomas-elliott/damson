import { useEffect, useRef } from "react";
import { FilePlus2 } from "lucide-react";

import {
  AssetCard,
  EmptyState,
  LicencePill,
  formatBytes,
} from "@/design-system";
import type { AssetRow } from "@/hooks/useDataQueries";

/**
 * Library grid — one AssetCard per row. Each card is a focusable button so
 * keyboard navigation (handled by the parent page) can scroll the
 * highlighted card into view via aria-current matching.
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
  const gridRef = useRef<HTMLDivElement>(null);

  // When `selectedId` changes from outside (keyboard navigation), make sure
  // the matching card is in view. AssetCard renders its outer wrapper as
  // the click target, so we look up via a data attribute we attach below.
  useEffect(() => {
    if (selectedId == null || !gridRef.current) return;
    const node = gridRef.current.querySelector(`[data-asset-id="${selectedId}"]`);
    if (node && "scrollIntoView" in node) {
      (node as HTMLElement).scrollIntoView({ block: "nearest" });
    }
  }, [selectedId]);

  if (assets.length === 0) {
    return (
      <EmptyState
        icon={<FilePlus2 size={28} />}
        title="No assets yet"
        description="Drop .unitypackage files, sprites, audio, or fonts onto this view — or click to select. Damson extracts, hashes, and licence-tags each import."
      />
    );
  }
  return (
    <div
      ref={gridRef}
      className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3 p-3.5"
    >
      {assets.map((a) => (
        <AssetCard
          key={a.id}
          name={a.name}
          data-asset-id={a.id}
          selected={selectedId === a.id}
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
              <LicencePill
                status={a.licence_status ?? "unk"}
                spdx={a.licence_spdx}
              />
            ) : undefined
          }
          needsReview={a.needs_review}
        />
      ))}
    </div>
  );
}
