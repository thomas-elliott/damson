import { forwardRef, type ButtonHTMLAttributes } from "react";
import { Plus } from "lucide-react";

import { cn, cva, type VariantProps } from "@/design-system/utils";

const tag = cva(
  "inline-flex h-[22px] items-center gap-1 rounded-pill px-2 text-small transition-colors duration-std ease",
  {
    variants: {
      variant: {
        solid: "border border-line bg-surface text-text-2 hover:text-text",
        add: "border border-dashed border-line-2 bg-transparent text-text-3 hover:border-text-4 hover:text-text-2",
      },
    },
    defaultVariants: { variant: "solid" },
  },
);

export interface TagProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof tag> {}

export const Tag = forwardRef<HTMLButtonElement, TagProps>(function Tag(
  { className, variant, children, type = "button", ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={cn(tag({ variant }), className)}
      {...props}
    >
      {variant === "add" && <Plus size={10} />}
      {children}
    </button>
  );
});
