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
      className={cn("flex items-center gap-2 border-b border-border", className)}
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
        "-mb-px inline-flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-ui text-subtle-foreground transition-colors duration-fast ease-standard hover:text-foreground",
        "data-[state=active]:border-primary data-[state=active]:text-primary",
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
    <span className="inline-flex h-[20px] items-center rounded-full bg-muted px-1.5 text-mono text-subtle-foreground">
      {children}
    </span>
  );
}
