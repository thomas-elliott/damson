import { type HTMLAttributes } from "react";

import { cn } from "@/design-system/utils";

export function Kbd({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return (
    <kbd
      className={cn(
        "inline-flex h-[20px] items-center rounded-xs border border-border-strong bg-muted px-1 text-mono text-subtle-foreground",
        className,
      )}
      {...props}
    />
  );
}
