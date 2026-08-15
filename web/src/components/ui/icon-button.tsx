import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";

import { cn } from "@/design-system/utils/index";

const iconButtonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center",
    "rounded-md transition-colors",
    "outline-none focus-visible:ring-2 focus-visible:ring-ring",
    "focus-visible:ring-offset-1 focus-visible:ring-offset-background",
    "disabled:pointer-events-none disabled:opacity-50",
    "[&_svg]:pointer-events-none",
  ],
  {
    variants: {
      variant: {
        primary: [
          "bg-primary text-primary-foreground border border-transparent",
          "hover:bg-primary/90 active:bg-primary/85 data-[pressed=true]:bg-primary/85",
        ],
        secondary: [
          "bg-card text-foreground border border-border",
          "hover:bg-accent hover:border-border-strong",
          "active:bg-background-active data-[pressed=true]:bg-accent",
        ],
        ghost: [
          "bg-transparent text-muted-foreground border border-transparent",
          "hover:bg-accent hover:text-foreground",
          "active:bg-background-active data-[pressed=true]:bg-accent data-[pressed=true]:text-foreground",
        ],
        destructive: [
          "bg-destructive text-destructive-foreground border border-transparent",
          "hover:bg-destructive/90 active:bg-destructive/85 data-[pressed=true]:bg-destructive/85",
        ],
      },
      size: {
        sm: "size-7 [&_svg:not([class*='size-'])]:size-3.5",
        md: "size-8 [&_svg:not([class*='size-'])]:size-4",
        lg: "size-9 [&_svg:not([class*='size-'])]:size-4",
      },
    },
    defaultVariants: { variant: "ghost", size: "md" },
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
 * A square, icon-only button. Sizes track Button's `h-7 / h-8 / h-9`, so the two
 * line up in a toolbar; the variant set and their class lists are Button's, so a
 * ghost icon and a ghost label behave identically on hover.
 *
 * Defaults differ from Button on purpose: `ghost`, because an icon-only control
 * is nearly always a toolbar affordance and a row of filled squares reads as a
 * mistake.
 *
 * Not here, because no consumer has needed them: `asChild` and `loading`. It does
 * take a `ref`, which Radix's `asChild` triggers require to position a popover or
 * tooltip against it.
 */
export function IconButton({
  className,
  variant,
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
      className={cn(iconButtonVariants({ variant, size }), className)}
      {...props}
    >
      {children}
    </button>
  );
}

export { iconButtonVariants };
