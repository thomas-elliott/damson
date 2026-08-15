import { useEffect, useState } from "react";
import { unzipSync } from "fflate";

import { fetchBlob } from "@/api";
import { formatBytes } from "@/design-system";
import type { FileNode } from "@/design-system";

// Cap the archive we'll download to list. Bigger than this → don't fetch.
const MAX_ZIP_BYTES = 64 * 1024 * 1024;
// Declared uncompressed/compressed ratio above which we flag a possible bomb.
const SUSPICIOUS_RATIO = 100;

interface ZipEntry {
  name: string;
  size: number;
  ratio: number;
}

interface Listing {
  key: string;
  entries?: ZipEntry[];
  error?: string;
}

/**
 * Safe contents listing for a .zip. Reads only the directory metadata via
 * fflate's filter hook — every entry is rejected for decompression, so nothing
 * is ever inflated (decompression-bomb safe). Flags entries whose declared
 * uncompressed/compressed ratio looks like a bomb. Default export for lazy use.
 */
export default function ZipPreview({ node }: { node: FileNode }) {
  const objectKey = node.objectKey;
  const [listing, setListing] = useState<Listing>();

  useEffect(() => {
    if (!objectKey) return;
    let cancelled = false;
    fetchBlob(`/api/files/${objectKey}`)
      .then(async (blob) => {
        if (blob.size > MAX_ZIP_BYTES) {
          if (!cancelled) setListing({ key: objectKey, error: "Archive too large to list." });
          return;
        }
        const bytes = new Uint8Array(await blob.arrayBuffer());
        const entries: ZipEntry[] = [];
        unzipSync(bytes, {
          filter: (f) => {
            if (!f.name.endsWith("/")) {
              entries.push({
                name: f.name,
                size: f.originalSize,
                ratio: f.size > 0 ? f.originalSize / f.size : 0,
              });
            }
            return false; // never inflate — directory listing only
          },
        });
        entries.sort((a, b) => a.name.localeCompare(b.name));
        if (!cancelled) setListing({ key: objectKey, entries });
      })
      .catch(() => {
        if (!cancelled) setListing({ key: objectKey, error: "Couldn’t read this archive." });
      });
    return () => {
      cancelled = true;
    };
  }, [objectKey]);

  const ready = listing?.key === objectKey ? listing : undefined;

  if (!ready) {
    return (
      <div className="grid h-full w-full place-items-center text-ui text-subtle-foreground">
        Reading archive…
      </div>
    );
  }
  if (ready.error) {
    return (
      <div className="grid h-full w-full place-items-center p-4 text-center text-ui text-subtle-foreground">
        {ready.error}
      </div>
    );
  }

  const entries = ready.entries ?? [];
  return (
    <div className="flex h-full w-full flex-col overflow-auto bg-muted text-left">
      <div className="sticky top-0 border-b border-border bg-muted px-3 py-2 text-small text-subtle-foreground">
        {entries.length} {entries.length === 1 ? "file" : "files"} — listing only, nothing extracted
      </div>
      <ul className="divide-y divide-border">
        {entries.map((e) => (
          <li key={e.name} className="flex items-center gap-2 px-3 py-1.5 text-small">
            <span
              className="min-w-0 flex-1 truncate font-mono text-muted-foreground"
              title={e.name}
            >
              {e.name}
            </span>
            {e.ratio > SUSPICIOUS_RATIO && (
              <span className="shrink-0 rounded-sm bg-warning-soft px-1.5 text-[10px] text-warning">
                {Math.round(e.ratio)}× ratio
              </span>
            )}
            <span className="shrink-0 font-mono tabular-nums text-faint-foreground">
              {formatBytes(e.size)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
