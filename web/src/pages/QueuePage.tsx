import { useNavigate } from "react-router-dom";
import { AlertTriangle, Inbox, Loader2, RotateCw } from "lucide-react";

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
import { useImports, type ImportRow } from "@/hooks/useDataQueries";
import { useRetryImport } from "@/hooks/useMutations";

/**
 * Import queue v2. Backed by GET /api/imports rather than the asset list,
 * so we surface in-flight + failed imports (the asset filter only shows
 * `needs_review` rows). Failed rows expose a Retry button; the rest open
 * the asset detail panel for review.
 */
export function QueuePage() {
  const { data: imports, loading } = useImports("in_flight");
  const navigate = useNavigate();
  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${imports.length}`}>
              <BreadcrumbItem current>Import queue</BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <div className="flex flex-1 items-center justify-center text-sm-plus text-text-3">
              Loading queue…
            </div>
          ) : imports.length === 0 ? (
            <EmptyState
              icon={<Inbox size={28} />}
              title="Nothing waiting for review"
              description="Everything that arrived via import has been checked. New unreviewed assets will show up here automatically."
            />
          ) : (
            <div className="flex flex-col gap-2 p-3">
              {imports.map((imp) => (
                <ImportQueueRow
                  key={imp.id}
                  imp={imp}
                  onOpen={() => navigate(`/library/${imp.asset_id}`)}
                />
              ))}
            </div>
          )}
        </>
      }
    />
  );
}

function ImportQueueRow({
  imp,
  onOpen,
}: {
  imp: ImportRow;
  onOpen: () => void;
}) {
  const retry = useRetryImport();
  const stage = imp.stage;
  const isFailed = stage === "failed";

  return (
    <QueueItem
      name={imp.asset_name || `Import #${imp.id}`}
      thumb={
        isFailed ? <AlertTriangle size={20} className="text-err" /> : <Loader2 size={20} className="text-text-3" />
      }
      sub={
        <>
          <Badge size="sm" mono tone={isFailed ? "err" : imp.needs_review ? "warn" : "ok"}>
            {stage}
          </Badge>
          {isFailed && imp.error && (
            <span className="truncate text-err">{imp.error}</span>
          )}
          <span className="text-text-4">
            updated {new Date(imp.updated_at).toLocaleString()}
          </span>
        </>
      }
      actions={
        <>
          {isFailed && (
            <Button
              size="sm"
              variant="ghost"
              leadingIcon={<RotateCw size={12} />}
              disabled={retry.isPending}
              onClick={() => retry.mutate(imp.id)}
            >
              Retry
            </Button>
          )}
          <Button size="sm" variant="primary" onClick={onOpen}>
            Review
          </Button>
        </>
      }
    />
  );
}
