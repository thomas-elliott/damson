import { forwardRef, type ButtonHTMLAttributes } from "react";
import { Plus } from "lucide-react";

import { cn } from "@/design-system/utils";

// Keyed by a tag's first path segment, which for store tags is the top-level
// store category.
const CATEGORY_BG: Record<string, string> = {
  "3d": "bg-category-3d",
  "2d": "bg-category-2d",
  audio: "bg-category-audio",
  vfx: "bg-category-vfx",
  tools: "bg-category-tools",
};

function categoryBg(path: string) {
  return CATEGORY_BG[path.split("/")[0].toLowerCase()] ?? "bg-category-other";
}

export interface TagProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "solid" | "add";
}

/**
 * A tag, cut like a luggage tag and tinted by its top-level category. A string
 * child is read as a path (`3d/character/humanoid`) and shown step by step.
 */
export const Tag = forwardRef<HTMLButtonElement, TagProps>(function Tag(
  { className, variant = "solid", children, type = "button", ...props },
  ref,
) {
  if (variant === "add") {
    return (
      <button
        ref={ref}
        type={type}
        className={cn(
          "inline-flex h-[22px] items-center gap-1 rounded-full border border-dashed border-border-strong px-2 text-label text-subtle-foreground transition-colors duration-fast ease-standard hover:border-faint-foreground hover:text-muted-foreground",
          className,
        )}
        {...props}
      >
        <Plus size={10} />
        {children}
      </button>
    );
  }

  const path = typeof children === "string" ? children : null;
  return (
    // The shape sits on an inner span: clip-path on the button would clip its
    // focus outline too.
    <button ref={ref} type={type} className={cn("inline-flex rounded-xs", className)} {...props}>
      <span
        className={cn(
          "inline-flex h-[22px] items-center gap-1 rounded-r-xs pl-3.5 pr-2 text-label font-semibold text-foreground transition-[filter] duration-fast ease-standard hover:brightness-110",
          // The point and the punched hole of a luggage tag.
          "[clip-path:polygon(6px_0,100%_0,100%_100%,6px_100%,0_50%)]",
          "[mask-image:radial-gradient(circle_at_7px_50%,transparent_2px,black_2.5px)]",
          path ? categoryBg(path) : "bg-category-other",
        )}
      >
        {path
          ? path.split("/").map((step, i) => (
              <span key={i} className="inline-flex items-center gap-1">
                {i > 0 && <span className="font-normal text-subtle-foreground">/</span>}
                {step}
              </span>
            ))
          : children}
      </span>
    </button>
  );
});
