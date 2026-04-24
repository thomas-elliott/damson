import { type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface AppShellProps {
  rail: ReactNode;
  sidebar?: ReactNode;
  main: ReactNode;
  /** Override the default sidebar width (px). Defaults to `--ds-sidebar-w`. */
  sidebarWidth?: number;
  className?: string;
}

/**
 * Three-column app frame: rail (52px) | sidebar (resizable, default 248px) | main (fills).
 * Heights track the viewport; horizontal overflow is clipped so only the main
 * panel scrolls internally.
 */
export function AppShell({
  rail,
  sidebar,
  main,
  sidebarWidth,
  className,
}: AppShellProps) {
  const sidebarCol = sidebar
    ? `${sidebarWidth ?? "var(--ds-sidebar-w)"}${typeof sidebarWidth === "number" ? "px" : ""}`
    : "0px";

  return (
    <div
      className={cn(
        "grid h-screen min-h-[640px] overflow-hidden bg-bg text-text",
        className,
      )}
      style={{
        gridTemplateColumns: `var(--ds-rail-w) ${sidebarCol} 1fr`,
      }}
    >
      <div className="border-r border-line bg-panel">{rail}</div>
      {sidebar ? (
        <div className="border-r border-line bg-panel">{sidebar}</div>
      ) : (
        <div />
      )}
      <div className="relative flex min-w-0 flex-col">{main}</div>
    </div>
  );
}
