import { useCallback, useEffect, useState, type CSSProperties, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

import { ResizeHandle } from "./ResizeHandle";

export interface AppShellProps {
  rail: ReactNode;
  sidebar?: ReactNode;
  main: ReactNode;
  /** Override the default sidebar width (px). Defaults to `--sidebar-w`. */
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
 * overflow is clipped so only the main panel scrolls internally. Below the
 * `md` breakpoint the sidebar floats over main as a drawer instead of taking
 * a column.
 *
 * When `resizableSidebar` is set, a 4px drag handle floats over the
 * sidebar/main boundary and persists width in localStorage.
 *
 * The main column is the page's `<main id="main">` landmark, and a "Skip to
 * content" link to it is the first thing a keyboard user reaches.
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

  return (
    <div
      className={cn(
        "relative grid h-screen min-h-[640px] grid-rows-1 overflow-hidden bg-background text-foreground",
        // Below md the sidebar column closes and the sidebar floats over main
        // as a drawer, so main keeps the width a narrow screen has left.
        "grid-cols-[var(--rail-w)_var(--sidebar-col)_1fr] max-md:grid-cols-[var(--rail-w)_0px_1fr]",
        className,
      )}
      style={{ "--sidebar-col": `${sidebarColPx}px` } as CSSProperties}
    >
      {/* First stop for a keyboard user, past the rail to the page. Absolute
          even when shown, so it never takes a grid cell. */}
      <a
        href="#main"
        onClick={(e) => {
          e.preventDefault();
          document.getElementById("main")?.focus();
        }}
        className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-tooltip focus:rounded-md focus:border focus:border-border-strong focus:bg-popover focus:px-3 focus:py-1.5 focus:text-ui focus:text-foreground focus:shadow-md"
      >
        Skip to content
      </a>
      <div className="border-r border-border bg-card">{rail}</div>
      {sidebar ? (
        <div className="relative border-r border-border bg-card max-md:absolute max-md:inset-y-0 max-md:left-[var(--rail-w)] max-md:z-detail max-md:w-sidebar max-md:max-w-[calc(100%-var(--rail-w))] max-md:shadow-lg">
          {sidebar}
          {resizableSidebar && (
            <div className="absolute right-[-2px] top-0 z-resizer h-full w-1 max-md:hidden">
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
      <main
        id="main"
        tabIndex={-1}
        className="relative col-start-3 flex min-w-0 flex-col outline-none"
      >
        {main}
      </main>
    </div>
  );
}
