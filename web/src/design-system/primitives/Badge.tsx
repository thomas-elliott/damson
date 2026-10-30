import { forwardRef, type HTMLAttributes } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

import { StatusDot } from "./StatusDot";

// A status badge is a lit dot in a sunken pill; the text stays foreground and
// the dot carries the tone. Neutral has no dot, for plain labels.
const well = "bg-background-inset inset-shadow-well";

const badge = cva(
  "inline-flex items-center gap-1.5 whitespace-nowrap font-medium text-foreground",
  {
    variants: {
      tone: {
        neutral: [well, "text-muted-foreground"],
        // Accent is a count or a label (filters applied, smart collection), not
        // a status, so it is a solid brand fill rather than a dot.
        accent: "bg-primary text-primary-foreground",
        ok: well,
        warn: well,
        err: well,
        unk: well,
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
  { className, tone, shape, size, mono, children, ...props },
  ref,
) {
  const status = tone === "ok" || tone === "warn" || tone === "err" || tone === "unk";
  return (
    <span ref={ref} className={cn(badge({ tone, shape, size, mono }), className)} {...props}>
      {status && <StatusDot tone={tone} />}
      {children}
    </span>
  );
});
