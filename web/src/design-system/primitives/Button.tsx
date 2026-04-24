import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const button = cva(
  [
    "inline-flex items-center justify-center gap-1.5 rounded-sm font-medium whitespace-nowrap",
    "transition-colors duration-std ease",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
    "disabled:cursor-not-allowed disabled:opacity-40",
  ],
  {
    variants: {
      variant: {
        // Filled — primary call to action.
        primary: [
          "bg-accent text-accent-fg border border-transparent",
          "hover:bg-accent-2",
          "active:bg-accent-2",
          "data-[pressed=true]:bg-accent-2",
        ],
        // Bordered neutral — visible weight at rest.
        secondary: [
          "bg-surface text-text-2 border border-line",
          "hover:bg-surface-2 hover:text-text hover:border-line-2",
          "active:bg-surface-2",
          "data-[pressed=true]:bg-surface-2 data-[pressed=true]:text-text data-[pressed=true]:border-line-2",
        ],
        // Minimal — transparent at rest, reveals on hover.
        ghost: [
          "bg-transparent text-text-3 border border-transparent",
          "hover:bg-surface hover:text-text",
          "active:bg-surface",
          "data-[pressed=true]:bg-surface data-[pressed=true]:text-text",
        ],
        // Destructive — muted red tint with red text.
        danger: [
          "bg-err-tint text-err border border-err/40",
          "hover:bg-err/20 hover:border-err/60",
          "active:bg-err/25",
          "data-[pressed=true]:bg-err/25",
        ],
      },
      size: {
        sm: "h-6 px-2 text-xs",
        md: "h-7 px-2.5 text-sm-plus",
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

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  function Button(
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
  },
);
