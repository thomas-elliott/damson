import { forwardRef, type ComponentPropsWithoutRef, type ReactNode } from "react";
import * as RadixTooltip from "@radix-ui/react-tooltip";

import { cn } from "@/design-system/utils";

/** Global provider — mount once near the app root. */
export function TooltipProvider({ children }: { children: ReactNode }) {
  return (
    <RadixTooltip.Provider delayDuration={250} skipDelayDuration={200}>
      {children}
    </RadixTooltip.Provider>
  );
}

export const Tooltip = RadixTooltip.Root;
export const TooltipTrigger = RadixTooltip.Trigger;

export const TooltipContent = forwardRef<
  React.ElementRef<typeof RadixTooltip.Content>,
  ComponentPropsWithoutRef<typeof RadixTooltip.Content>
>(function TooltipContent(
  { className, sideOffset = 8, side = "right", ...props },
  ref,
) {
  return (
    <RadixTooltip.Portal>
      <RadixTooltip.Content
        ref={ref}
        side={side}
        sideOffset={sideOffset}
        className={cn(
          "z-tooltip rounded-sm border border-line bg-surface-2 px-2 py-1 font-mono text-2xs text-text",
          "animate-fade-in data-[state=closed]:animate-none",
          className,
        )}
        {...props}
      />
    </RadixTooltip.Portal>
  );
});

/** Convenience — label that wraps a single trigger and just shows text. */
export function SimpleTooltip({
  label,
  children,
  side = "right",
}: {
  label: ReactNode;
  children: ReactNode;
  side?: RadixTooltip.TooltipContentProps["side"];
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side={side}>{label}</TooltipContent>
    </Tooltip>
  );
}
