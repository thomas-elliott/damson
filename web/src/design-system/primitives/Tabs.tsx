import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixTabs from "@radix-ui/react-tabs";

import { cn } from "@/design-system/utils";

export const Tabs = RadixTabs.Root;

export const TabsList = forwardRef<
  React.ElementRef<typeof RadixTabs.List>,
  ComponentPropsWithoutRef<typeof RadixTabs.List>
>(function TabsList({ className, ...props }, ref) {
  return (
    <RadixTabs.List
      ref={ref}
      className={cn("flex w-fit items-center gap-0.5 rounded-full bg-control p-0.5", className)}
      {...props}
    />
  );
});

export const TabsTrigger = forwardRef<
  React.ElementRef<typeof RadixTabs.Trigger>,
  ComponentPropsWithoutRef<typeof RadixTabs.Trigger>
>(function TabsTrigger({ className, ...props }, ref) {
  return (
    <RadixTabs.Trigger
      ref={ref}
      className={cn(
        "inline-flex h-7 items-center gap-1.5 rounded-full px-3 font-display text-label font-semibold uppercase tracking-wider text-subtle-foreground transition-colors duration-fast ease-standard hover:text-foreground",
        "data-[state=active]:bg-popover data-[state=active]:text-foreground data-[state=active]:shadow-sm",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
});

export const TabsContent = RadixTabs.Content;

/** Small count badge to sit alongside a TabsTrigger label. */
export function TabCount({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-mono font-normal normal-case tracking-normal text-subtle-foreground">
      {children}
    </span>
  );
}
