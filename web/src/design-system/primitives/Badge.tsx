import { forwardRef, type HTMLAttributes } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const badge = cva("inline-flex items-center gap-1 whitespace-nowrap border font-medium", {
  variants: {
    tone: {
      neutral: "bg-surface text-text-2 border-line",
      accent: "bg-accent-tint text-accent border-transparent",
      ok: "bg-ok-tint text-ok border-transparent",
      warn: "bg-warn-tint text-warn border-transparent",
      err: "bg-err-tint text-err border-transparent",
      unk: "bg-unk-tint text-unk border-transparent",
    },
    shape: {
      rect: "rounded-sm",
      pill: "rounded-pill",
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
