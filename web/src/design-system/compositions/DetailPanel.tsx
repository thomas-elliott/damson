import { forwardRef, useEffect, useId, useRef, type HTMLAttributes, type ReactNode } from "react";
import { X } from "lucide-react";

import { cn } from "@/design-system/utils";
import { IconButton } from "@/design-system/primitives/IconButton";
import { ScrollArea } from "@/design-system/primitives/ScrollArea";

import { ResizeHandle } from "./ResizeHandle";

export interface DetailPanelProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  title: ReactNode;
  /** Right-aligned header actions; sits next to the close button. */
  actions?: ReactNode;
  /** Called when the user clicks the close button or presses Escape. */
  onClose?: () => void;
  /** Override width (px). Defaults to `--detail-w` (440px). */
  width?: number | string;
  children: ReactNode;
  /** `overlay` floats above main (box-shadow, z-detail). `flush` sits in flow. */
  variant?: "overlay" | "flush";
  /** When set, renders a left-edge drag handle. Overlay variant only;
   *  ignored in fullscreen. Caller controls width via `width`/`onWidthChange`. */
  resizable?: boolean;
  /** Called continuously as the user drags the resize handle. */
  onWidthChange?: (next: number) => void;
  /** Min/max clamp for the resize handle (px). */
  minWidth?: number;
  maxWidth?: number;
  /** When true, the panel fills its positioned ancestor (covers main area).
   *  `width` is ignored. Slide-in animation still plays on first mount. */
  fullscreen?: boolean;
}

// An overlay opens over whatever the user was working in, so it takes focus
// to its heading as it mounts and gives focus back to what had it when it
// unmounts, unless the user has already moved focus elsewhere.
function useOverlayFocus(enabled: boolean) {
  const headingRef = useRef<HTMLHeadingElement>(null);
  const panelRef = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (!enabled) return;
    const opener = document.activeElement as HTMLElement | null;
    headingRef.current?.focus();
    const panel = panelRef.current;
    return () => {
      const active = document.activeElement;
      const lost = active === null || active === document.body || panel?.contains(active);
      if (lost && opener?.isConnected && opener !== document.body) opener.focus();
    };
  }, [enabled]);
  return { headingRef, panelRef };
}

/**
 * Slide-in side panel with a sticky header and a scrollable body. Use
 * `variant="overlay"` for the asset-detail-over-grid case, `variant="flush"`
 * when it's part of a full-page layout (FullPageView supplies its own side).
 *
 * Pass `resizable` (overlay only) plus `width` + `onWidthChange` to enable
 * left-edge drag-resize. Pass `fullscreen` to make the panel fill its
 * positioned ancestor — the resize handle is hidden in that mode.
 *
 * The panel is named by its title, an `h2`. The overlay moves focus to that
 * heading as it opens and returns it to the opener when it closes.
 */
export const DetailPanel = forwardRef<HTMLElement, DetailPanelProps>(function DetailPanel(
  {
    className,
    title,
    actions,
    onClose,
    width,
    variant = "overlay",
    children,
    resizable,
    onWidthChange,
    minWidth = 320,
    maxWidth,
    fullscreen,
    ...props
  },
  ref,
) {
  const titleId = useId();
  const { headingRef, panelRef } = useOverlayFocus(variant === "overlay");
  const setRefs = (node: HTMLElement | null) => {
    panelRef.current = node;
    if (typeof ref === "function") ref(node);
    else if (ref) ref.current = node;
  };
  const numericWidth = typeof width === "number" ? width : Number.NaN;
  const style: React.CSSProperties =
    !fullscreen && width ? { width: typeof width === "number" ? `${width}px` : width } : {};
  return (
    <aside
      ref={setRefs}
      aria-labelledby={titleId}
      className={cn(
        "flex h-full flex-col border-l border-border bg-card text-foreground",
        variant === "overlay" &&
          !fullscreen &&
          "absolute right-0 top-0 z-detail animate-slide-in-right shadow-lg",
        variant === "overlay" &&
          fullscreen &&
          "absolute inset-0 z-detail animate-slide-in-right shadow-lg",
        !width && variant === "overlay" && !fullscreen && "w-detail",
        !width && variant === "flush" && "w-full",
        className,
      )}
      style={style}
      {...props}
    >
      {resizable && variant === "overlay" && !fullscreen && onWidthChange && (
        <div className="absolute left-[-2px] top-0 z-resizer h-full w-1">
          <ResizeHandle
            value={Number.isFinite(numericWidth) ? numericWidth : minWidth}
            onChange={onWidthChange}
            min={minWidth}
            max={maxWidth ?? 1024}
            direction="left"
            aria-label="Resize details panel"
          />
        </div>
      )}
      <header className="flex shrink-0 items-center gap-2 border-b border-border px-3.5 py-2.5">
        <h2
          ref={headingRef}
          id={titleId}
          tabIndex={-1}
          className="flex-1 truncate text-subhead-semibold text-foreground outline-none"
        >
          {title}
        </h2>
        {actions && <div className="flex items-center gap-1">{actions}</div>}
        {onClose && (
          <IconButton aria-label="Close details" size="sm" onClick={onClose}>
            <X />
          </IconButton>
        )}
      </header>
      <ScrollArea className="min-h-0 flex-1">
        <div className="px-3.5 py-3">{children}</div>
      </ScrollArea>
    </aside>
  );
});
