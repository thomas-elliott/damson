import { forwardRef, type HTMLAttributes } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const badge = cva("inline-flex items-center gap-1 whitespace-nowrap border font-medium", {
  variants: {
    tone: {
      neutral: "bg-muted text-muted-foreground border-border",
      accent: "bg-primary-soft text-primary border-transparent",
      ok: "bg-success-soft text-success border-transparent",
      warn: "bg-warning-soft text-warning border-transparent",
      err: "bg-destructive-soft text-destructive border-transparent",
      unk: "bg-unknown-soft text-unknown border-transparent",
    },
    shape: {
      rect: "rounded-sm",
      pill: "rounded-full",
    },
    size: {
      sm: "h-[20px] px-1.5 text-small",
      md: "h-5 px-2 text-small",
    },
    mono: { true: "font-mono", false: "" },
  },
  defaultVariants: {
    tone: "neutral",
    shape: "rect",
    size: "sm",
    mono: false,
  },
});

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badge> {}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(function Badge(
  { className, tone, shape, size, mono, ...props },
  ref,
) {
  return (
    <span ref={ref} className={cn(badge({ tone, shape, size, mono }), className)} {...props} />
  );
});
