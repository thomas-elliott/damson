import { type HTMLAttributes } from "react";

import { cn } from "@/design-system/utils";

export function Kbd({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return (
    <kbd
      className={cn(
        "inline-flex h-[18px] items-center rounded-[3px] border border-line-2 bg-surface px-1 font-mono text-2xs text-text-3",
        className,
      )}
      {...props}
    />
  );
}
