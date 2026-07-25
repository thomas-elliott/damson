import { forwardRef, type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

import { AssetThumb, type ThumbDescriptor } from "./AssetThumb";

export interface AssetCardProps extends Omit<HTMLAttributes<HTMLButtonElement>, "onSelect"> {
  name: string;
  thumb?: ThumbDescriptor;
  /**
   * Real thumbnail image URL (e.g. an object URL for a server-generated
   * thumbnail). When set it renders over the procedural `thumb`; while it's
   * loading or absent, `thumb` is the fallback.
   */
  imageUrl?: string;
  /** Short type label shown as an overlay badge on the thumbnail. */
  typeBadge?: ReactNode;
  /** Meta line under the name (file size, file count, etc.). */
  sub?: ReactNode;
  /** Licence/state pills pinned to the name row. */
  trailing?: ReactNode;
  /** Flag for the review dot in the thumb's top-right corner. */
  needsReview?: boolean;
  selected?: boolean;
  onSelect?: () => void;
}

/**
 * Asset grid tile — square thumbnail above name/meta. Selection state gives
 * it an accent border + ring. Keep the API generic (name, sub, trailing) so
 * feature-level cards can compose without extending domain types.
 */
export const AssetCard = forwardRef<HTMLButtonElement, AssetCardProps>(function AssetCard(
  {
    className,
    name,
    thumb,
    imageUrl,
    typeBadge,
    sub,
    trailing,
    needsReview,
    selected,
    onSelect,
    onClick,
    ...props
  },
  ref,
) {
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
        "flex w-full min-w-0 flex-col gap-2 rounded-md text-left",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring",
        className,
      )}
      {...props}
    >
      <div
        className={cn(
          "relative aspect-square overflow-hidden rounded-md border border-line bg-panel transition-colors duration-std ease",
          "group-hover:border-line-2",
          "aria-[pressed=true]:border-accent aria-[pressed=true]:ring-2 aria-[pressed=true]:ring-accent-ring",
          selected && "border-accent ring-2 ring-accent-ring",
        )}
      >
        {imageUrl ? (
          <img src={imageUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <AssetThumb thumb={thumb} />
        )}
        {typeBadge && (
          <span className="absolute left-1.5 top-1.5 inline-flex items-center rounded-[3px] bg-black/55 px-1.5 py-0.5 font-mono text-[9.5px] uppercase tracking-wide text-white">
            {typeBadge}
          </span>
        )}
        {needsReview && (
          <span
            aria-label="Needs review"
            className="absolute right-1.5 top-1.5 block h-1.5 w-1.5 rounded-full bg-accent ring-2 ring-panel"
          />
        )}
      </div>
      <div className="min-w-0 px-0.5">
        <div className="truncate text-ui font-medium text-text">{name}</div>
        {(sub || trailing) && (
          <div className="mt-0.5 flex items-center justify-between gap-1.5">
            {sub && (
              <div className="flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap text-caption">
                {sub}
              </div>
            )}
            {trailing && <div className="shrink-0">{trailing}</div>}
          </div>
        )}
      </div>
    </button>
  );
});
