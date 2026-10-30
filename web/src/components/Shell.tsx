import { type ReactNode } from "react";
import { NavLink, useLocation } from "react-router";
import {
  Bell,
  Boxes,
  FileSearch,
  FolderGit2,
  Images,
  Inbox,
  Library,
  Settings as SettingsIcon,
  Tags,
} from "lucide-react";

import {
  AppShell,
  BrandMark,
  LoadingState,
  NavRail,
  NavRailItem,
  NavRailSpacer,
} from "@/design-system";
import { useNotifications } from "@/components/notifications/NotificationProvider";
import { UserMenu } from "@/components/UserMenu";
import { useImports, useMe, usePreviewQueue } from "@/hooks/useDataQueries";

/**
 * Damson's top-level layout. Wraps every authenticated page with the nav
 * rail + optional sidebar. Pages pass `main` and (optionally) `sidebar`;
 * the rail is owned by the shell so library switching and signed-in state
 * stays in one place.
 */
export function Shell({ main, sidebar }: { main: ReactNode; sidebar?: ReactNode }) {
  const { data: me, loading } = useMe();
  if (loading) return <LoadingShell />;
  if (!me) return null;

  return (
    <AppShell
      rail={<DamsonRail displayName={me.display_name} email={me.email} />}
      sidebar={sidebar}
      main={main}
      resizableSidebar={!!sidebar}
    />
  );
}

// The same full-screen state as AuthGuard's, so a cold load reads as one
// state rather than two in sequence.
function LoadingShell() {
  return <LoadingState className="h-screen bg-background" />;
}

function DamsonRail({ displayName, email }: { displayName: string; email: string }) {
  const { pathname } = useLocation();
  const { total: queueCount } = useImports("in_flight");
  // Badge count comes from the query (refreshed on its staleTime); the live SSE
  // stream is subscribed on the Processing queue page itself, mirroring how the
  // import queue works — so no always-open EventSource hangs off every page.
  const { total: processingCount } = usePreviewQueue();
  const { unreadCount, inFlightCount, toggleOpen } = useNotifications();
  const notifBadge = unreadCount + inFlightCount;
  return (
    <NavRail brand={<BrandMark />}>
      <RailLink
        to="/library"
        icon={<Library size={16} />}
        label="Library"
        active={pathname.startsWith("/library")}
      />
      <RailLink
        to="/tags"
        icon={<Tags size={16} />}
        label="Tags"
        active={pathname.startsWith("/tags")}
      />
      <RailLink
        to="/queue"
        icon={<Inbox size={16} />}
        label="Import queue"
        active={pathname.startsWith("/queue")}
        badge={queueCount > 0 ? queueCount : undefined}
      />
      <RailLink
        to="/processing"
        icon={<Images size={16} />}
        label="Processing queue"
        active={pathname.startsWith("/processing")}
        badge={processingCount > 0 ? processingCount : undefined}
      />
      <RailLink
        to="/projects"
        icon={<FolderGit2 size={16} />}
        label="Projects"
        active={pathname.startsWith("/projects")}
      />
      <RailLink
        to="/collections"
        icon={<Boxes size={16} />}
        label="Collections"
        active={pathname.startsWith("/collections")}
      />
      <RailLink
        to="/audit"
        icon={<FileSearch size={16} />}
        label="Audit log"
        active={pathname.startsWith("/audit")}
      />
      <NavRailSpacer />
      <NavRailItem
        icon={<Bell size={16} />}
        label="Notifications"
        badge={notifBadge > 0 ? notifBadge : undefined}
        onSelect={toggleOpen}
      />
      <RailLink
        to="/settings"
        icon={<SettingsIcon size={16} />}
        label="Settings"
        active={pathname.startsWith("/settings")}
      />
      <UserMenu displayName={displayName} email={email} />
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
