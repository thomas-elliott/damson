import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixSeparator from "@radix-ui/react-separator";

import { cn } from "@/design-system/utils";

export type SeparatorProps = ComponentPropsWithoutRef<typeof RadixSeparator.Root>;

export const Separator = forwardRef<React.ElementRef<typeof RadixSeparator.Root>, SeparatorProps>(
  function Separator({ className, orientation = "horizontal", decorative = true, ...props }, ref) {
    return (
      <RadixSeparator.Root
        ref={ref}
        orientation={orientation}
        decorative={decorative}
        className={cn(
          "shrink-0 bg-border",
          orientation === "horizontal" ? "h-px w-full" : "h-[18px] w-px",
          className,
        )}
        {...props}
      />
    );
  },
);
