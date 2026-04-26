import { type CSSProperties, type ReactNode } from "react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const glyph = cva(
  "inline-grid place-items-center rounded-md font-semibold text-text",
  {
    variants: {
      size: {
        sm: "h-7 w-7 text-[14px]",
        md: "h-9 w-9 text-[14px]",
        lg: "h-12 w-12 text-[16px]",
      },
    },
    defaultVariants: { size: "md" },
  },
);

export interface GlyphProps extends VariantProps<typeof glyph> {
  children: ReactNode;
  /** OKLCH or hex background colour; defaults to `--ds-surface`. */
  color?: string;
  /** Override the foreground colour; defaults to `--ds-text`. Use for low-contrast backgrounds. */
  textColor?: string;
  className?: string;
  style?: CSSProperties;
}

export function Glyph({
  children,
  color,
  textColor,
  size,
  className,
  style,
}: GlyphProps) {
  return (
    <span
      aria-hidden
      className={cn(glyph({ size }), className)}
      style={{
        background: color ?? "var(--ds-surface)",
        color: textColor,
        ...style,
      }}
    >
      {children}
    </span>
  );
}
