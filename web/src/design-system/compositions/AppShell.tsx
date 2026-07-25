import { useCallback, useEffect, useState, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

import { ResizeHandle } from "./ResizeHandle";

export interface AppShellProps {
  rail: ReactNode;
  sidebar?: ReactNode;
  main: ReactNode;
  /** Override the default sidebar width (px). Defaults to `--ds-sidebar-w`. */
  sidebarWidth?: number;
  /** Allow the sidebar to be resized with a drag handle. Width is
   * persisted to localStorage under `damson.sidebar.w`. */
  resizableSidebar?: boolean;
  /** Clamp bounds for the resizable sidebar. */
  sidebarMin?: number;
  sidebarMax?: number;
  className?: string;
}

const SIDEBAR_W_KEY = "damson.sidebar.w";
const DEFAULT_SIDEBAR_W = 248;

/**
 * Three-column app frame: rail (52px) | sidebar (optionally resizable,
 * default 248px) | main (fills). Heights track the viewport; horizontal
 * overflow is clipped so only the main panel scrolls internally.
 *
 * When `resizableSidebar` is set, a 4px drag handle floats over the
 * sidebar/main boundary and persists width in localStorage.
 */
export function AppShell({
  rail,
  sidebar,
  main,
  sidebarWidth,
  resizableSidebar,
  sidebarMin = 200,
  sidebarMax = 480,
  className,
}: AppShellProps) {
  // The stored width is read whether or not a sidebar is mounted, so a page
  // that starts collapsed restores the user's width on open instead of
  // overwriting it with the default.
  const [stateW, setStateW] = useState<number>(() => {
    if (sidebarWidth !== undefined) return sidebarWidth;
    if (typeof window === "undefined") return DEFAULT_SIDEBAR_W;
    const stored = window.localStorage.getItem(SIDEBAR_W_KEY);
    const parsed = stored ? Number(stored) : NaN;
    if (Number.isFinite(parsed) && parsed >= sidebarMin && parsed <= sidebarMax) {
      return parsed;
    }
    return DEFAULT_SIDEBAR_W;
  });

  useEffect(() => {
    if (!resizableSidebar) return;
    window.localStorage.setItem(SIDEBAR_W_KEY, String(stateW));
  }, [resizableSidebar, stateW]);

  const onResize = useCallback((next: number) => setStateW(next), []);

  const sidebarColPx = sidebar ? stateW : 0;
  const gridTemplate = sidebar
    ? `var(--ds-rail-w) ${sidebarColPx}px 1fr`
    : `var(--ds-rail-w) 0px 1fr`;

  return (
    <div
      className={cn(
        "grid h-screen min-h-[640px] grid-rows-1 overflow-hidden bg-bg text-text",
        className,
      )}
      style={{ gridTemplateColumns: gridTemplate }}
    >
      <div className="border-r border-line bg-panel">{rail}</div>
      {sidebar ? (
        <div className="relative border-r border-line bg-panel">
          {sidebar}
          {resizableSidebar && (
            <div className="absolute right-[-2px] top-0 z-resizer h-full w-1">
              <ResizeHandle
                value={stateW}
                onChange={onResize}
                min={sidebarMin}
                max={sidebarMax}
                aria-label="Resize sidebar"
              />
            </div>
          )}
        </div>
      ) : (
        <div />
      )}
      <div className="relative flex min-w-0 flex-col">{main}</div>
    </div>
  );
}
