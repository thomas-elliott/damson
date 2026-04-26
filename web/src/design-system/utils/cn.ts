import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// The design system uses custom font-size keys (2xs, xs-plus, sm-plus, base,
// md) that are not in tailwind-merge's default groups. Teach tailwind-merge
// about them so utilities like `text-sm-plus` aren't mistaken for colour
// classes and accidentally deduplicated against `text-accent-fg`.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        {
          text: [
            // Layer 1 — size primitives (new system).
            "body",
            "ui",
            "small",

            // Layer 2 — composed semantic tokens (new system).
            "title",
            "title-reg",
            "title-medium",
            "title-semibold",
            "title-bold",
            "heading",
            "heading-reg",
            "heading-medium",
            "heading-semibold",
            "heading-bold",
            "subhead",
            "subhead-reg",
            "subhead-medium",
            "subhead-semibold",
            "subhead-bold",
            "label",
            "mono",
            "caption",
            "num",
            "table-row",
            "table-header",
            "list-row",

            // OLD tokens — kept until Task 12.
            "2xs",
            "xs",
            "xs-plus",
            "sm",
            "sm-plus",
            "base",
            "md",
            "lg",
            "xl",
            "2xl",
          ],
        },
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
