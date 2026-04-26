import { forwardRef, type HTMLAttributes, type ReactNode } from "react";
import { Folder, Sliders } from "lucide-react";

import { cn } from "@/design-system/utils";
import { Pill } from "@/design-system/primitives/Pill";

export interface CollectionCardProps
  extends Omit<HTMLAttributes<HTMLButtonElement>, "onSelect" | "title"> {
  name: string;
  count: number;
  kind?: "manual" | "smart";
  /** Optional description of the smart collection's query. */
  queryDescription?: ReactNode;
  icon?: ReactNode;
  onSelect?: () => void;
}

export const CollectionCard = forwardRef<HTMLButtonElement, CollectionCardProps>(
  function CollectionCard(
    {
      className,
      name,
      count,
      kind = "manual",
      queryDescription,
      icon,
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
        onClick={(e) => {
          onSelect?.();
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
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <span className="grid h-7 w-7 place-items-center rounded-md bg-surface text-text-3">
              {icon ?? (kind === "smart" ? <Sliders size={14} /> : <Folder size={14} />)}
            </span>
            <div className="text-ui font-medium text-text">{name}</div>
          </div>
          {kind === "smart" && (
            <Pill tone="accent" size="sm">
              <Sliders size={9} /> SMART
            </Pill>
          )}
        </div>
        {queryDescription && (
          <div className="rounded-sm border border-line-2 bg-bg px-2 py-1.5 text-mono text-text-3">
            {queryDescription}
          </div>
        )}
        <div className="text-small text-text-3">
          <span className="font-mono tabular-nums text-text-2">{count}</span>
          <span> assets</span>
        </div>
      </button>
    );
  },
);
