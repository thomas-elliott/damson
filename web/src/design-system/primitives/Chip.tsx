import { forwardRef, type HTMLAttributes } from "react";
import { X } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface ChipProps extends HTMLAttributes<HTMLSpanElement> {
  label?: string;
  onRemove?: () => void;
}

export const Chip = forwardRef<HTMLSpanElement, ChipProps>(function Chip(
  { className, label, onRemove, children, ...props },
  ref,
) {
  return (
    <span
      ref={ref}
      className={cn(
        "inline-flex h-[22px] items-center gap-1 rounded-pill border border-line bg-panel pl-2 pr-0.5 text-xs text-text-2",
        className,
      )}
      {...props}
    >
      {label && <span className="text-text-3">{label}</span>}
      <span className="font-medium text-text">{children}</span>
      {onRemove && (
        <button
          type="button"
          aria-label="Remove"
          onClick={onRemove}
          className="grid h-[18px] w-[18px] place-items-center rounded-[3px] text-text-3 transition-colors duration-std ease hover:bg-surface-2 hover:text-text"
        >
          <X size={11} />
        </button>
      )}
    </span>
  );
});
