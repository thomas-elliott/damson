import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixCheckbox from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface CheckboxProps
  extends ComponentPropsWithoutRef<typeof RadixCheckbox.Root> {
  /** Accessible label; required when a visible label isn't provided elsewhere. */
  "aria-label"?: string;
}

export const Checkbox = forwardRef<
  React.ElementRef<typeof RadixCheckbox.Root>,
  CheckboxProps
>(function Checkbox({ className, ...props }, ref) {
  return (
    <RadixCheckbox.Root
      ref={ref}
      className={cn(
        "grid h-[14px] w-[14px] shrink-0 place-items-center rounded-[3px] border border-line-2 bg-bg text-accent-fg transition-colors duration-std ease",
        "hover:border-text-4",
        "data-[state=checked]:border-transparent data-[state=checked]:bg-accent",
        "data-[state=indeterminate]:border-transparent data-[state=indeterminate]:bg-accent",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <RadixCheckbox.Indicator className="grid place-items-center">
        <Check size={10} strokeWidth={3} />
      </RadixCheckbox.Indicator>
    </RadixCheckbox.Root>
  );
});
