import type { AssetDetailView, AssetVersionView } from "@/hooks/useDataQueries";

const byNewestImport = (a: AssetVersionView, b: AssetVersionView) =>
  b.import_date.localeCompare(a.import_date);

// Download and re-extract target the latest version (by import date) that still
// retains its original archive: the current version may be a subset, or an
// earlier re-extract that no longer has one. Download works for any retained
// package (unitypackage or zip); re-extract is Unity-only.

/** The version whose original archive a download fetches, if any still has one. */
export function downloadSource(asset: AssetDetailView): AssetVersionView | undefined {
  return [...asset.versions]
    .filter((v) => v.has_original && v.original_package_object_key)
    .sort(byNewestImport)[0];
}

/** The file name a downloaded original saves as. */
export function originalFileName(asset: AssetDetailView, version: AssetVersionView): string {
  return `${asset.name}.${version.package_kind === "zip" ? "zip" : "unitypackage"}`;
}

/** The Unity version re-extract rebuilds the file list from, if any. */
export function reExtractSource(asset: AssetDetailView): AssetVersionView | undefined {
  return [...asset.versions]
    .filter((v) => v.package_kind === "unitypackage" && v.has_original)
    .sort(byNewestImport)[0];
}

/** A source link's host, the short name it shows under. */
export function sourceLabel(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
}
