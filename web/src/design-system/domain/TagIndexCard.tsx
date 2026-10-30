import { cn } from "@/design-system/utils";

import { TypesetCover } from "./TypesetCover";

export interface TagIndexCover {
  name: string;
  publisher?: string;
  imageUrl?: string;
}

export interface TagIndexCardProps {
  /** A second-level tag such as `3d/environment`. */
  path: string;
  count: number;
  /** Its largest child tags, biggest first. */
  topChildren?: { name: string; count: number }[];
  /** Up to three covers from inside the tag; the first is the large tile. */
  covers: TagIndexCover[];
  selected?: boolean;
  onOpen?: () => void;
  className?: string;
}

function CoverTile({ cover }: { cover?: TagIndexCover }) {
  if (!cover) return <div className="bg-card" />;
  return (
    <div className="relative overflow-hidden bg-card">
      {cover.imageUrl ? (
        <img src={cover.imageUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        <TypesetCover name={cover.name} publisher={cover.publisher} compact />
      )}
    </div>
  );
}

/**
 * A card on the tag index: a mosaic of covers from inside the tag, the tag's
 * name under its parent, its count and its biggest child tags.
 */
export function TagIndexCard({
  path,
  count,
  topChildren = [],
  covers,
  selected,
  onOpen,
  className,
}: TagIndexCardProps) {
  const i = path.lastIndexOf("/");
  const parent = i < 0 ? null : path.slice(0, i);
  const name = path.slice(i + 1);
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-pressed={selected}
      className={cn(
        "flex w-full min-w-0 flex-col gap-2 rounded-md text-left",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        className,
      )}
    >
      <div
        className={cn(
          "grid aspect-[4/3] w-full grid-cols-[1.2fr_1fr] grid-rows-2 gap-0.5 overflow-hidden rounded-md",
          "[&>:first-child]:row-span-2",
          selected && "ring-2 ring-primary ring-offset-[3px] ring-offset-background",
        )}
      >
        <CoverTile cover={covers[0]} />
        <CoverTile cover={covers[1]} />
        <CoverTile cover={covers[2]} />
      </div>
      <div className="flex min-w-0 items-baseline gap-2">
        {parent && (
          <span className="text-overline text-subtle-foreground">
            {parent.replaceAll("/", " / ")} /
          </span>
        )}
        <span className="truncate font-display text-subhead-bold uppercase tracking-wide text-foreground">
          {name}
        </span>
        <span className="text-mono text-subtle-foreground">{count}</span>
      </div>
      {topChildren.length > 0 && (
        <span className="truncate text-label text-subtle-foreground">
          {topChildren.map((c) => `${c.name} ${c.count}`).join(" · ")}
        </span>
      )}
    </button>
  );
}
