import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixSelect from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";

import { cn } from "@/design-system/utils";

export const Select = RadixSelect.Root;
export const SelectGroup = RadixSelect.Group;
export const SelectValue = RadixSelect.Value;

export const SelectTrigger = forwardRef<
  React.ElementRef<typeof RadixSelect.Trigger>,
  ComponentPropsWithoutRef<typeof RadixSelect.Trigger>
>(function SelectTrigger({ className, children, ...props }, ref) {
  return (
    <RadixSelect.Trigger
      ref={ref}
      className={cn(
        "inline-flex h-7 items-center justify-between gap-2 rounded-sm border border-line bg-bg px-2 text-ui text-text transition-colors duration-std ease",
        "hover:border-line-2",
        "data-[state=open]:border-transparent data-[state=open]:outline data-[state=open]:outline-2 data-[state=open]:outline-accent-ring",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      {children}
      <RadixSelect.Icon asChild>
        <ChevronDown size={12} className="text-text-3" />
      </RadixSelect.Icon>
    </RadixSelect.Trigger>
  );
});

export const SelectContent = forwardRef<
  React.ElementRef<typeof RadixSelect.Content>,
  ComponentPropsWithoutRef<typeof RadixSelect.Content>
>(function SelectContent({ className, position = "popper", sideOffset = 4, style, ...props }, ref) {
  return (
    <RadixSelect.Portal>
      <RadixSelect.Content
        ref={ref}
        position={position}
        sideOffset={sideOffset}
        // Force z-index inline so the Select floats above a Radix Dialog
        // (which sits at z:500). Using the class alone failed to win the
        // stacking war in practice — Radix Popper applies its own inline
        // style on the same element, and any future change to merge/order
        // could silently drop the `z-popover` class. Inline wins.
        style={{ zIndex: 600, ...style }}
        className={cn(
          "z-popover overflow-hidden rounded-md border border-line bg-panel text-ui text-text-2 shadow-elevated",
          "data-[state=open]:animate-fade-in",
          position === "popper" && "min-w-[var(--radix-select-trigger-width)]",
          className,
        )}
        {...props}
      >
        <RadixSelect.ScrollUpButton className="flex cursor-default items-center justify-center py-1 text-text-3">
          <ChevronUp size={12} />
        </RadixSelect.ScrollUpButton>
        <RadixSelect.Viewport className="p-1">{props.children}</RadixSelect.Viewport>
        <RadixSelect.ScrollDownButton className="flex cursor-default items-center justify-center py-1 text-text-3">
          <ChevronDown size={12} />
        </RadixSelect.ScrollDownButton>
      </RadixSelect.Content>
    </RadixSelect.Portal>
  );
});

export const SelectItem = forwardRef<
  React.ElementRef<typeof RadixSelect.Item>,
  ComponentPropsWithoutRef<typeof RadixSelect.Item>
>(function SelectItem({ className, children, ...props }, ref) {
  return (
    <RadixSelect.Item
      ref={ref}
      className={cn(
        "relative flex cursor-pointer select-none items-center gap-2 rounded-sm py-1.5 pl-7 pr-2 text-ui outline-hidden transition-colors duration-std ease",
        "data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50",
        "data-[highlighted]:bg-surface data-[highlighted]:text-text",
        className,
      )}
      {...props}
    >
      <span className="absolute left-2 grid h-4 w-4 place-items-center text-accent">
        <RadixSelect.ItemIndicator>
          <Check size={12} strokeWidth={3} />
        </RadixSelect.ItemIndicator>
      </span>
      <RadixSelect.ItemText>{children}</RadixSelect.ItemText>
    </RadixSelect.Item>
  );
});

export const SelectLabel = forwardRef<
  React.ElementRef<typeof RadixSelect.Label>,
  ComponentPropsWithoutRef<typeof RadixSelect.Label>
>(function SelectLabel({ className, ...props }, ref) {
  return (
    <RadixSelect.Label
      ref={ref}
      className={cn("px-2 py-1 text-label text-text-4", className)}
      {...props}
    />
  );
});

export const SelectSeparator = forwardRef<
  React.ElementRef<typeof RadixSelect.Separator>,
  ComponentPropsWithoutRef<typeof RadixSelect.Separator>
>(function SelectSeparator({ className, ...props }, ref) {
  return (
    <RadixSelect.Separator ref={ref} className={cn("my-1 h-px bg-line", className)} {...props} />
  );
});
