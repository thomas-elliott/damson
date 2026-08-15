import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// The design system uses a two-layer font-size system: 3 size primitives
// (body/ui/small) and 22 composed semantic tokens (text-title, text-mono,
// text-caption, etc.) that are not in tailwind-merge's default groups.
// Teach tailwind-merge about them so e.g. `text-mono` isn't mistaken for a
// colour and accidentally deduplicated against `text-primary-foreground`.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        {
          text: [
            "body",
            "ui",
            "small",
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
          ],
        },
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
