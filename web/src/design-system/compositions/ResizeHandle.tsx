import { useCallback, useEffect, useRef, useState } from "react";

import { cn } from "@/design-system/utils";

export interface ResizeHandleProps {
  /** Current width (px). */
  value: number;
  /** Called continuously as the user drags. */
  onChange: (next: number) => void;
  /** Min/max clamps. */
  min?: number;
  max?: number;
  /** `left` = drag right expands (handle on the right edge of the resized element). */
  direction?: "left" | "right";
  className?: string;
  "aria-label"?: string;
}

/**
 * A 4px-wide vertical draggable divider. Doesn't own the sized element —
 * the consumer reads `value` and applies it (usually as a width style).
 * Keyboard: ArrowLeft/ArrowRight by 8px, Shift for 32px.
 */
export function ResizeHandle({
  value,
  onChange,
  min = 200,
  max = 380,
  direction = "right",
  className,
  "aria-label": ariaLabel = "Resize",
}: ResizeHandleProps) {
  const [dragging, setDragging] = useState(false);
  const startX = useRef(0);
  const startVal = useRef(0);

  const onPointerDown = useCallback(
    (e: React.PointerEvent) => {
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
      startX.current = e.clientX;
      startVal.current = value;
      setDragging(true);
    },
    [value],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent) => {
      if (!dragging) return;
      const delta = e.clientX - startX.current;
      const next = direction === "right" ? startVal.current + delta : startVal.current - delta;
      onChange(Math.max(min, Math.min(max, next)));
    },
    [dragging, direction, onChange, min, max],
  );

  const onPointerUp = useCallback((e: React.PointerEvent) => {
    (e.target as HTMLElement).releasePointerCapture(e.pointerId);
    setDragging(false);
  }, []);

  useEffect(() => {
    if (!dragging) return;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    return () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
  }, [dragging]);

  return (
    <button
      type="button"
      role="separator"
      aria-label={ariaLabel}
      aria-orientation="vertical"
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      data-dragging={dragging ? "true" : undefined}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onKeyDown={(e) => {
        const step = e.shiftKey ? 32 : 8;
        if (e.key === "ArrowLeft") {
          e.preventDefault();
          const delta = direction === "right" ? -step : step;
          onChange(Math.max(min, Math.min(max, value + delta)));
        } else if (e.key === "ArrowRight") {
          e.preventDefault();
          const delta = direction === "right" ? step : -step;
          onChange(Math.max(min, Math.min(max, value + delta)));
        }
      }}
      className={cn(
        "group relative h-full w-1 cursor-col-resize bg-transparent transition-colors duration-fast ease-standard",
        "hover:bg-ring/30",
        "data-[dragging=true]:bg-primary",
        "focus-visible:bg-ring/50 focus-visible:outline-hidden",
        className,
      )}
    >
      <span aria-hidden className="sr-only">
        {ariaLabel}
      </span>
    </button>
  );
}
