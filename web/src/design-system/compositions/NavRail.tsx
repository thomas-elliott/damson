import { forwardRef, type ComponentPropsWithoutRef, type ReactNode } from "react";

import { cn } from "@/design-system/utils";
import { SimpleTooltip } from "@/design-system/primitives/Tooltip";

export interface NavRailProps extends ComponentPropsWithoutRef<"nav"> {
  brand?: ReactNode;
  children: ReactNode;
}

export const NavRail = forwardRef<HTMLElement, NavRailProps>(function NavRail(
  { className, brand, children, ...props },
  ref,
) {
  return (
    <nav
      ref={ref}
      className={cn(
        "flex h-full w-rail flex-col items-center gap-1 py-2",
        className,
      )}
      aria-label="Primary"
      {...props}
    >
      {brand && <div className="mb-2 grid h-[30px] w-[30px] place-items-center">{brand}</div>}
      {children}
    </nav>
  );
});

export interface NavRailItemProps extends ComponentPropsWithoutRef<"button"> {
  label: string;
  icon: ReactNode;
  active?: boolean;
  badge?: ReactNode;
  /** Called when the item is activated. */
  onSelect?: () => void;
}

/**
 * One rail destination: 34×34 icon button with tooltip label on hover, optional
 * count badge, and a left accent bar when active.
 */
export const NavRailItem = forwardRef<HTMLButtonElement, NavRailItemProps>(
  function NavRailItem(
    { className, label, icon, active, badge, onSelect, onClick, ...props },
    ref,
  ) {
    return (
      <SimpleTooltip label={label} side="right">
        <button
          ref={ref}
          type="button"
          aria-label={label}
          aria-current={active ? "page" : undefined}
          onClick={(e) => {
            onSelect?.();
            onClick?.(e);
          }}
          className={cn(
            "relative grid h-[34px] w-[34px] place-items-center rounded-sm text-text-3 transition-colors duration-std ease",
            "hover:bg-surface hover:text-text",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
            "aria-[current=page]:bg-surface aria-[current=page]:text-text",
            className,
          )}
          {...props}
        >
          {active && (
            <span
              aria-hidden
              className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-r-sm bg-accent"
            />
          )}
          {icon}
          {badge !== undefined && (
            <span className="absolute -right-0.5 -top-0.5 inline-flex h-[15px] min-w-[15px] items-center justify-center rounded-pill bg-accent px-1 font-mono text-[9px] font-semibold text-accent-fg">
              {badge}
            </span>
          )}
        </button>
      </SimpleTooltip>
    );
  },
);

export function NavRailSpacer() {
  return <div className="flex-1" aria-hidden />;
}
