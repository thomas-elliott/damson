import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface EmptyStateProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  icon?: ReactNode;
  title?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
}

/**
 * Centered "nothing here" state — icon + heading + body + optional action.
 * Use inside grids, tables, sidebars for empty results.
 */
export function EmptyState({
  className,
  icon,
  title,
  description,
  action,
  ...props
}: EmptyStateProps) {
  return (
    <div
      role="status"
      className={cn(
        "flex flex-col items-center justify-center gap-3 px-6 py-14 text-center text-text-3",
        className,
      )}
      {...props}
    >
      {icon && <div className="text-text-4">{icon}</div>}
      {title && (
        <h3 className="text-md font-medium text-text-2">{title}</h3>
      )}
      {description && (
        <p className="max-w-sm text-sm-plus text-text-3">{description}</p>
      )}
      {action && <div className="pt-1">{action}</div>}
    </div>
  );
}
