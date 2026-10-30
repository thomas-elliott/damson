import { type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface SegmentedControlOption<T extends string> {
  value: T;
  label?: ReactNode;
  icon?: ReactNode;
  "aria-label"?: string;
}

export interface SegmentedControlProps<T extends string> {
  value: T;
  onValueChange: (value: T) => void;
  options: SegmentedControlOption<T>[];
  className?: string;
  "aria-label"?: string;
}

export function SegmentedControl<T extends string>({
  value,
  onValueChange,
  options,
  className,
  "aria-label": ariaLabel,
}: SegmentedControlProps<T>) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className={cn("inline-flex rounded-full bg-control p-0.5", className)}
    >
      {options.map((opt) => {
        const selected = opt.value === value;
        const onlyIcon = opt.icon && !opt.label;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={opt["aria-label"]}
            onClick={() => onValueChange(opt.value)}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full font-display text-label font-semibold uppercase tracking-wider text-subtle-foreground transition-colors duration-fast ease-standard hover:text-foreground",
              onlyIcon ? "h-6 w-7 justify-center" : "h-6 px-2.5",
              selected && "bg-foreground text-background hover:text-background",
            )}
          >
            {opt.icon}
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
