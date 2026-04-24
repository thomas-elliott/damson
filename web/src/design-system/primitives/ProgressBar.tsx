import { cn, cva, type VariantProps } from "@/design-system/utils";

const track = cva("w-full overflow-hidden rounded-full bg-line", {
  variants: {
    size: {
      sm: "h-[3px]",
      md: "h-1",
    },
  },
  defaultVariants: { size: "sm" },
});

const fill = cva("h-full transition-[width] duration-200 ease", {
  variants: {
    tone: {
      accent: "bg-accent",
      ok: "bg-ok",
      warn: "bg-warn",
      err: "bg-err",
    },
  },
  defaultVariants: { tone: "accent" },
});

export interface ProgressBarProps extends VariantProps<typeof track> {
  value: number;
  max?: number;
  tone?: VariantProps<typeof fill>["tone"];
  className?: string;
  "aria-label"?: string;
}

export function ProgressBar({
  value,
  max = 100,
  tone,
  size,
  className,
  ...props
}: ProgressBarProps) {
  const pct = Math.max(0, Math.min(100, (value / max) * 100));
  return (
    <div
      role="progressbar"
      aria-valuenow={Math.round(pct)}
      aria-valuemin={0}
      aria-valuemax={100}
      className={cn(track({ size }), className)}
      {...props}
    >
      <div className={fill({ tone })} style={{ width: `${pct}%` }} />
    </div>
  );
}
