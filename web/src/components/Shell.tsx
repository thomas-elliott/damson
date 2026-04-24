import { type ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import {
  Boxes,
  FileSearch,
  FolderGit2,
  Inbox,
  Library,
  Settings as SettingsIcon,
} from "lucide-react";

import {
  AppShell,
  NavRail,
  NavRailItem,
  NavRailSpacer,
} from "@/design-system";
import { useMe } from "@/hooks/useDataQueries";

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
  return (
    <NavRail brand={<DamsonBrand />}>
      <RailLink to="/library" icon={<Library size={16} />} label="Library" active={pathname.startsWith("/library")} />
      <RailLink to="/queue" icon={<Inbox size={16} />} label="Import queue" active={pathname.startsWith("/queue")} />
      <RailLink to="/projects" icon={<FolderGit2 size={16} />} label="Projects" active={pathname.startsWith("/projects")} />
      <RailLink to="/collections" icon={<Boxes size={16} />} label="Collections" active={pathname.startsWith("/collections")} />
      <RailLink to="/audit" icon={<FileSearch size={16} />} label="Audit log (M7)" disabled />
      <NavRailSpacer />
      <RailLink to="/settings" icon={<SettingsIcon size={16} />} label="Settings" active={pathname.startsWith("/settings")} />
    </NavRail>
  );
}

function RailLink({
  to,
  icon,
  label,
  active,
  disabled,
}: {
  to: string;
  icon: ReactNode;
  label: string;
  active?: boolean;
  disabled?: boolean;
}) {
  if (disabled) {
    return <NavRailItem icon={icon} label={label} disabled />;
  }
  return (
    <NavLink to={to} aria-label={label} className="contents">
      {({ isActive }) => (
        <NavRailItem icon={icon} label={label} active={active ?? isActive} />
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

