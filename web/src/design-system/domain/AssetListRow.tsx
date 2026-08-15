import { forwardRef, type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

import { AssetThumb, type ThumbDescriptor } from "./AssetThumb";

export interface AssetListRowProps extends Omit<HTMLAttributes<HTMLTableRowElement>, "onSelect"> {
  name: string;
  thumb?: ThumbDescriptor;
  /** Cells to render after the name (each wrapped in its own <td>). */
  cells?: ReactNode[];
  selected?: boolean;
  onSelect?: () => void;
}

/**
 * Asset list row — table row variant for dense list views. Leading cell is
 * 22px mini-thumb + asset name; subsequent cells are caller-provided so the
 * column schema can vary between views.
 */
export const AssetListRow = forwardRef<HTMLTableRowElement, AssetListRowProps>(
  function AssetListRow(
    { className, name, thumb, cells, selected, onSelect, onClick, ...props },
    ref,
  ) {
    return (
      <tr
        ref={ref}
        aria-selected={selected || undefined}
        onClick={(e) => {
          onSelect?.();
          onClick?.(e);
        }}
        className={cn(
          "cursor-pointer text-list-row text-muted-foreground transition-colors duration-std ease",
          "hover:bg-card hover:text-foreground",
          "aria-selected:bg-primary-soft aria-selected:text-foreground",
          className,
        )}
        {...props}
      >
        <td className="px-2.5 py-1.5">
          <div className="flex items-center gap-2">
            <div className="h-[22px] w-[22px] shrink-0 overflow-hidden rounded-[3px] border border-border bg-muted">
              <AssetThumb thumb={thumb} />
            </div>
            <span className="truncate font-medium text-foreground">{name}</span>
          </div>
        </td>
        {cells?.map((cell, i) => (
          <td key={i} className="px-2.5 py-1.5">
            {cell}
          </td>
        ))}
      </tr>
    );
  },
);

/**
 * Sticky header row for AssetListRow tables — matches the mockup's small-caps
 * label styling.
 */
export function AssetListHead({ columns }: { columns: ReactNode[] }) {
  return (
    <thead className="sticky top-0 z-10 bg-background">
      <tr className="text-table-header text-subtle-foreground">
        <th className="px-2.5 py-1.5 text-left">Name</th>
        {columns.map((col, i) => (
          <th key={i} className="px-2.5 py-1.5 text-left">
            {col}
          </th>
        ))}
      </tr>
    </thead>
  );
}
