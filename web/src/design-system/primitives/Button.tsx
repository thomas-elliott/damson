import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const button = cva(
  [
    "inline-flex items-center justify-center gap-1.5 rounded-sm font-medium whitespace-nowrap",
    "transition-colors duration-std ease",
    "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
    "disabled:cursor-not-allowed disabled:opacity-40",
  ],
  {
    variants: {
      variant: {
        // Filled — primary call to action.
        primary: [
          "bg-primary text-primary-foreground border border-transparent",
          "hover:bg-primary-hover",
          "active:bg-primary-hover",
          "data-[pressed=true]:bg-primary-hover",
        ],
        // Bordered neutral — visible weight at rest.
        secondary: [
          "bg-muted text-muted-foreground border border-border",
          "hover:bg-accent hover:text-foreground hover:border-border-strong",
          "active:bg-accent",
          "data-[pressed=true]:bg-accent data-[pressed=true]:text-foreground data-[pressed=true]:border-border-strong",
        ],
        // Minimal — transparent at rest, reveals on hover.
        ghost: [
          "bg-transparent text-subtle-foreground border border-transparent",
          "hover:bg-muted hover:text-foreground",
          "active:bg-muted",
          "data-[pressed=true]:bg-muted data-[pressed=true]:text-foreground",
        ],
        // Destructive — muted red tint with red text.
        danger: [
          "bg-destructive-soft text-destructive border border-destructive/40",
          "hover:bg-destructive/20 hover:border-destructive/60",
          "active:bg-destructive/25",
          "data-[pressed=true]:bg-destructive/25",
        ],
      },
      size: {
        sm: "h-6 px-2 text-small",
        md: "h-7 px-2.5 text-ui",
      },
    },
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof button> & {
    leadingIcon?: ReactNode;
    trailingIcon?: ReactNode;
    pressed?: boolean;
  };

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    className,
    variant,
    size,
    leadingIcon,
    trailingIcon,
    pressed,
    children,
    type = "button",
    ...props
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      data-pressed={pressed ? "true" : undefined}
      className={cn(button({ variant, size }), className)}
      {...props}
    >
      {leadingIcon}
      {children}
      {trailingIcon}
    </button>
  );
});
