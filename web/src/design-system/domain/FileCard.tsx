import { forwardRef, type HTMLAttributes } from "react";
import { Check, Folder } from "lucide-react";

import { cn } from "@/design-system/utils";
import { formatBytes } from "@/design-system/utils/format";

import { FileThumb, type FileThumbProps } from "./FileThumb";
import type { FileNode } from "./fileTree";

export interface FolderCardProps extends Omit<HTMLAttributes<HTMLButtonElement>, "onOpen"> {
  name: string;
  /** Number of items the folder contains (recursive). */
  count: number;
  onOpen?: () => void;
}

/** Grid tile for a folder — opens into the folder on click. */
export const FolderCard = forwardRef<HTMLButtonElement, FolderCardProps>(function FolderCard(
  { className, name, count, onOpen, onClick, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      onClick={(e) => {
        onOpen?.();
        onClick?.(e);
      }}
      className={cn(
        "flex flex-col gap-2 rounded-md text-left",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
      {...props}
    >
      <div className="grid aspect-square place-items-center rounded-md border border-border bg-card text-subtle-foreground transition-colors duration-fast ease-standard hover:border-border-strong hover:text-muted-foreground">
        <Folder size={34} strokeWidth={1.25} />
      </div>
      <div className="px-0.5">
        <div className="truncate text-ui font-medium text-foreground">{name}</div>
        <div className="mt-0.5 text-caption">
          {count} {count === 1 ? "item" : "items"}
        </div>
      </div>
    </button>
  );
});

export interface FileCardProps extends Omit<HTMLAttributes<HTMLButtonElement>, "onSelect"> {
  node: FileNode;
  selected?: boolean;
  /** Highlighted as the currently open file in the detail panel. */
  open?: boolean;
  /** Shown under the name in flat results mode instead of the size. */
  pathLabel?: string;
  /** Resolved thumbnail image URL (object URL) — see FileThumb.imageUrl. */
  imageUrl?: string;
  imageSrc?: FileThumbProps["imageSrc"];
  onOpen?: () => void;
  onSelect?: () => void;
}

/**
 * Grid tile for a file — thumbnail with a mono type badge and a selection
 * check, name + size/path beneath. Selection state mirrors AssetCard.
 */
export const FileCard = forwardRef<HTMLButtonElement, FileCardProps>(function FileCard(
  {
    className,
    node,
    selected,
    open,
    pathLabel,
    imageUrl,
    imageSrc,
    onOpen,
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
        if (e.shiftKey || e.metaKey || e.ctrlKey) onSelect?.();
        else onOpen?.();
        onClick?.(e);
      }}
      className={cn(
        "group/file flex flex-col gap-2 rounded-md text-left",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
      {...props}
    >
      <div
        className={cn(
          "relative aspect-square overflow-hidden rounded-md border border-border bg-card transition-colors duration-fast ease-standard",
          "group-hover/file:border-border-strong",
          open && "border-border-strong",
          selected && "border-primary ring-2 ring-ring",
        )}
      >
        <FileThumb node={node} imageUrl={imageUrl} imageSrc={imageSrc} />
        {node.ext && (
          <span className="absolute left-1.5 top-1.5 inline-flex items-center rounded-xs bg-black/55 px-1.5 py-0.5 font-mono text-[9.5px] uppercase tracking-wide text-white">
            {node.ext.replace(".", "")}
          </span>
        )}
        <span
          className={cn(
            "absolute right-1.5 top-1.5 grid h-4 w-4 place-items-center rounded-full text-primary-foreground transition-opacity duration-fast ease-standard",
            selected ? "bg-primary opacity-100" : "opacity-0",
          )}
        >
          <Check size={11} strokeWidth={3} />
        </span>
      </div>
      <div className="px-0.5">
        <div className="truncate text-ui font-medium text-foreground">{node.name}</div>
        <div className="mt-0.5 truncate text-caption">
          {pathLabel ? (
            <span className="font-mono text-faint-foreground">{pathLabel}</span>
          ) : (
            formatBytes(node.size)
          )}
        </div>
      </div>
    </button>
  );
});
