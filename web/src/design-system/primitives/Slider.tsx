import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixSlider from "@radix-ui/react-slider";

import { cn } from "@/design-system/utils";

export type SliderProps = ComponentPropsWithoutRef<typeof RadixSlider.Root>;

export const Slider = forwardRef<
  React.ElementRef<typeof RadixSlider.Root>,
  SliderProps
>(function Slider({ className, ...props }, ref) {
  return (
    <RadixSlider.Root
      ref={ref}
      className={cn(
        "relative flex w-full touch-none select-none items-center",
        className,
      )}
      {...props}
    >
      <RadixSlider.Track className="relative h-[3px] grow overflow-hidden rounded-full bg-line">
        <RadixSlider.Range className="absolute h-full bg-accent" />
      </RadixSlider.Track>
      <RadixSlider.Thumb
        aria-label="Value"
        className="block h-3 w-3 rounded-full border-2 border-panel bg-accent outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring"
      />
    </RadixSlider.Root>
  );
});
