import { Link } from "react-router";

import { ActivityRow, Badge, EmptyState } from "@/design-system";
import {
  formatAuditAction,
  formatAuditActor,
  formatAuditTarget,
  formatAuditTimestamp,
} from "@/lib/auditFormat";
import { type AuditEntry } from "@/hooks/useDataQueries";

/**
 * Reusable audit list. Used by the per-asset/per-project sections in the
 * detail panel and by the dedicated `/audit` and `/admin/audit` pages.
 *
 * `linkTargets` controls whether entries that point at an asset/project/etc.
 * render their target as a router link. The detail-panel sections turn it
 * off (the user is already looking at the target) — the standalone pages
 * turn it on so a click jumps to the relevant detail view.
 *
 * `showLibrary` prepends the source library name to each target. Only the
 * sysadmin scope=all feed sets this — the per-library page already filters
 * by library, so the chip would be redundant noise there.
 */
export function AuditList({
  entries,
  emptyTitle = "No activity yet",
  emptyDescription = "Audit entries appear here as soon as someone makes a change.",
  linkTargets = false,
  showLibrary = false,
}: {
  entries: AuditEntry[];
  emptyTitle?: string;
  emptyDescription?: string;
  linkTargets?: boolean;
  showLibrary?: boolean;
}) {
  if (entries.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }
  return (
    <div className="rounded-md border border-border bg-card">
      {entries.map((e) => (
        <ActivityRow
          key={e.id}
          timestamp={formatAuditTimestamp(e.created_at)}
          actor={formatAuditActor(e.user_display_name, e.user_email)}
          action={formatAuditAction(e.action_type)}
          target={
            <>
              {showLibrary && e.library_name && (
                <Badge size="sm" tone="unk" className="mr-1.5 align-middle">
                  {e.library_name}
                </Badge>
              )}
              {renderTarget(e, linkTargets)}
            </>
          }
        />
      ))}
    </div>
  );
}

function renderTarget(e: AuditEntry, linkTargets: boolean) {
  const named = formatAuditTarget(e.target_type, e.target_id, e.target_name);
  // A named target keeps its id beside it, muted, to tell same-named rows apart.
  const label =
    e.target_name && e.target_id ? (
      <>
        {named} <span className="font-normal text-subtle-foreground">#{e.target_id}</span>
      </>
    ) : (
      named
    );
  if (!linkTargets || !e.target_id) return label;
  switch (e.target_type) {
    case "asset":
      return (
        <Link to={`/library/${e.target_id}`} className="hover:underline">
          {label}
        </Link>
      );
    case "project":
      return (
        <Link to={`/projects/${e.target_id}`} className="hover:underline">
          {label}
        </Link>
      );
    case "collection":
      return (
        <Link to={`/collections/${e.target_id}`} className="hover:underline">
          {label}
        </Link>
      );
    default:
      return label;
  }
}
