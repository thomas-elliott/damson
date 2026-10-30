import { cn, cva, type VariantProps } from "@/design-system/utils";

const dot = cva("inline-block rounded-full", {
  variants: {
    tone: {
      accent: "bg-primary shadow-[0_0_6px_var(--primary)]",
      ok: "bg-success shadow-[0_0_6px_var(--success)]",
      warn: "bg-warning shadow-[0_0_6px_var(--warning)]",
      err: "bg-destructive shadow-[0_0_6px_var(--destructive)]",
      unk: "bg-unknown",
      neutral: "bg-subtle-foreground",
    },
    size: {
      sm: "h-1.5 w-1.5",
      md: "h-2 w-2",
    },
  },
  defaultVariants: { tone: "unk", size: "sm" },
});

export interface StatusDotProps extends VariantProps<typeof dot> {
  className?: string;
  ring?: boolean;
}

export function StatusDot({ tone, size, ring, className }: StatusDotProps) {
  return (
    <span aria-hidden className={cn(dot({ tone, size }), ring && "ring-2 ring-card", className)} />
  );
}
