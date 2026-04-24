import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

import { ProjectGlyph } from "./ProjectGlyph";

export interface ProjectListRowProps
  extends Omit<HTMLAttributes<HTMLDivElement>, "onSelect" | "title"> {
  name: string;
  glyph: string;
  color: string;
  /** Extra cells on the right — rendered space-separated. */
  cells?: ReactNode[];
  onSelect?: () => void;
}

/**
 * Single-row project entry. Uses flex rather than table cells so callers
 * can mix content types freely. Column widths are the responsibility of the
 * list container (grid-template-columns on the parent).
 */
export function ProjectListRow({
  className,
  name,
  glyph,
  color,
  cells,
  onSelect,
  onClick,
  ...props
}: ProjectListRowProps) {
  return (
    <div
      onClick={(e) => {
        onSelect?.();
        onClick?.(e);
      }}
      className={cn(
        "grid cursor-pointer items-center gap-3 border-b border-line px-3.5 py-2 text-sm-plus text-text-2 transition-colors duration-std ease",
        "hover:bg-surface hover:text-text last:border-b-0",
        className,
      )}
      {...props}
    >
      <div className="flex items-center gap-2.5">
        <ProjectGlyph size="sm" glyph={glyph} color={color} />
        <span className="truncate font-medium text-text">{name}</span>
      </div>
      {cells}
    </div>
  );
}
