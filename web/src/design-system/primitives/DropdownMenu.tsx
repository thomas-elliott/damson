import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixDropdown from "@radix-ui/react-dropdown-menu";
import { Check } from "lucide-react";

import { cn } from "@/design-system/utils";

export const DropdownMenu = RadixDropdown.Root;
export const DropdownMenuTrigger = RadixDropdown.Trigger;
export const DropdownMenuGroup = RadixDropdown.Group;
export const DropdownMenuPortal = RadixDropdown.Portal;
export const DropdownMenuSub = RadixDropdown.Sub;
export const DropdownMenuRadioGroup = RadixDropdown.RadioGroup;

export const DropdownMenuContent = forwardRef<
  React.ElementRef<typeof RadixDropdown.Content>,
  ComponentPropsWithoutRef<typeof RadixDropdown.Content>
>(function DropdownMenuContent({ className, sideOffset = 4, style, ...props }, ref) {
  return (
    <RadixDropdown.Portal>
      <RadixDropdown.Content
        ref={ref}
        sideOffset={sideOffset}
        style={{ zIndex: 600, ...style }}
        className={cn(
          "z-popover min-w-[10rem] rounded-md border border-line bg-panel p-1 text-ui text-text-2 shadow-elevated",
          "data-[state=open]:animate-fade-in",
          className,
        )}
        {...props}
      />
    </RadixDropdown.Portal>
  );
});

const itemClasses =
  "relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-ui outline-hidden transition-colors duration-std ease data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 data-[highlighted]:bg-surface data-[highlighted]:text-text";

export const DropdownMenuItem = forwardRef<
  React.ElementRef<typeof RadixDropdown.Item>,
  ComponentPropsWithoutRef<typeof RadixDropdown.Item>
>(function DropdownMenuItem({ className, ...props }, ref) {
  return <RadixDropdown.Item ref={ref} className={cn(itemClasses, className)} {...props} />;
});

export const DropdownMenuCheckboxItem = forwardRef<
  React.ElementRef<typeof RadixDropdown.CheckboxItem>,
  ComponentPropsWithoutRef<typeof RadixDropdown.CheckboxItem>
>(function DropdownMenuCheckboxItem({ className, children, checked, ...props }, ref) {
  return (
    <RadixDropdown.CheckboxItem
      ref={ref}
      checked={checked}
      className={cn(itemClasses, "pl-7", className)}
      {...props}
    >
      <span className="absolute left-2 grid h-4 w-4 place-items-center text-accent">
        <RadixDropdown.ItemIndicator>
          <Check size={12} strokeWidth={3} />
        </RadixDropdown.ItemIndicator>
      </span>
      {children}
    </RadixDropdown.CheckboxItem>
  );
});

export const DropdownMenuRadioItem = forwardRef<
  React.ElementRef<typeof RadixDropdown.RadioItem>,
  ComponentPropsWithoutRef<typeof RadixDropdown.RadioItem>
>(function DropdownMenuRadioItem({ className, children, ...props }, ref) {
  return (
    <RadixDropdown.RadioItem ref={ref} className={cn(itemClasses, "pl-7", className)} {...props}>
      <span className="absolute left-2 grid h-4 w-4 place-items-center text-accent">
        <RadixDropdown.ItemIndicator>
          <span className="h-1.5 w-1.5 rounded-full bg-accent" />
        </RadixDropdown.ItemIndicator>
      </span>
      {children}
    </RadixDropdown.RadioItem>
  );
});

export const DropdownMenuLabel = forwardRef<
  React.ElementRef<typeof RadixDropdown.Label>,
  ComponentPropsWithoutRef<typeof RadixDropdown.Label>
>(function DropdownMenuLabel({ className, ...props }, ref) {
  return (
    <RadixDropdown.Label
      ref={ref}
      className={cn("px-2 py-1 text-label text-text-4", className)}
      {...props}
    />
  );
});

export const DropdownMenuSeparator = forwardRef<
  React.ElementRef<typeof RadixDropdown.Separator>,
  ComponentPropsWithoutRef<typeof RadixDropdown.Separator>
>(function DropdownMenuSeparator({ className, ...props }, ref) {
  return (
    <RadixDropdown.Separator ref={ref} className={cn("my-1 h-px bg-line", className)} {...props} />
  );
});

export const DropdownMenuShortcut = ({
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) => (
  <span className={cn("ml-auto text-mono text-text-4", className)} {...props} />
);
