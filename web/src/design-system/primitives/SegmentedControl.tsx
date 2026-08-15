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
      className={cn("inline-flex rounded-sm border border-border bg-card p-px", className)}
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
              "inline-flex items-center gap-1.5 rounded-[3px] text-small text-subtle-foreground transition-colors duration-std ease hover:text-foreground",
              onlyIcon ? "h-5 w-6 justify-center" : "h-5 px-1.5",
              selected && "bg-accent text-foreground",
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
