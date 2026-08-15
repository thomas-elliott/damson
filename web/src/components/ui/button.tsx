import { Slot, Slottable } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/design-system/utils/index";

// Contract settled 2026-08-15 across damson, mangapack and taskmanager
// (personal-practices/standards/design-system.md, "The component contract"):
//   variants  primary | secondary | ghost | destructive
//   sizes     sm | md | lg
//   default   primary
//
// `outline` and `default` are COMPATIBILITY ALIASES, not part of the contract.
// They exist so an upstream shadcn block pasted in — which will say
// `variant="outline"` or rely on `variant="default"` — renders instead of
// falling back to no variant classes at all. Two lines of insurance.
const PRIMARY = [
  "bg-primary text-primary-foreground border border-transparent",
  "hover:bg-primary/90 active:bg-primary/85 data-[pressed=true]:bg-primary/85",
];

const SECONDARY = [
  "bg-card text-foreground border border-border",
  "hover:bg-accent hover:border-border-strong",
  "active:bg-background-active data-[pressed=true]:bg-accent",
];

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
        primary: PRIMARY,
        secondary: SECONDARY,
        ghost: [
          "bg-transparent text-muted-foreground border border-transparent",
          "hover:bg-accent hover:text-foreground",
          "active:bg-background-active data-[pressed=true]:bg-accent data-[pressed=true]:text-foreground",
        ],
        // Filled, not tinted. damson uses a soft red tint and it reads better in
        // a toolbar, but a confirm dialog needs the strong signal and every
        // other system fills it. Recorded as an open design question in
        // docs/roadmap.md — a `destructive-subtle` variant may follow.
        destructive: [
          "bg-destructive text-destructive-foreground border border-transparent",
          "hover:bg-destructive/90 active:bg-destructive/85 data-[pressed=true]:bg-destructive/85",
        ],
        outline: SECONDARY,
        default: PRIMARY,
      },
      size: {
        sm: "h-7 px-2.5 text-xs [&_svg:not([class*='size-'])]:size-3.5",
        md: "h-8 px-3 text-sm",
        lg: "h-9 px-4 text-sm",
      },
      fullWidth: {
        true: "w-full",
        false: "",
      },
    },
    defaultVariants: { variant: "primary", size: "md", fullWidth: false },
  },
);

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & {
    /** Render as the single child element instead of a `<button>` — the
     * Button-as-link case. shadcn's convention; upstream blocks rely on it. */
    asChild?: boolean;
    leadingIcon?: ReactNode;
    trailingIcon?: ReactNode;
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
  fullWidth,
  asChild = false,
  leadingIcon,
  trailingIcon,
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
      className={cn(buttonVariants({ variant, size, fullWidth }), className)}
      {...props}
    >
      {loading ? <Loader2 className="animate-spin" aria-hidden /> : leadingIcon}
      {/* `Slottable` is required, not decorative. With `asChild`, Radix's Slot
          demands a SINGLE element child, and the icon slots either side make
          three — it throws "Slot failed to slot onto its children. Expected a
          single React element child or `Slottable`". Wrapping `children` marks
          which one Slot should merge into, and lets asChild keep the icons. */}
      <Slottable>{children}</Slottable>
      {trailingIcon}
    </Comp>
  );
}

export { buttonVariants };
