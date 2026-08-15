import type { ConfigExtension, DefaultClassGroupIds, DefaultThemeGroupIds } from "tailwind-merge";

/**
 * Teaches tailwind-merge the eight role-named font sizes, so `cn` treats them as
 * font sizes rather than colours.
 *
 * ```ts
 * import { extendTailwindMerge } from "tailwind-merge";
 * import { typeScale } from "@/lib/type-scale";
 *
 * const twMerge = extendTailwindMerge(typeScale);
 * ```
 *
 * Without it, `cn("text-body", "text-muted-foreground")` returns
 * `"text-muted-foreground"` alone. tailwind-merge matches its built-in font-size
 * group on `/^(\d+(\.\d+)?)?(xs|sm|md|lg|xl)$/` plus the literal `base`, so
 * `text-body` fails that test, falls through to the `text-color` group, and is
 * dropped as a conflicting colour.
 *
 * Radius and elevation need no equivalent: every name in both of those scales is
 * a t-shirt size, which the same regex already accepts.
 */
export const typeScale: ConfigExtension<DefaultClassGroupIds, DefaultThemeGroupIds> = {
  extend: {
    theme: {
      text: ["display", "title", "heading", "subhead", "field", "body", "ui", "label"],
    },
  },
};
