import type { AssetRow } from "@/hooks/useDataQueries";

function formatSize(bytes: number): string {
  if (bytes === 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let n = bytes;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n < 10 && i > 0 ? 1 : 0)} ${units[i]}`;
}

// First-pass grid: square thumbnails (icon fallback only — no real thumbs
// until M6), name under each tile. Virtualisation is held until we actually
// have asset volumes that justify it.
export function AssetGrid({ assets }: { assets: AssetRow[] }) {
  if (assets.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-text-3">
        Drop files here to begin, or click to select.
      </div>
    );
  }
  return (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3 p-4">
      {assets.map((a) => (
        <li
          key={a.id}
          className="group flex flex-col overflow-hidden rounded-md border border-line bg-panel transition hover:border-accent"
        >
          <div className="aspect-square bg-surface" aria-hidden>
            {/* Icon fallback — real thumbnails arrive in M6. */}
            <div className="flex h-full items-center justify-center text-xs text-text-3">
              {a.file_count} {a.file_count === 1 ? "file" : "files"}
            </div>
          </div>
          <div className="flex flex-col gap-0.5 p-2 text-xs">
            <div className="truncate font-medium text-text">{a.name}</div>
            <div className="flex items-center justify-between text-text-3">
              <span>{formatSize(a.size_bytes)}</span>
              {a.needs_review && (
                <span className="rounded bg-accent/20 px-1.5 py-0.5 font-mono text-[10px] text-accent-fg">
                  review
                </span>
              )}
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
