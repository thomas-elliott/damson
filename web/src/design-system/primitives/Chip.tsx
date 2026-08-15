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
        "inline-flex h-[22px] items-center gap-1 rounded-full border border-border bg-card pl-2 pr-0.5 text-small text-muted-foreground",
        className,
      )}
      {...props}
    >
      {label && <span className="text-subtle-foreground">{label}</span>}
      <span className="font-medium text-foreground">{children}</span>
      {onRemove && (
        <button
          type="button"
          aria-label="Remove"
          onClick={onRemove}
          className="grid h-[18px] w-[18px] place-items-center rounded-[3px] text-subtle-foreground transition-colors duration-std ease hover:bg-accent hover:text-foreground"
        >
          <X size={11} />
        </button>
      )}
    </span>
  );
});
