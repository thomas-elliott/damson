import { useCallback, useState, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Inbox,
  Loader2,
  Package,
  RotateCw,
  X,
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
  Pager,
  QueueItem,
  Toolbar,
} from "@/design-system";
import {
  TRIAGE_PAGE,
  useMe,
  usePreviewQueue,
  useTriage,
  type PreviewRow,
  type TriageRow,
} from "@/hooks/useDataQueries";
import { useImportEvents } from "@/hooks/useImportEvents";
import { usePreviewEvents } from "@/hooks/usePreviewEvents";
import { useDismissImport, useRetryImport, useReviewAsset } from "@/hooks/useMutations";

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
 * The triage queue. Backed by GET /api/triage: every asset marked needs
 * review, however it got into the library, plus any whose latest import is in
 * flight or failed. Marking an asset reviewed happens here and nowhere else in
 * the panel or the package page. A failed import can be retried or dismissed.
 * Preview rendering still in progress shows below, and a retained tail of
 * recently reviewed assets closes the page.
 */
export function TriagePage() {
  const navigate = useNavigate();
  const [search, setSearch] = useSearchParams();
  const page = Math.max(1, Number(search.get("page")) || 1);
  const { data: assets, reviewed, total, loading, failed, reload } = useTriage(page);
  const { data: previews, finished: finishedPreviews } = usePreviewQueue();
  const { data: me } = useMe();
  useImportEvents(me?.current_library?.id);
  usePreviewEvents(me?.current_library?.id);

  const [reserved, setReserved] = useState(0);
  const onReservedWidthChange = useCallback((w: number) => setReserved(w), []);

  const reviewing = search.get("asset") ? Number(search.get("asset")) : null;
  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(search);
    if (value == null) next.delete(key);
    else next.set(key, value);
    setSearch(next, { replace: true });
  };
  const setReviewing = (id: number | null) => setParam("asset", id == null ? null : String(id));

  // A finished rollup only matters here if some of its previews failed.
  const processing = [...previews, ...finishedPreviews.filter((p) => p.failed > 0)];
  const pageCount = Math.ceil(total / TRIAGE_PAGE);
  const empty = total === 0 && processing.length === 0 && reviewed.length === 0;

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${total}`}>
              <BreadcrumbItem current heading>
                Triage
              </BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <LoadingState />
          ) : failed ? (
            <ErrorState title="Couldn't load the triage queue" onRetry={reload} />
          ) : empty ? (
            <EmptyState
              icon={<Inbox size={28} />}
              title="Nothing to triage"
              description="Assets marked needs review, imports in progress and failed imports show up here."
            />
          ) : (
            <PageContent
              style={{ paddingRight: reserved ? reserved + 14 : undefined }}
              className="gap-2"
            >
              <SectionHeading>Needs attention</SectionHeading>
              {assets.length === 0 ? (
                <div className="px-1 py-6 text-center text-ui text-subtle-foreground">
                  Nothing waiting for review.
                </div>
              ) : (
                assets.map((row) => (
                  <TriageQueueRow
                    key={row.asset_id}
                    row={row}
                    active={reviewing === row.asset_id}
                    onOpen={() => setReviewing(row.asset_id)}
                  />
                ))
              )}
              {pageCount > 1 && (
                <Pager
                  className="self-center"
                  page={page}
                  pageCount={pageCount}
                  onPageChange={(p) => setParam("page", p === 1 ? null : String(p))}
                />
              )}
              {processing.length > 0 && (
                <>
                  <SectionHeading>Processing previews</SectionHeading>
                  {processing.map((row) => (
                    <PreviewQueueRow
                      key={row.version_id}
                      row={row}
                      onOpen={() => setReviewing(row.asset_id)}
                    />
                  ))}
                </>
              )}
              {reviewed.length > 0 && (
                <>
                  <SectionHeading>Recently reviewed</SectionHeading>
                  {reviewed.map((row) => (
                    <ReviewedRow
                      key={row.asset_id}
                      row={row}
                      active={reviewing === row.asset_id}
                      onOpen={() => setReviewing(row.asset_id)}
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

function SectionHeading({ children }: { children: ReactNode }) {
  return (
    <h2 className="mt-4 mb-1 px-1 text-caption uppercase tracking-wide text-faint-foreground first:mt-0">
      {children}
    </h2>
  );
}

const IN_FLIGHT_STAGES = ["uploaded", "extracting", "detecting", "thumbnailing"];

// TriageThumb shows the asset's thumbnail once it has one, else its state:
// spinner in flight, warning on failure, placeholder otherwise.
function TriageThumb({ row }: { row: TriageRow }) {
  const url = useBlobUrl(
    row.thumbnail_object_key ? `/api/thumbs/${row.thumbnail_object_key}` : undefined,
  );
  if (url) return <img src={url} alt="" className="h-full w-full object-cover" />;
  if (row.import?.stage === "failed")
    return <AlertTriangle size={20} className="text-destructive" />;
  if (row.import && IN_FLIGHT_STAGES.includes(row.import.stage))
    return <Loader2 size={20} className="animate-spin text-subtle-foreground" />;
  return <Package size={20} className="text-faint-foreground" />;
}

function activeRowClass(active: boolean): string {
  return active ? "border-primary ring-1 ring-ring" : "";
}

// TriageStatus names the row's state in the panel's words: "Needs review" is
// the same badge the quick look shows, and an import only shows while it is
// still running or has failed.
function TriageStatus({ row }: { row: TriageRow }) {
  const imp = row.import;
  return (
    <>
      {imp?.stage === "failed" ? (
        <Badge size="sm" tone="err">
          Import failed
        </Badge>
      ) : imp ? (
        <Badge size="sm" tone="unk">
          Importing · {imp.stage}
        </Badge>
      ) : row.needs_review ? (
        <Badge size="sm" tone="warn">
          Needs review
        </Badge>
      ) : (
        <Badge size="sm" tone="ok">
          Reviewed
        </Badge>
      )}
      {row.matched ? (
        <Badge size="sm" tone="ok">
          Store match
        </Badge>
      ) : (
        <Badge size="sm">No store match</Badge>
      )}
      <span className="text-faint-foreground">
        updated {new Date(row.updated_at).toLocaleString()}
      </span>
    </>
  );
}

function TriageDetail({ row }: { row: TriageRow }) {
  if (row.import?.stage === "failed" && row.import.error) {
    return <p className="line-clamp-3 text-label text-destructive">{row.import.error}</p>;
  }
  const description = row.description ? plainSnippet(row.description) : "";
  return description ? (
    <p className="line-clamp-3 text-label text-subtle-foreground">{description}</p>
  ) : (
    <p className="text-label text-subtle-foreground">No description yet.</p>
  );
}

function TriageQueueRow({
  row,
  active,
  onOpen,
}: {
  row: TriageRow;
  active: boolean;
  onOpen: () => void;
}) {
  const retry = useRetryImport();
  const dismiss = useDismissImport();
  const review = useReviewAsset();
  const imp = row.import;
  const isFailed = imp?.stage === "failed";
  const inFlight = !!imp && !isFailed;

  return (
    <QueueItem
      className={cn("cursor-pointer", activeRowClass(active))}
      onClick={onOpen}
      thumbSize={80}
      name={row.asset_name || `Asset #${row.asset_id}`}
      thumb={<TriageThumb row={row} />}
      sub={<TriageStatus row={row} />}
      detail={<TriageDetail row={row} />}
      actions={
        // Peer actions, so one weight in one row; primary on every row of a
        // list would mean nothing.
        <div className="flex flex-wrap items-center gap-1" onClick={(e) => e.stopPropagation()}>
          <Button size="sm" variant="secondary" onClick={onOpen}>
            Open
          </Button>
          {isFailed && imp ? (
            <>
              <Button
                size="sm"
                variant="secondary"
                leadingIcon={<RotateCw size={12} />}
                disabled={retry.isPending}
                onClick={() => retry.mutate(imp.id)}
              >
                Retry
              </Button>
              <Button
                size="sm"
                variant="secondary"
                leadingIcon={<X size={12} />}
                title="Take this out of the queue. The asset stays in the library."
                disabled={dismiss.isPending}
                onClick={() => dismiss.mutate({ importId: imp.id, assetId: row.asset_id })}
              >
                Dismiss
              </Button>
            </>
          ) : (
            !inFlight && (
              <>
                <FindListingButton
                  assetId={row.asset_id}
                  assetName={row.asset_name}
                  label={row.matched ? "Re-match" : "Find listing"}
                />
                <Button
                  size="sm"
                  variant="secondary"
                  leadingIcon={<Check size={12} />}
                  disabled={review.isPending}
                  onClick={() => review.mutate(row.asset_id)}
                >
                  Mark reviewed
                </Button>
              </>
            )
          )}
        </div>
      }
    />
  );
}

function ReviewedRow({
  row,
  active,
  onOpen,
}: {
  row: TriageRow;
  active: boolean;
  onOpen: () => void;
}) {
  return (
    <QueueItem
      className={activeRowClass(active)}
      name={row.asset_name || `Asset #${row.asset_id}`}
      thumb={
        row.thumbnail_object_key ? (
          <TriageThumb row={row} />
        ) : (
          <CheckCircle2 size={20} className="text-subtle-foreground" />
        )
      }
      sub={<TriageStatus row={row} />}
      actions={
        <Button size="sm" variant="secondary" onClick={onOpen}>
          Open
        </Button>
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

  let thumb = <CheckCircle2 size={20} className="text-subtle-foreground" />;
  if (processing) thumb = <Loader2 size={20} className="animate-spin text-subtle-foreground" />;
  else if (hasFailures) thumb = <AlertTriangle size={20} className="text-destructive" />;

  return (
    <QueueItem
      name={row.asset_name || `Version #${row.version_id}`}
      thumb={thumb}
      sub={
        <>
          <Badge size="sm" tone={processing ? "unk" : hasFailures ? "err" : "ok"}>
            {processing ? "Rendering" : hasFailures ? "Some previews failed" : "Rendered"}
          </Badge>
          <span className="tabular-nums text-subtle-foreground">
            {row.done}/{attempted} previews
          </span>
          {hasFailures && <span className="text-destructive">{row.failed} failed</span>}
          <span className="text-faint-foreground">
            updated {new Date(row.updated_at).toLocaleString()}
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
