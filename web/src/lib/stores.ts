// Display names for the storefronts the metadata adapters support. The API
// stores the short identifier; the UI shows the store's own name.
const STORE_LABELS: Record<string, string> = {
  unity: "Unity Asset Store",
  sketchfab: "Sketchfab",
  itch: "itch.io",
  generic: "Other",
};

// storeLabel falls back to the raw identifier, so a store added server-side
// still renders rather than disappearing.
export function storeLabel(store: string): string;
export function storeLabel(store: string | undefined): string | undefined;
export function storeLabel(store: string | undefined): string | undefined {
  if (!store) return undefined;
  return STORE_LABELS[store] ?? store;
}
