import { cn, cva, type VariantProps } from "@/design-system/utils";

const dot = cva("inline-block rounded-full", {
  variants: {
    tone: {
      accent: "bg-accent",
      ok: "bg-ok",
      warn: "bg-warn",
      err: "bg-err",
      unk: "bg-unk",
      neutral: "bg-text-3",
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
    <span
      aria-hidden
      className={cn(
        dot({ tone, size }),
        ring && "ring-2 ring-panel",
        className,
      )}
    />
  );
}
