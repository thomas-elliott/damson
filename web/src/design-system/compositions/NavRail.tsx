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
      className={cn("flex h-full w-rail flex-col items-center gap-1 py-2", className)}
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
 * One rail destination: 34×34 icon button with tooltip label on hover and an
 * optional count badge. The active item takes a soft brand tint.
 */
export const NavRailItem = forwardRef<HTMLButtonElement, NavRailItemProps>(function NavRailItem(
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
          "relative grid h-[34px] w-[34px] place-items-center rounded-md text-subtle-foreground transition-colors duration-fast ease-standard",
          "hover:bg-control hover:text-foreground",
          "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
          "aria-[current=page]:bg-primary-soft aria-[current=page]:text-foreground",
          className,
        )}
        {...props}
      >
        {icon}
        {badge !== undefined && (
          <span className="absolute -right-0.5 -top-0.5 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-info px-1 text-mono font-semibold text-info-foreground">
            {badge}
          </span>
        )}
      </button>
    </SimpleTooltip>
  );
});

export function NavRailSpacer() {
  return <div className="flex-1" aria-hidden />;
}
