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
