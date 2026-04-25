import { type ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import {
  Boxes,
  FileSearch,
  FolderGit2,
  Inbox,
  Library,
  Moon,
  Settings as SettingsIcon,
  Sun,
} from "lucide-react";

import {
  AppShell,
  NavRail,
  NavRailItem,
  NavRailSpacer,
} from "@/design-system";
import { useImports, useMe } from "@/hooks/useDataQueries";
import { useTheme } from "@/hooks/useTheme";

/**
 * Damson's top-level layout. Wraps every authenticated page with the nav
 * rail + optional sidebar. Pages pass `main` and (optionally) `sidebar`;
 * the rail is owned by the shell so library switching and signed-in state
 * stays in one place.
 */
export function Shell({
  main,
  sidebar,
}: {
  main: ReactNode;
  sidebar?: ReactNode;
}) {
  const { data: me, loading } = useMe();
  if (loading) return <LoadingShell />;
  if (!me) return null;

  return (
    <AppShell
      rail={<DamsonRail />}
      sidebar={sidebar}
      main={main}
      resizableSidebar={!!sidebar}
    />
  );
}

function LoadingShell() {
  return (
    <div className="grid h-screen place-items-center bg-bg text-sm-plus text-text-3">
      Loading…
    </div>
  );
}

function DamsonRail() {
  const { pathname } = useLocation();
  const { mode, toggle } = useTheme();
  const { total: queueCount } = useImports("in_flight");
  return (
    <NavRail brand={<DamsonBrand />}>
      <RailLink to="/library" icon={<Library size={16} />} label="Library" active={pathname.startsWith("/library")} />
      <RailLink
        to="/queue"
        icon={<Inbox size={16} />}
        label="Import queue"
        active={pathname.startsWith("/queue")}
        badge={queueCount > 0 ? queueCount : undefined}
      />
      <RailLink to="/projects" icon={<FolderGit2 size={16} />} label="Projects" active={pathname.startsWith("/projects")} />
      <RailLink to="/collections" icon={<Boxes size={16} />} label="Collections" active={pathname.startsWith("/collections")} />
      <RailLink to="/audit" icon={<FileSearch size={16} />} label="Audit log" active={pathname.startsWith("/audit")} />
      <NavRailSpacer />
      <NavRailItem
        icon={mode === "dark" ? <Sun size={16} /> : <Moon size={16} />}
        label={mode === "dark" ? "Switch to light theme" : "Switch to dark theme"}
        onSelect={toggle}
      />
      <RailLink to="/settings" icon={<SettingsIcon size={16} />} label="Settings" active={pathname.startsWith("/settings")} />
    </NavRail>
  );
}

function RailLink({
  to,
  icon,
  label,
  active,
  badge,
}: {
  to: string;
  icon: ReactNode;
  label: string;
  active?: boolean;
  badge?: ReactNode;
}) {
  return (
    <NavLink to={to} aria-label={label} className="contents">
      {({ isActive }) => (
        <NavRailItem icon={icon} label={label} active={active ?? isActive} badge={badge} />
      )}
    </NavLink>
  );
}

function DamsonBrand() {
  return (
    <div
      aria-label="Damson"
      className="grid h-[30px] w-[30px] place-items-center rounded-md bg-gradient-to-br from-accent to-accent-2 font-mono text-sm-plus font-bold text-accent-fg"
    >
      D
    </div>
  );
}

