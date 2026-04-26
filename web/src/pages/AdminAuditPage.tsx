import { Link, Navigate } from "react-router";

import { AuditList } from "@/components/AuditList";
import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  Toolbar,
} from "@/design-system";
import { useAdminAudit, useMe } from "@/hooks/useDataQueries";

/**
 * Server-level audit log. Sysadmin-only — non-sysadmin users get bounced
 * back to the library audit. The endpoint also returns 403 for
 * non-sysadmins, so this is defence-in-depth, not the only gate.
 */
export function AdminAuditPage() {
  const { data: me, loading: meLoading } = useMe();
  const isSysadmin = me?.role === "sysadmin";
  const { data: entries, total, loading } = useAdminAudit({ limit: 200 }, isSysadmin);

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

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={loading ? "…" : `${total}`}>
              <BreadcrumbItem>
                <Link to="/audit" className="hover:underline">Audit log</Link>
              </BreadcrumbItem>
              <BreadcrumbItem current>Server</BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          <div className="mx-auto w-full max-w-[960px] p-3">
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-ui text-text-3">
                Loading…
              </div>
            ) : (
              <AuditList
                entries={entries}
                emptyTitle="No server-level events yet"
                emptyDescription="Library creation, role changes, and server-settings updates appear here."
              />
            )}
          </div>
        </>
      }
    />
  );
}
