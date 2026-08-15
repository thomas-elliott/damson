import { clsx, type ClassValue } from "clsx";
import {
  type ConfigExtension,
  type DefaultClassGroupIds,
  type DefaultThemeGroupIds,
  extendTailwindMerge,
  mergeConfigs,
} from "tailwind-merge";

import { typeScale } from "@/lib/type-scale";

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

const twMerge = extendTailwindMerge(typeScale, (config) => mergeConfigs(config, damsonRoles));

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
