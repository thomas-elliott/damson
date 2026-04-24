import { forwardRef, type HTMLAttributes, type ReactNode } from "react";
import { X } from "lucide-react";

import { cn } from "@/design-system/utils";
import { IconButton } from "@/design-system/primitives/IconButton";
import { ScrollArea } from "@/design-system/primitives/ScrollArea";

export interface DetailPanelProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  title: ReactNode;
  /** Right-aligned header actions; sits next to the close button. */
  actions?: ReactNode;
  /** Called when the user clicks the close button or presses Escape. */
  onClose?: () => void;
  /** Override width (px). Defaults to `--ds-detail-w` (440px). */
  width?: number | string;
  children: ReactNode;
  /** `overlay` floats above main (box-shadow, z-detail). `flush` sits in flow. */
  variant?: "overlay" | "flush";
}

/**
 * Slide-in side panel with a sticky header and a scrollable body. Use
 * `variant="overlay"` for the asset-detail-over-grid case, `variant="flush"`
 * when it's part of a full-page layout (FullPageView supplies its own side).
 */
export const DetailPanel = forwardRef<HTMLElement, DetailPanelProps>(
  function DetailPanel(
    {
      className,
      title,
      actions,
      onClose,
      width,
      variant = "overlay",
      children,
      ...props
    },
    ref,
  ) {
    const style: React.CSSProperties = width
      ? { width: typeof width === "number" ? `${width}px` : width }
      : {};
    return (
      <aside
        ref={ref}
        aria-label="Details"
        className={cn(
          "flex h-full flex-col border-l border-line bg-panel text-text",
          variant === "overlay" &&
            "absolute right-0 top-0 z-detail animate-slide-in-right shadow-elevated",
          !width && variant === "overlay" && "w-detail",
          !width && variant === "flush" && "w-full",
          className,
        )}
        style={style}
        {...props}
      >
        <header className="flex shrink-0 items-center gap-2 border-b border-line px-3.5 py-2.5">
          <div className="flex-1 truncate text-md font-semibold text-text">
            {title}
          </div>
          {actions && <div className="flex items-center gap-1">{actions}</div>}
          {onClose && (
            <IconButton aria-label="Close details" onClick={onClose}>
              <X size={14} />
            </IconButton>
          )}
        </header>
        <ScrollArea className="min-h-0 flex-1">
          <div className="px-3.5 py-3">{children}</div>
        </ScrollArea>
      </aside>
    );
  },
);
