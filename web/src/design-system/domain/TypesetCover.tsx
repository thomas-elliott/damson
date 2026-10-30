import { cn } from "@/design-system/utils";

// Yellow and orange are left out: they are the brand and "needs review".
const HUES = [145, 165, 190, 215, 240, 265, 290, 315, 340, 10];

// A stable hue per publisher, so one publisher's packs share a colour.
function hueFor(key: string) {
  let h = 0;
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) % 1000;
  return HUES[h % HUES.length];
}

export interface TypesetCoverProps {
  name: string;
  publisher?: string;
  /** For small tiles such as a mosaic: the name only, no publisher or rule. */
  compact?: boolean;
  className?: string;
}

/**
 * The cover for an asset with no preview image: its name set large on a
 * colour taken from its publisher. The colours are generated, like
 * `glyph_color`, so they sit outside the theme tokens and read the same in
 * both themes.
 */
export function TypesetCover({ name, publisher, compact, className }: TypesetCoverProps) {
  const hue = hueFor(publisher || name);
  return (
    // Decorative: the card that holds it carries the name for screen readers.
    <div
      aria-hidden
      className={cn("flex h-full w-full flex-col justify-between p-[8%] @container", className)}
      style={{ background: `oklch(0.36 0.07 ${hue})`, color: `oklch(0.93 0.04 ${hue})` }}
    >
      {publisher && !compact ? (
        <span className="truncate font-display text-label font-semibold uppercase tracking-widest opacity-80">
          {publisher}
        </span>
      ) : (
        <span />
      )}
      {/* Bottom margin leaves room for the card's caption. */}
      <div className={cn("flex flex-col gap-2", !compact && "mb-[28%]")}>
        {!compact && (
          <span className="h-0.5 w-7" style={{ background: `oklch(0.75 0.12 ${hue})` }} />
        )}
        <span
          className="line-clamp-4 font-display font-bold uppercase leading-none break-words"
          // A mosaic tile can be a few dozen pixels wide on a phone; a floor
          // that suits a card would break its words mid-way.
          style={{ fontSize: compact ? "clamp(9px, 13cqw, 22px)" : "clamp(14px, 13cqw, 26px)" }}
        >
          {/* Let snake_case names break after an underscore, not mid-word. */}
          {name.replaceAll("_", "_\u200b")}
        </span>
      </div>
    </div>
  );
}
