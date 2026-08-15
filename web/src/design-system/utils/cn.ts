import { clsx, type ClassValue } from "clsx";
import {
  type ConfigExtension,
  type DefaultClassGroupIds,
  type DefaultThemeGroupIds,
  extendTailwindMerge,
  mergeConfigs,
} from "tailwind-merge";

import { typeScale } from "@/lib/type-scale";

// The contract's 8 role sizes come from `typeScale`; these are damson's own
// composed roles on top of it. Neither set looks like a t-shirt size, so without
// both registered tailwind-merge reads them as colours — `cn("text-mono",
// "text-muted-foreground")` would silently drop the size.
const damsonRoles: ConfigExtension<DefaultClassGroupIds, DefaultThemeGroupIds> = {
  extend: {
    classGroups: {
      "font-size": [
        {
          text: [
            "overline",
            "title-reg",
            "title-medium",
            "title-semibold",
            "title-bold",
            "heading-reg",
            "heading-medium",
            "heading-semibold",
            "heading-bold",
            "subhead-reg",
            "subhead-medium",
            "subhead-semibold",
            "subhead-bold",
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
};

// Composed through mergeConfigs, not spread: extendTailwindMerge takes only one
// ConfigExtension, and a spread would drop any key `typeScale` grows later.
const twMerge = extendTailwindMerge(typeScale, (config) => mergeConfigs(config, damsonRoles));

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
