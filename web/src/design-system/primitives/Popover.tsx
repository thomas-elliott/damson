import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixPopover from "@radix-ui/react-popover";

import { cn } from "@/design-system/utils";

export const Popover = RadixPopover.Root;
export const PopoverTrigger = RadixPopover.Trigger;
export const PopoverAnchor = RadixPopover.Anchor;

export const PopoverContent = forwardRef<
  React.ElementRef<typeof RadixPopover.Content>,
  ComponentPropsWithoutRef<typeof RadixPopover.Content>
>(function PopoverContent(
  { className, sideOffset = 6, align = "start", ...props },
  ref,
) {
  return (
    <RadixPopover.Portal>
      <RadixPopover.Content
        ref={ref}
        sideOffset={sideOffset}
        align={align}
        className={cn(
          "z-tooltip rounded-md border border-line bg-panel p-3 text-sm-plus text-text shadow-elevated",
          "data-[state=open]:animate-fade-in",
          className,
        )}
        {...props}
      />
    </RadixPopover.Portal>
  );
});
