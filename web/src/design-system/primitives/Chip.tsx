import { forwardRef, type HTMLAttributes } from "react";
import { X } from "lucide-react";

import { cn } from "@/design-system/utils";

interface ChipBaseProps extends HTMLAttributes<HTMLSpanElement> {
  label?: string;
}

// A removable chip must say what its button removes: several render side by
// side, and "Remove" alone does not tell a screen-reader user which.
export type ChipProps = ChipBaseProps &
  (
    | { onRemove?: undefined; removeLabel?: undefined }
    | {
        onRemove: () => void;
        /** The remove button's accessible name, e.g. "Remove type filter: Image". */
        removeLabel: string;
      }
  );

export const Chip = forwardRef<HTMLSpanElement, ChipProps>(function Chip(
  { className, label, onRemove, removeLabel, children, ...props },
  ref,
) {
  return (
    <span
      ref={ref}
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-full bg-control pl-2.5 pr-1 text-label text-muted-foreground",
        className,
      )}
      {...props}
    >
      {label && <span className="text-subtle-foreground">{label}</span>}
      <span className="font-medium text-foreground">{children}</span>
      {onRemove && (
        <button
          type="button"
          aria-label={removeLabel}
          onClick={onRemove}
          className="grid h-[18px] w-[18px] place-items-center rounded-full text-subtle-foreground transition-colors duration-fast ease-standard hover:bg-control-hover hover:text-foreground"
        >
          <X size={11} />
        </button>
      )}
    </span>
  );
});
