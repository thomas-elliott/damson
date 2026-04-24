import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const iconButton = cva(
  "grid place-items-center rounded-sm text-text-3 transition-colors duration-std ease hover:bg-surface hover:text-text disabled:cursor-not-allowed disabled:opacity-40",
  {
    variants: {
      size: {
        sm: "h-6 w-6",
        md: "h-[34px] w-[34px]",
      },
      tone: {
        neutral: "",
        accent:
          "text-text-2 data-[active=true]:text-text data-[active=true]:bg-surface",
      },
    },
    defaultVariants: { size: "sm", tone: "neutral" },
  },
);

export interface IconButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof iconButton> {
  "aria-label": string;
  active?: boolean;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  function IconButton(
    { className, size, tone, active, children, type = "button", ...props },
    ref,
  ) {
    return (
      <button
        ref={ref}
        type={type}
        data-active={active ? "true" : undefined}
        className={cn(iconButton({ size, tone }), className)}
        {...props}
      >
        {children}
      </button>
    );
  },
);
