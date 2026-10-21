import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/design-system/utils";

export type EmptyStateProps = Omit<ComponentProps<"div">, "title"> & {
  title?: ReactNode;
  description?: ReactNode;
  /** Icon element, sized by the caller. Rendered above the title, muted. */
  icon?: ReactNode;
  /** Primary affordance — a Button, a link, or a small group of them. */
  action?: ReactNode;
};

export function EmptyState({
  className,
  title,
  description,
  icon,
  action,
  ...props
}: EmptyStateProps) {
  return (
    <div
      role="status"
      data-slot="empty-state"
      className={cn(
        "flex flex-col items-center justify-center gap-2 px-4 py-10 text-center",
        className,
      )}
      {...props}
    >
      {icon && (
        <div data-slot="empty-state-icon" className="mb-1 text-faint-foreground [&_svg]:size-8">
          {icon}
        </div>
      )}
      <p data-slot="empty-state-title" className="text-subhead font-medium text-foreground">
        {title}
      </p>
      {description && (
        <p
          data-slot="empty-state-description"
          className="max-w-[42ch] text-body text-muted-foreground"
        >
          {description}
        </p>
      )}
      {action && (
        <div data-slot="empty-state-action" className="mt-2 flex items-center gap-2">
          {action}
        </div>
      )}
    </div>
  );
}
