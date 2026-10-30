import { forwardRef, type HTMLAttributes, type ReactNode } from "react";
import { Check } from "lucide-react";

import { Badge } from "@/design-system/primitives/Badge";
import { cn } from "@/design-system/utils";

import { AssetThumb, type ThumbDescriptor } from "./AssetThumb";
import { TypesetCover } from "./TypesetCover";

export interface AssetCardProps extends Omit<HTMLAttributes<HTMLButtonElement>, "onSelect"> {
  name: string;
  /** Colours the typeset cover shown when there is no image or `thumb`. */
  publisher?: string;
  thumb?: ThumbDescriptor;
  /**
   * Real thumbnail image URL (e.g. an object URL for a server-generated
   * thumbnail). When set it renders over the procedural `thumb`; while it's
   * loading or absent, `thumb` or the typeset cover is the fallback.
   */
  imageUrl?: string;
  /** Short type label shown as an overlay badge on the cover. */
  typeBadge?: ReactNode;
  /** Meta line in the caption (file size, file count, etc.). */
  sub?: ReactNode;
  /** Licence/state pills at the end of the caption. */
  trailing?: ReactNode;
  needsReview?: boolean;
  updateAvailable?: boolean;
  selected?: boolean;
  onSelect?: () => void;
}

/**
 * Asset grid tile — a 3:4 cover with the name over the art. An asset with no
 * preview gets a typeset cover, which already prints the name, so its caption
 * carries only the meta line. Needs review and update badges sit at the foot.
 * Selection is a brand ring and a tick.
 */
export const AssetCard = forwardRef<HTMLButtonElement, AssetCardProps>(function AssetCard(
  {
    className,
    name,
    publisher,
    thumb,
    imageUrl,
    typeBadge,
    sub,
    trailing,
    needsReview,
    updateAvailable,
    selected,
    onSelect,
    onClick,
    ...props
  },
  ref,
) {
  const typeset = !imageUrl && !thumb;
  return (
    <button
      ref={ref}
      type="button"
      aria-pressed={selected}
      onClick={(e) => {
        onSelect?.();
        onClick?.(e);
      }}
      className={cn(
        "relative block w-full min-w-0 rounded-md text-left",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        className,
      )}
      {...props}
    >
      <div
        className={cn(
          "relative aspect-[3/4] overflow-hidden rounded-md bg-card transition-shadow duration-fast ease-standard",
          selected && "ring-2 ring-primary ring-offset-[3px] ring-offset-background",
        )}
      >
        {imageUrl ? (
          <img src={imageUrl} alt="" className="h-full w-full object-cover" />
        ) : typeset ? (
          <TypesetCover name={name} publisher={publisher} />
        ) : (
          <AssetThumb thumb={thumb} />
        )}
        {typeBadge && (
          <span className="absolute left-1.5 top-1.5 inline-flex items-center rounded-xs bg-black/55 px-1.5 py-0.5 font-mono text-label uppercase tracking-wide text-white">
            {typeBadge}
          </span>
        )}
        <div
          className={cn(
            "absolute inset-x-0 bottom-0 flex flex-col gap-0.5 px-2 pb-1.5 text-white",
            typeset ? "pt-2" : "bg-linear-to-t from-black/85 to-transparent pt-6",
          )}
        >
          {/* The typeset cover prints the name; screen readers still need it. */}
          <span
            className={cn("line-clamp-2 text-ui font-semibold leading-tight", typeset && "sr-only")}
          >
            {name}
          </span>
          {(sub || trailing || needsReview || updateAvailable) && (
            <div className="flex items-center justify-between gap-1.5">
              <div className="flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap text-label text-white/80">
                {sub}
              </div>
              {/* The states sit at the foot, clear of the cover's publisher
                  line and of the art's subject. */}
              {(needsReview || updateAvailable || trailing) && (
                <div className="flex shrink-0 items-center gap-1">
                  {needsReview && <Badge tone="warn">Review</Badge>}
                  {updateAvailable && (
                    <Badge tone="accent" title="A newer version is available from the store">
                      Update
                    </Badge>
                  )}
                  {trailing}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
      {selected && (
        <span
          aria-hidden
          className="absolute -right-2 -top-2 grid size-5 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-background"
        >
          <Check size={12} strokeWidth={3} />
        </span>
      )}
    </button>
  );
});
