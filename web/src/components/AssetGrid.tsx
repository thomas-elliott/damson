import { FilePlus2 } from "lucide-react";

import {
  AssetCard,
  EmptyState,
  formatBytes,
} from "@/design-system";
import type { AssetRow } from "@/hooks/useDataQueries";

/**
 * Library grid — one AssetCard per row. Thumbnails are placeholders until M6
 * lands real thumbnail rendering; for now every asset renders the unknown
 * glyph.
 */
export function AssetGrid({ assets }: { assets: AssetRow[] }) {
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
    <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3 p-3.5">
      {assets.map((a) => (
        <AssetCard
          key={a.id}
          name={a.name}
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
        />
      ))}
    </div>
  );
}
