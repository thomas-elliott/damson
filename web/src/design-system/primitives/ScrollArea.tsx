import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixScrollArea from "@radix-ui/react-scroll-area";

import { cn } from "@/design-system/utils";

export const ScrollArea = forwardRef<
  React.ElementRef<typeof RadixScrollArea.Root>,
  ComponentPropsWithoutRef<typeof RadixScrollArea.Root>
>(function ScrollArea({ className, children, ...props }, ref) {
  return (
    <RadixScrollArea.Root
      ref={ref}
      className={cn("relative overflow-hidden", className)}
      {...props}
    >
      {/* tabIndex={0} satisfies axe's scrollable-region-focusable rule — the
       * viewport can scroll, so it needs keyboard focus. */}
      <RadixScrollArea.Viewport tabIndex={0} className="h-full w-full">
        {children}
      </RadixScrollArea.Viewport>
      <ScrollBar orientation="vertical" />
      <ScrollBar orientation="horizontal" />
      <RadixScrollArea.Corner />
    </RadixScrollArea.Root>
  );
});

const ScrollBar = forwardRef<
  React.ElementRef<typeof RadixScrollArea.ScrollAreaScrollbar>,
  ComponentPropsWithoutRef<typeof RadixScrollArea.ScrollAreaScrollbar>
>(function ScrollBar({ className, orientation = "vertical", ...props }, ref) {
  return (
    <RadixScrollArea.Scrollbar
      ref={ref}
      orientation={orientation}
      className={cn(
        "flex touch-none select-none p-0.5 transition-colors duration-fast ease-standard",
        orientation === "vertical" && "h-full w-2",
        orientation === "horizontal" && "h-2 w-full flex-col",
        className,
      )}
      {...props}
    >
      <RadixScrollArea.Thumb
        className={cn(
          "relative flex-1 rounded-full bg-border-strong transition-colors duration-fast ease-standard hover:bg-faint-foreground",
        )}
      />
    </RadixScrollArea.Scrollbar>
  );
});
