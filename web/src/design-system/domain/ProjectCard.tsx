import { forwardRef, type HTMLAttributes, type ReactNode } from "react";
import { Plus } from "lucide-react";

import { cn } from "@/design-system/utils";

import { ProjectGlyph } from "./ProjectGlyph";

export interface ProjectCardProps
  extends Omit<HTMLAttributes<HTMLButtonElement>, "title"> {
  name: string;
  glyph: string;
  color: string;
  path?: string;
  engine?: string;
  /** Arbitrary secondary content below the header (stats, licence bar, etc.). */
  children?: ReactNode;
  onOpen?: () => void;
}

export const ProjectCard = forwardRef<HTMLButtonElement, ProjectCardProps>(
  function ProjectCard(
    { className, name, glyph, color, path, engine, children, onOpen, onClick, ...props },
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
          "flex flex-col gap-3 rounded-md border border-line bg-panel p-3.5 text-left transition-colors duration-std ease",
          "hover:border-line-2",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
          className,
        )}
        {...props}
      >
        <div className="flex items-start gap-3">
          <ProjectGlyph size="md" glyph={glyph} color={color} />
          <div className="min-w-0 flex-1">
            <div className="truncate text-ui font-medium text-text">
              {name}
            </div>
            {(path || engine) && (
              <div className="mt-0.5 flex items-center gap-1.5 text-caption">
                {engine && <span>{engine}</span>}
                {engine && path && <span className="text-text-3">·</span>}
                {path && (
                  <span className="truncate font-mono text-text-3">{path}</span>
                )}
              </div>
            )}
          </div>
        </div>
        {children && <div className="flex flex-col gap-2">{children}</div>}
      </button>
    );
  },
);

/**
 * "New project" dashed card — matches `.proj-card.new` in the mockup. Use
 * inside the same grid as ProjectCard.
 */
export function ProjectCardNew({
  className,
  onClick,
  label = "New project",
}: {
  className?: string;
  onClick?: () => void;
  label?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex min-h-[180px] cursor-pointer flex-col items-center justify-center gap-2 rounded-md border border-dashed border-line-2 bg-transparent p-4 text-ui text-text-3 transition-colors duration-std ease",
        "hover:border-text-4 hover:text-text-2",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
        className,
      )}
    >
      <Plus size={18} />
      <span>{label}</span>
    </button>
  );
}
