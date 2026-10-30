import { useCallback, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Inbox,
  Loader2,
  Package,
  RotateCw,
} from "lucide-react";

import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";
import { FindListingButton } from "@/components/detail/MetadataActions";
import { useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";
import { Shell } from "@/components/Shell";
import {
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  cn,
  EmptyState,
  ErrorState,
  LoadingState,
  PageContent,
  QueueItem,
  Toolbar,
} from "@/design-system";
import { useImports, useMe, type ImportRow } from "@/hooks/useDataQueries";
import { useImportEvents } from "@/hooks/useImportEvents";
import { useRetryImport, useReviewAsset } from "@/hooks/useMutations";

const IN_FLIGHT_STAGES = ["uploaded", "extracting", "detecting", "thumbnailing"];

// plainSnippet flattens storefront markdown to prose for the clamped card line.
function plainSnippet(md: string): string {
  return md
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[*_`#>]/g, "")
    .replace(/\\([-.])/g, "$1")
    .replace(/\s+/g, " ")
    .trim();
}

/**
 * Import queue. Backed by GET /api/imports: the active section is the review
 * backlog — anything in flight, failed, or completed but not yet reviewed —
 * and a retained "finished" section shows recently-reviewed imports (7 days /
 * 50 rows). Review happens in place: the card carries the thumbnail and
 * description, opens the detail panel beside the list, and offers store-match
 * and mark-reviewed actions without leaving the queue.
 */
export function QueuePage() {
  const navigate = useNavigate();
  const { data: imports, finished, loading, failed, reload } = useImports("in_flight");
  const { data: me } = useMe();
  useImportEvents(me?.current_library?.id);
  const [search, setSearch] = useSearchParams();

  const [reserved, setReserved] = useState(0);
  const onReservedWidthChange = useCallback((w: number) => setReserved(w), []);

  const reviewing = search.get("asset") ? Number(search.get("asset")) : null;
  const setReviewing = (id: number | null) => {
    const next = new URLSearchParams(search);
    if (id == null) next.delete("asset");
    else next.set("asset", String(id));
    setSearch(next, { replace: true });
  };

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${imports.length}`}>
              <BreadcrumbItem current heading>
                Import queue
              </BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <LoadingState />
          ) : failed ? (
            <ErrorState title="Couldn't load the import queue" onRetry={reload} />
          ) : imports.length === 0 && finished.length === 0 ? (
            <EmptyState
              icon={<Inbox size={28} />}
              title="Nothing waiting for review"
              description="Everything that arrived via import has been checked. New unreviewed assets will show up here automatically."
            />
          ) : (
            <PageContent
              style={{ paddingRight: reserved ? reserved + 14 : undefined }}
              className="gap-2"
            >
              {imports.length === 0 ? (
                <div className="px-1 py-6 text-center text-ui text-subtle-foreground">
                  Nothing waiting for review.
                </div>
              ) : (
                imports.map((imp) => (
                  <ImportQueueRow
                    key={imp.id}
                    imp={imp}
                    active={reviewing === imp.asset_id}
                    onReview={() => setReviewing(imp.asset_id)}
                  />
                ))
              )}
              {finished.length > 0 && (
                <>
                  <div className="mt-4 mb-1 px-1 text-caption uppercase tracking-wide text-faint-foreground">
                    Finished
                  </div>
                  {finished.map((imp) => (
                    <FinishedImportRow
                      key={imp.id}
                      imp={imp}
                      active={reviewing === imp.asset_id}
                      onOpen={() => setReviewing(imp.asset_id)}
                    />
                  ))}
                </>
              )}
            </PageContent>
          )}
          <AssetDetailPanel
            assetId={reviewing}
            onClose={() => setReviewing(null)}
            onOpenPackage={(id) => navigate(`/library/${id}/package`)}
            onReservedWidthChange={onReservedWidthChange}
          />
        </>
      }
    />
  );
}

// ImportThumb shows the asset's thumbnail once it has one, else the stage:
// spinner in flight, warning on failure, placeholder when done without an image.
function ImportThumb({ imp }: { imp: ImportRow }) {
  const url = useBlobUrl(
    imp.thumbnail_object_key ? `/api/thumbs/${imp.thumbnail_object_key}` : undefined,
  );
  if (url) return <img src={url} alt="" className="h-full w-full object-cover" />;
  if (imp.stage === "failed") return <AlertTriangle size={20} className="text-destructive" />;
  if (IN_FLIGHT_STAGES.includes(imp.stage))
    return <Loader2 size={20} className="animate-spin text-subtle-foreground" />;
  return <Package size={20} className="text-faint-foreground" />;
}

function activeRowClass(active: boolean): string {
  return active ? "border-primary ring-1 ring-ring" : "";
}

function FinishedImportRow({
  imp,
  active,
  onOpen,
}: {
  imp: ImportRow;
  active: boolean;
  onOpen: () => void;
}) {
  return (
    <QueueItem
      className={activeRowClass(active)}
      name={imp.asset_name || `Import #${imp.id}`}
      thumb={
        imp.thumbnail_object_key ? (
          <ImportThumb imp={imp} />
        ) : (
          <CheckCircle2 size={20} className="text-subtle-foreground" />
        )
      }
      sub={
        <>
          <Badge size="sm" mono tone="ok">
            reviewed
          </Badge>
          <span className="text-faint-foreground">
            updated {new Date(imp.updated_at).toLocaleString()}
          </span>
        </>
      }
      actions={
        <Button size="sm" variant="secondary" onClick={onOpen}>
          Open
        </Button>
      }
    />
  );
}

function ImportQueueRow({
  imp,
  active,
  onReview,
}: {
  imp: ImportRow;
  active: boolean;
  onReview: () => void;
}) {
  const retry = useRetryImport();
  const review = useReviewAsset();
  const isFailed = imp.stage === "failed";
  const description = imp.description ? plainSnippet(imp.description) : "";

  return (
    <QueueItem
      className={cn("cursor-pointer", activeRowClass(active))}
      onClick={onReview}
      thumbSize={80}
      name={imp.asset_name || `Import #${imp.id}`}
      thumb={<ImportThumb imp={imp} />}
      sub={
        <>
          <Badge size="sm" mono tone={isFailed ? "err" : imp.needs_review ? "warn" : "ok"}>
            {imp.stage}
          </Badge>
          {imp.has_metadata && (
            <Badge size="sm" mono tone="ok">
              matched
            </Badge>
          )}
          {isFailed && imp.error && <span className="truncate text-destructive">{imp.error}</span>}
          <span className="text-faint-foreground">
            updated {new Date(imp.updated_at).toLocaleString()}
          </span>
        </>
      }
      detail={
        description ? (
          <p className="line-clamp-3 text-label text-subtle-foreground">{description}</p>
        ) : (
          <p className="text-label text-faint-foreground">
            No description yet — try a store match.
          </p>
        )
      }
      actions={
        // Peer actions, so one weight in one row; primary on every row of a
        // list would mean nothing.
        <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
          <Button size="sm" variant="secondary" onClick={onReview}>
            Review
          </Button>
          <FindListingButton
            assetId={imp.asset_id}
            assetName={imp.asset_name}
            label={imp.has_metadata ? "Re-match" : "Find listing"}
          />
          {imp.needs_review && (
            <Button
              size="sm"
              variant="secondary"
              leadingIcon={<Check size={12} />}
              disabled={review.isPending}
              onClick={() => review.mutate(imp.asset_id)}
            >
              Mark reviewed
            </Button>
          )}
          {isFailed && (
            <Button
              size="sm"
              variant="secondary"
              leadingIcon={<RotateCw size={12} />}
              disabled={retry.isPending}
              onClick={() => retry.mutate(imp.id)}
            >
              Retry
            </Button>
          )}
        </div>
      }
    />
  );
}
