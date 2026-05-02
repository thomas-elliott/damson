import { useSearchParams, Link, Navigate } from "react-router";

import { AuditList } from "@/components/AuditList";
import { Shell } from "@/components/Shell";
import { Breadcrumb, BreadcrumbItem, SegmentedControl, Toolbar } from "@/design-system";
import { useAdminAudit, useMe, type AdminAuditScope } from "@/hooks/useDataQueries";

/**
 * Sysadmin audit log. Two scopes:
 *
 *   - Server (default): events with `library_id IS NULL` — library
 *     create/delete, role changes, server-settings updates.
 *   - All: server-level + every library-scoped event across all libraries.
 *     Use this to spot cross-library bursts ("library X had 200 deletions
 *     in the last hour") without joining each library individually.
 *
 * Non-sysadmins get bounced back to the library audit. The endpoint also
 * returns 403 for non-sysadmins, so this is defence-in-depth.
 */
export function AdminAuditPage() {
  const { data: me, loading: meLoading } = useMe();
  const isSysadmin = me?.role === "sysadmin";
  const [params, setParams] = useSearchParams();
  const scope: AdminAuditScope = params.get("scope") === "all" ? "all" : "server";
  const { data: entries, total, loading } = useAdminAudit({ limit: 200, scope }, isSysadmin);

  if (meLoading) {
    return (
      <Shell
        main={
          <div className="flex flex-1 items-center justify-center text-ui text-text-3">
            Loading…
          </div>
        }
      />
    );
  }
  if (!isSysadmin) {
    return <Navigate to="/audit" replace />;
  }

  const setScope = (next: AdminAuditScope) => {
    const p = new URLSearchParams(params);
    if (next === "server") p.delete("scope");
    else p.set("scope", next);
    setParams(p, { replace: true });
  };

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={loading ? "…" : `${total}`}>
              <BreadcrumbItem>
                <Link to="/audit" className="hover:underline">
                  Audit log
                </Link>
              </BreadcrumbItem>
              <BreadcrumbItem current>
                {scope === "all" ? "All libraries" : "Server"}
              </BreadcrumbItem>
            </Breadcrumb>
            <div className="ml-auto">
              <SegmentedControl
                value={scope}
                onValueChange={(v) => setScope(v as AdminAuditScope)}
                options={[
                  { value: "server", label: "Server" },
                  { value: "all", label: "All libraries" },
                ]}
                aria-label="Audit scope"
              />
            </div>
          </Toolbar>
          <div className="mx-auto w-full max-w-[960px] p-3">
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-ui text-text-3">
                Loading…
              </div>
            ) : (
              <AuditList
                entries={entries}
                showLibrary={scope === "all"}
                emptyTitle={scope === "all" ? "No activity yet" : "No server-level events yet"}
                emptyDescription={
                  scope === "all"
                    ? "Library and server-level activity will appear here."
                    : "Library creation, role changes, and server-settings updates appear here."
                }
              />
            )}
          </div>
        </>
      }
    />
  );
}
