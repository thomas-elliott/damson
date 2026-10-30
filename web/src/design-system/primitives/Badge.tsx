import { forwardRef, type HTMLAttributes } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

// A status badge is a lit dot in a sunken pill; the text stays foreground and
// the dot carries the tone. Neutral has no dot, for plain labels.
const dot = "before:size-1.5 before:shrink-0 before:rounded-full";

const badge = cva(
  "inline-flex items-center gap-1.5 whitespace-nowrap bg-background-inset font-medium text-foreground inset-shadow-well",
  {
    variants: {
      tone: {
        neutral: "text-muted-foreground",
        // Accent is a count or a label (filters applied, smart collection), not
        // a status, so it is a solid brand fill rather than a dot.
        accent: "bg-primary text-primary-foreground inset-shadow-none",
        ok: [dot, "before:bg-success before:shadow-[0_0_6px_var(--success)]"],
        warn: [dot, "before:bg-warning before:shadow-[0_0_6px_var(--warning)]"],
        err: [dot, "before:bg-destructive before:shadow-[0_0_6px_var(--destructive)]"],
        unk: [dot, "before:bg-unknown"],
      },
      shape: {
        rect: "rounded-sm",
        pill: "rounded-full",
      },
      size: {
        sm: "h-5 px-2 text-label",
        md: "h-6 px-2.5 text-label",
      },
      mono: { true: "font-mono", false: "" },
    },
    defaultVariants: {
      tone: "neutral",
      shape: "pill",
      size: "sm",
      mono: false,
    },
  },
);

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badge> {}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(function Badge(
  { className, tone, shape, size, mono, ...props },
  ref,
) {
  return (
    <span ref={ref} className={cn(badge({ tone, shape, size, mono }), className)} {...props} />
  );
});
