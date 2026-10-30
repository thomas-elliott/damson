// A store search for an unmatched asset returns whatever the store ranks
// highest, which for a name like "Sample Terrain Textures" is Gaia Pro and
// Behavior Designer. A suggestion is worth showing only when it shares most of
// the asset's words; the rest are noise the user would have to read past.

// Words that say nothing about which pack this is.
const FILLER = new Set([
  "a",
  "an",
  "and",
  "by",
  "for",
  "free",
  "kit",
  "lite",
  "of",
  "pack",
  "package",
  "sample",
  "the",
  "unity",
  "with",
]);

function words(s: string): Set<string> {
  return new Set(
    s
      .toLowerCase()
      // Raw folder names separate words with underscores or dashes.
      .split(/[^\p{L}\p{N}]+/u)
      .filter((w) => w.length > 1 && !FILLER.has(w) && !/^v?\d+$/.test(w)),
  );
}

/**
 * Whether a store listing is plausibly the asset: more than half of the asset
 * name's meaningful words appear in the listing title, so a two-word name needs
 * both. A name with no meaningful words matches nothing.
 */
export function isRelevantListing(assetName: string, listingTitle: string): boolean {
  const name = words(assetName);
  if (name.size === 0) return false;
  const title = words(listingTitle);
  let shared = 0;
  for (const w of name) if (title.has(w)) shared++;
  return shared / name.size > 0.5;
}

/** The candidates worth showing for an asset, in the store's order. */
export function relevantListings<T extends { title: string }>(
  assetName: string,
  candidates: T[],
): T[] {
  return candidates.filter((c) => isRelevantListing(assetName, c.title));
}
