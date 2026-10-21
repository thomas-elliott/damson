import { Slot, Slottable } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/design-system/utils";

const buttonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap",
    "rounded-md font-medium transition-colors",
    "outline-none focus-visible:ring-2 focus-visible:ring-ring",
    "focus-visible:ring-offset-1 focus-visible:ring-offset-background",
    "disabled:pointer-events-none disabled:opacity-50",
    "[&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4",
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
        // A soft tint, not a fill: a filled red reads fine in a confirm dialog
        // and shouts in a toolbar.
        destructive: [
          "bg-destructive-soft text-destructive border border-destructive/40",
          "hover:bg-destructive/20 hover:border-destructive/60",
          "active:bg-destructive/25 data-[pressed=true]:bg-destructive/25",
        ],
      },
      size: {
        sm: "h-7 px-2.5 text-label [&_svg:not([class*='size-'])]:size-3.5",
        md: "h-8 px-3 text-ui",
        lg: "h-9 px-4 text-ui",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & {
    /** Render as the single child element instead of a `<button>` — the
     * Button-as-link case. */
    asChild?: boolean;
    leadingIcon?: ReactNode;
    /** Swaps the leading icon for a spinner and disables the button. Does NOT
     * hide the label: a button that changes width mid-click moves the pointer. */
    loading?: boolean;
    /** Toggle-button state, styled via `data-pressed`. Set `aria-pressed` too. */
    pressed?: boolean;
  };

export function Button({
  className,
  variant,
  size,
  asChild = false,
  leadingIcon,
  loading = false,
  pressed,
  disabled,
  children,
  type = "button",
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp
      data-slot="button"
      data-pressed={pressed ? "true" : undefined}
      type={asChild ? undefined : type}
      disabled={disabled || loading}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    >
      {loading ? <Loader2 className="animate-spin" aria-hidden /> : leadingIcon}
      {/* With `asChild`, Radix's Slot demands a single element child; the icon
          slot makes two. `Slottable` marks which one to merge into. */}
      <Slottable>{children}</Slottable>
    </Comp>
  );
}

export { buttonVariants };
