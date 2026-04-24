/** Human-readable byte size using SI units (1 KB = 1000 B). */
export function formatBytes(b: number): string {
  if (b === 0) return "—";
  if (b < 1_000) return `${b} B`;
  if (b < 1_000_000) return `${(b / 1_000).toFixed(1)} KB`;
  if (b < 1_000_000_000) return `${(b / 1_000_000).toFixed(1)} MB`;
  return `${(b / 1_000_000_000).toFixed(2)} GB`;
}
