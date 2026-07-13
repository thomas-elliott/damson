import { useNavigate } from "react-router";
import { AlertTriangle, CheckCircle2, Images, Loader2 } from "lucide-react";

import { Shell } from "@/components/Shell";
import {
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  EmptyState,
  QueueItem,
  Toolbar,
} from "@/design-system";
import { usePreviewQueue, useMe, type PreviewRow } from "@/hooks/useDataQueries";
import { usePreviewEvents } from "@/hooks/usePreviewEvents";

/**
 * Preview processing queue. Backed by GET /api/previews — the per-version
 * rollups still being rendered. Mirrors the import QueuePage: live updates
 * arrive over /api/previews/events via usePreviewEvents, subscribed here (not
 * in the always-mounted shell) so no EventSource hangs off every page. Requeue
 * actions live on the asset/file detail panels, so rows here just open the asset.
 */
export function PreviewQueuePage() {
  const { data: previews, finished, loading } = usePreviewQueue();
  const { data: me } = useMe();
  usePreviewEvents(me?.current_library?.id);
  const navigate = useNavigate();
  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${previews.length}`}>
              <BreadcrumbItem current>Processing queue</BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <div className="flex flex-1 items-center justify-center text-ui text-text-3">
              Loading queue…
            </div>
          ) : previews.length === 0 && finished.length === 0 ? (
            <EmptyState
              icon={<Images size={28} />}
              title="No previews processing"
              description="Backend previews generated after import show up here while they render. Regenerate one from an asset or file to see it appear."
            />
          ) : (
            <div className="flex flex-col gap-2 p-3">
              {previews.length === 0 ? (
                <div className="px-1 py-6 text-center text-ui text-text-3">
                  Nothing processing right now.
                </div>
              ) : (
                previews.map((row) => (
                  <PreviewQueueRow
                    key={row.version_id}
                    row={row}
                    onOpen={() => navigate(`/library/${row.asset_id}`)}
                  />
                ))
              )}
              {finished.length > 0 && (
                <>
                  <div className="mt-4 mb-1 px-1 text-caption uppercase tracking-wide text-text-4">
                    Finished
                  </div>
                  {finished.map((row) => (
                    <PreviewQueueRow
                      key={row.version_id}
                      row={row}
                      onOpen={() => navigate(`/library/${row.asset_id}`)}
                    />
                  ))}
                </>
              )}
            </div>
          )}
        </>
      }
    />
  );
}

function PreviewQueueRow({ row, onOpen }: { row: PreviewRow; onOpen: () => void }) {
  const processing = row.status === "processing";
  const hasFailures = row.failed > 0;
  // Denominator is the renderable set (excludes unsupported files, which are
  // never attempted), so "done/attempted" reflects real preview progress.
  const attempted = row.done + row.failed + row.processing + row.queued;

  let thumb = <CheckCircle2 size={20} className="text-text-3" />;
  if (processing) thumb = <Loader2 size={20} className="animate-spin text-text-3" />;
  else if (hasFailures) thumb = <AlertTriangle size={20} className="text-err" />;

  return (
    <QueueItem
      name={row.asset_name || `Version #${row.version_id}`}
      thumb={thumb}
      sub={
        <>
          <Badge size="sm" mono tone={processing ? "warn" : "ok"}>
            {row.status}
          </Badge>
          <span className="tabular-nums text-text-3">
            {row.done}/{attempted} previews
          </span>
          {hasFailures && <span className="text-err">{row.failed} failed</span>}
          <span className="text-text-4">updated {new Date(row.updated_at).toLocaleString()}</span>
        </>
      }
      actions={
        <Button size="sm" variant="primary" onClick={onOpen}>
          Open
        </Button>
      }
    />
  );
}
