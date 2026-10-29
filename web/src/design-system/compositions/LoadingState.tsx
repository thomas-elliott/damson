import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/design-system/utils";

export type LoadingStateProps = Omit<ComponentProps<"div">, "children"> & {
  /** Defaults to "Loading…". */
  label?: ReactNode;
  /**
   * `block` fills its flex parent, centred, in place of the content, like
   * EmptyState. `inline` is a single line inside a section or list.
   */
  variant?: "block" | "inline";
};

/** A load in flight, in place of the content it will become. */
export function LoadingState({
  className,
  label = "Loading…",
  variant = "block",
  ...props
}: LoadingStateProps) {
  return (
    <div
      role="status"
      data-slot="loading-state"
      className={cn(
        "text-ui text-subtle-foreground",
        variant === "block" && "flex flex-1 items-center justify-center px-4 py-10",
        variant === "inline" && "py-2",
        className,
      )}
      {...props}
    >
      {label}
    </div>
  );
}
