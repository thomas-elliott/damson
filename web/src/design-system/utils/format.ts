/** Human-readable byte size using SI units (1 KB = 1000 B). */
export function formatBytes(b: number): string {
  if (b === 0) return "—";
  if (b < 1_000) return `${b} B`;
  if (b < 1_000_000) return `${(b / 1_000).toFixed(1)} KB`;
  if (b < 1_000_000_000) return `${(b / 1_000_000).toFixed(1)} MB`;
  return `${(b / 1_000_000_000).toFixed(2)} GB`;
}

/**
 * An asset name as a person would write it. Names taken from a folder or file
 * (`POLYGON_Fantasy_Village_URP`) use underscores for spaces; this shows them
 * as spaces. Display only: the stored name, and search, keep the original.
 */
export function tidyAssetName(name: string): string {
  const tidy = name.replace(/_+/g, " ").replace(/\s+/g, " ").trim();
  return tidy || name;
}
