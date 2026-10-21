import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";

import { cn } from "@/design-system/utils";

const iconButtonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center",
    "rounded-md transition-colors",
    "outline-none focus-visible:ring-2 focus-visible:ring-ring",
    "focus-visible:ring-offset-1 focus-visible:ring-offset-background",
    "disabled:pointer-events-none disabled:opacity-50",
    "[&_svg]:pointer-events-none",
    "bg-transparent text-muted-foreground border border-transparent",
    "hover:bg-accent hover:text-foreground",
    "active:bg-background-active data-[pressed=true]:bg-accent data-[pressed=true]:text-foreground",
  ],
  {
    variants: {
      size: {
        sm: "size-7 [&_svg:not([class*='size-'])]:size-3.5",
        md: "size-8 [&_svg:not([class*='size-'])]:size-4",
        lg: "size-9 [&_svg:not([class*='size-'])]:size-4",
      },
    },
    defaultVariants: { size: "md" },
  },
);

export type IconButtonProps = ComponentProps<"button"> &
  VariantProps<typeof iconButtonVariants> & {
    /** Required, not optional: the icon is the only content, so without this the
     * button has no accessible name. This is the whole reason the component
     * exists rather than a `size="icon"` on Button — TypeScript refuses the
     * unlabelled call instead of axe reporting it later. */
    "aria-label": string;
    /** Toggle-button state, styled via `data-pressed`. Set `aria-pressed` too. */
    pressed?: boolean;
  };

/**
 * A square, icon-only ghost button. Sizes track Button's `h-7 / h-8 / h-9`, and
 * its classes are Button's `ghost`, so the two line up and hover alike in a
 * toolbar.
 *
 * It takes a `ref`, which Radix's `asChild` triggers require to position a
 * popover or tooltip against it.
 */
export function IconButton({
  className,
  size,
  pressed,
  type = "button",
  children,
  ...props
}: IconButtonProps) {
  return (
    <button
      data-slot="icon-button"
      data-pressed={pressed ? "true" : undefined}
      type={type}
      className={cn(iconButtonVariants({ size }), className)}
      {...props}
    >
      {children}
    </button>
  );
}
