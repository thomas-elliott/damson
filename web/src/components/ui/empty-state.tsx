import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/design-system/utils/index";

// The spike's OWNED primitive: shadcn's registry has no equivalent, so this is
// the case that proves your own components ride the same rails as upstream's.
// If only upstream items can be distributed, the registry is a convenience;
// if this one installs the same way, it is the distribution mechanism.
//
// Contract lifted from mangapack's `empty-state.svelte` (title, optional
// description, actions slot) — the cleanest of the existing four and the one
// derived from taskmanager, so it carries both Svelte projects' thinking.
// Changes in the port: `icon` is a slot rather than absent, and `action` is a
// named prop instead of a default children slot, so the component does not
// depend on Button.

export type EmptyStateProps = Omit<ComponentProps<"div">, "title"> & {
  title: ReactNode;
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
      <p data-slot="empty-state-title" className="text-base font-medium text-foreground">
        {title}
      </p>
      {description && (
        <p
          data-slot="empty-state-description"
          className="max-w-[42ch] text-sm text-muted-foreground"
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
