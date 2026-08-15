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
        "flex flex-col items-center justify-center gap-3 px-6 py-14 text-center text-subtle-foreground",
        className,
      )}
      {...props}
    >
      {icon && <div className="text-faint-foreground">{icon}</div>}
      {title && <h3 className="text-subhead-medium text-muted-foreground">{title}</h3>}
      {description && <p className="max-w-sm text-ui text-subtle-foreground">{description}</p>}
      {action && <div className="pt-1">{action}</div>}
    </div>
  );
}
