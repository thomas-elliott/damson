import { useEffect, useRef, useState } from "react";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
  FolderOpen,
  GitMerge,
  Maximize2,
  Minimize2,
  Pencil,
  RefreshCw,
  RotateCw,
  Trash2,
  X,
} from "lucide-react";

import { isNotFound } from "@/api";
import {
  Button,
  DetailPanel,
  EmptyState,
  ErrorState,
  FieldRow,
  IconButton,
  Input,
  KeyValueList,
  LicencePill,
  Markdown,
  MarkdownEditor,
  Pill,
  Section,
  StatusDot,
  Tag,
  VersionItem,
  cn,
  formatBytes,
  isBrowsable,
  useConfirm,
} from "@/design-system";
import { AuditList } from "@/components/AuditList";
import {
  downloadFile,
  useBlobUrls,
  type BlobSource,
} from "@/components/filebrowser/useFileObjectUrl";
import {
  useAssetAudit,
  useAssetDetail,
  type AssetDetailView,
  type AssetImageView,
  type AssetVersionView,
  type LicenceView,
} from "@/hooks/useDataQueries";
import {
  useAddAssetTag,
  useDeleteAsset,
  useDeleteVersion,
  usePatchAsset,
  usePatchVersion,
  useReExtractVersion,
  useRemoveAssetTag,
  useRequeueAssetPreviews,
  useRescanAsset,
  useReviewAsset,
  useReviewLicence,
} from "@/hooks/useMutations";

import { DetailsSection } from "./DetailsSection";
import { LicenceEditor } from "./LicenceEditor";
import { LocationSection } from "./LocationSection";
import { MergeDialog } from "./MergeDialog";
import { EnrichmentReview, MetadataActions } from "./MetadataActions";

const WIDTH_KEY = "damson.detail.w";
const DEFAULT_WIDTH = 440;
const MIN_WIDTH = 320;
const MOBILE_QUERY = "(max-width: 640px)";

function readStoredWidth(): number {
  if (typeof window === "undefined") return DEFAULT_WIDTH;
  const raw = window.localStorage.getItem(WIDTH_KEY);
  const n = raw ? Number(raw) : NaN;
  if (Number.isFinite(n) && n >= MIN_WIDTH) return n;
  return DEFAULT_WIDTH;
}

export function AssetDetailPanel({
  assetId,
  onClose,
  onBrowse,
  onReservedWidthChange,
}: {
  assetId: number | null;
  onClose: () => void;
  /** Open the scoped file browser for this asset (browsable packs only). */
  onBrowse?: (id: number) => void;
  /**
   * Reports how much horizontal space the open panel covers (0 when closed or
   * fullscreen), so a page whose content reaches the right edge can pad itself
   * out of the way instead of being overlapped.
   */
  onReservedWidthChange?: (width: number) => void;
}) {
  const { data: asset, loading, error, failed, reload } = useAssetDetail(assetId);
  const [width, setWidth] = useState<number>(readStoredWidth);
  const [userFullscreen, setUserFullscreen] = useState(false);
  const [autoFullscreen, setAutoFullscreen] = useState(false);
  const [maxWidth, setMaxWidth] = useState<number>(1024);
  const asideRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(WIDTH_KEY, String(width));
  }, [width]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia(MOBILE_QUERY);
    const update = () => setAutoFullscreen(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  // Track the panel's positioned ancestor so the user can't drag past
  // the available main-area width.
  useEffect(() => {
    const aside = asideRef.current;
    if (!aside) return;
    const parent = aside.offsetParent as HTMLElement | null;
    if (!parent) return;
    const update = () => setMaxWidth(parent.clientWidth);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(parent);
    return () => observer.disconnect();
  }, [assetId]);

  const isFullscreen = userFullscreen || autoFullscreen;
  // Clamp to the available space at render time (window resize, sidebar
  // widened) while preserving the user's chosen width in state, so it springs
  // back when room reappears.
  const effectiveWidth = Math.min(Math.max(MIN_WIDTH, width), maxWidth);
  const reservedWidth = assetId == null || isFullscreen ? 0 : effectiveWidth;

  useEffect(() => {
    onReservedWidthChange?.(reservedWidth);
  }, [reservedWidth, onReservedWidthChange]);

  if (assetId == null) return null;

  const canBrowse = !!asset && isBrowsable(asset.files.length);
  const headerActions = (
    <>
      {canBrowse && onBrowse && asset && (
        <IconButton
          aria-label="Browse files"
          title="Browse files"
          size="sm"
          onClick={() => onBrowse(asset.id)}
        >
          <FolderOpen />
        </IconButton>
      )}
      {!autoFullscreen && (
        <IconButton
          aria-label={userFullscreen ? "Exit fullscreen" : "Fullscreen"}
          size="sm"
          onClick={() => setUserFullscreen((v) => !v)}
        >
          {userFullscreen ? <Minimize2 /> : <Maximize2 />}
        </IconButton>
      )}
    </>
  );

  return (
    <DetailPanel
      ref={asideRef}
      onClose={onClose}
      title={asset?.name ?? (loading ? "Loading…" : "Asset")}
      variant="overlay"
      resizable
      width={effectiveWidth}
      onWidthChange={setWidth}
      minWidth={MIN_WIDTH}
      maxWidth={maxWidth}
      fullscreen={isFullscreen}
      actions={headerActions}
    >
      {/* key by asset id so edit state re-initialises from props when a
          different asset opens, instead of syncing via an effect. */}
      {asset ? (
        <AssetDetailBody
          key={asset.id}
          asset={asset}
          onBrowse={onBrowse ? () => onBrowse(asset.id) : undefined}
          onDeleted={onClose}
        />
      ) : failed && isNotFound(error) ? (
        <EmptyState title="Asset not found" description="It may have been deleted." />
      ) : failed ? (
        <ErrorState title="Couldn't load this asset" onRetry={reload} />
      ) : null}
    </DetailPanel>
  );
}

function AssetDetailBody({
  asset,
  onBrowse,
  onDeleted,
}: {
  asset: AssetDetailView;
  onBrowse?: () => void;
  onDeleted: () => void;
}) {
  const review = useReviewAsset();
  const del = useDeleteAsset();
  const addTag = useAddAssetTag();
  const removeTag = useRemoveAssetTag();
  const reviewLicence = useReviewLicence();
  const requeuePreviews = useRequeueAssetPreviews();
  const reextract = useReExtractVersion();
  const rescan = useRescanAsset();
  const [rescanNote, setRescanNote] = useState<string | null>(null);

  const byNewestImport = (a: AssetVersionView, b: AssetVersionView) =>
    b.import_date.localeCompare(a.import_date);
  const downloadSource = [...asset.versions]
    .filter((v) => v.has_original && v.original_package_object_key)
    .sort(byNewestImport)[0];
  const downloadKey = downloadSource?.original_package_object_key;
  const onDownloadOriginal =
    downloadSource && downloadKey
      ? () => {
          const ext = downloadSource.package_kind === "zip" ? "zip" : "unitypackage";
          void downloadFile(downloadKey, `${asset.name}.${ext}`).catch(() => {});
        }
      : undefined;
  const reExtractSource = [...asset.versions]
    .filter((v) => v.package_kind === "unitypackage" && v.has_original)
    .sort(byNewestImport)[0];
  const onReExtract = reExtractSource
    ? () =>
        reextract.mutate(
          { assetId: asset.id, versionId: reExtractSource.id },
          { onSuccess: () => requeuePreviews.mutate(asset.id) },
        )
    : undefined;
  const reExtractPending = reextract.isPending || requeuePreviews.isPending;

  const onRescan = () => {
    setRescanNote(null);
    rescan.mutate(asset.id, {
      onSuccess: (res) => {
        const parts: string[] = [];
        if (res.thumbnail_regenerated) parts.push("thumbnail refreshed");
        if (res.previews_queued > 0) parts.push(`${res.previews_queued} previews queued`);
        if (res.licence_updated) parts.push(`licence ${res.detected_type || "detected"}`);
        setRescanNote(
          parts.length > 0
            ? `Rescanned — ${parts.join(" · ")}`
            : "Rescan complete — nothing to update",
        );
      },
    });
  };

  const [newTag, setNewTag] = useState("");
  const [licenceOpen, setLicenceOpen] = useState(false);
  const [mergeOpen, setMergeOpen] = useState(false);
  const { confirm, dialog: confirmDialog } = useConfirm();

  const onAddTag = () => {
    const v = newTag.trim();
    if (!v) return;
    addTag.mutate({ assetId: asset.id, path: v }, { onSuccess: () => setNewTag("") });
  };

  return (
    <div className="@container flex flex-col gap-3 p-3">
      {/* The name lives in the panel header; only status chips sit here. */}
      {(asset.needs_review || asset.is_reference) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {asset.needs_review && (
            <button
              type="button"
              onClick={() => review.mutate(asset.id)}
              className="inline-flex items-center gap-1 rounded-full border border-warning-soft bg-warning-soft px-2 py-0.5 text-mono text-warning"
            >
              <StatusDot tone="warn" /> Needs review
              <Check size={11} />
            </button>
          )}
          {asset.is_reference && (
            <>
              {asset.publisher && (
                <span className="text-label text-muted-foreground">{asset.publisher}</span>
              )}
              <Pill tone="neutral">Reference · not stored</Pill>
            </>
          )}
        </div>
      )}

      {/* Storefront gallery — above Overview so the pictures read as the hero. */}
      <AssetGallery images={asset.images} />

      <OverviewSection asset={asset} />

      <DetailsSection asset={asset} />

      {/* Tags */}
      <Section title="Tags" accessory={`${asset.tags.length}`} defaultOpen>
        <div className="flex flex-wrap items-center gap-1.5">
          {asset.tags.map((t) => (
            <Tag
              key={t.id}
              aria-label={`Remove ${t.path}`}
              onClick={() => removeTag.mutate({ assetId: asset.id, tagId: t.id })}
            >
              {t.path}
              <span aria-hidden className="ml-1 text-subtle-foreground">
                ×
              </span>
            </Tag>
          ))}
          <Input
            value={newTag}
            onChange={(e) => setNewTag(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                onAddTag();
              }
            }}
            placeholder="environment/trees"
            aria-label="New tag path"
            className="h-7 w-[180px] text-label"
          />
        </div>
      </Section>

      {/* A reference asset holds a location in place of versions and files. */}
      {asset.is_reference ? (
        <LocationSection asset={asset} />
      ) : (
        <StoredContent
          asset={asset}
          onBrowse={onBrowse}
          onDownloadOriginal={onDownloadOriginal}
          onReExtract={onReExtract}
          reExtractPending={reExtractPending}
          onRescan={onRescan}
          rescanPending={rescan.isPending}
          rescanNote={rescanNote}
          onRequeuePreviews={() => requeuePreviews.mutate(asset.id)}
          requeuePending={requeuePreviews.isPending}
        />
      )}

      {/* Licence */}
      <Section
        title="Licence"
        open={licenceOpen}
        onOpenChange={setLicenceOpen}
        accessory={licenceOpen ? null : <LicenceSummary licence={asset.licence ?? null} />}
      >
        <LicenceEditor
          assetId={asset.id}
          existing={asset.licence ?? null}
          onReviewed={() => reviewLicence.mutate(asset.id)}
        />
      </Section>

      {/* Projects using this asset */}
      <Section title="Projects" accessory={`${asset.projects.length}`} defaultOpen={false}>
        {asset.projects.length === 0 ? (
          <div className="text-label text-faint-foreground">Not pinned to any project yet</div>
        ) : (
          <KeyValueList
            entries={asset.projects.map((p) => ({
              key: p.project_name,
              value: (
                <span className="font-mono text-muted-foreground">
                  {/* A reference asset's pin carries no version. */}
                  {asset.is_reference ? "Reference" : p.version_label}
                </span>
              ),
              mono: false,
            }))}
          />
        )}
      </Section>

      {/* Collections */}
      <Section title="Collections" accessory={`${asset.collections.length}`} defaultOpen={false}>
        {asset.collections.length === 0 ? (
          <div className="text-label text-faint-foreground">Not in any collections</div>
        ) : (
          <ul className="flex flex-col gap-0.5 text-label text-muted-foreground">
            {asset.collections.map((c) => (
              <li key={c.collection_id}>{c.collection_name}</li>
            ))}
          </ul>
        )}
      </Section>

      {/* Audit */}
      <Section title="Audit" defaultOpen={false}>
        <AssetAuditSection assetId={asset.id} />
      </Section>

      {/* Footer actions */}
      <div className="mt-2 flex items-center justify-between border-t border-border pt-3">
        <div className="flex items-center gap-2">
          <Button
            variant="destructive"
            leadingIcon={<Trash2 size={12} />}
            onClick={async () => {
              const ok = await confirm({
                title: `Delete "${asset.name}"?`,
                description: asset.is_reference
                  ? "This removes the record. Nothing at its path is touched."
                  : "This removes all versions and files.",
                confirmLabel: "Delete asset",
                tone: "destructive",
              });
              if (ok) del.mutate(asset.id, { onSuccess: onDeleted });
            }}
          >
            Delete asset
          </Button>
          {/* Merging moves versions, which a reference asset has none of. */}
          {!asset.is_reference && (
            <Button
              variant="ghost"
              leadingIcon={<GitMerge size={12} />}
              onClick={() => setMergeOpen(true)}
            >
              Merge…
            </Button>
          )}
        </div>
      </div>
      <MergeDialog
        targetId={asset.id}
        targetName={asset.name}
        open={mergeOpen}
        onOpenChange={setMergeOpen}
      />
      {confirmDialog}
    </div>
  );
}

// StoredContent is the versions and files of an asset whose data was imported.
function StoredContent({
  asset,
  onBrowse,
  onDownloadOriginal,
  onReExtract,
  reExtractPending,
  onRescan,
  rescanPending,
  rescanNote,
  onRequeuePreviews,
  requeuePending,
}: {
  asset: AssetDetailView;
  onBrowse?: () => void;
  onDownloadOriginal?: () => void;
  onReExtract?: () => void;
  reExtractPending: boolean;
  onRescan: () => void;
  rescanPending: boolean;
  rescanNote: string | null;
  onRequeuePreviews: () => void;
  requeuePending: boolean;
}) {
  return (
    <>
      {/* Versions */}
      <Section title="Versions" accessory={`${asset.versions.length}`} defaultOpen>
        <div className="flex flex-col gap-0.5">
          {asset.versions.map((v) => (
            <VersionRow key={v.id} assetId={asset.id} version={v} />
          ))}
        </div>
      </Section>

      {/* Files (current version) */}
      <Section title="Files" accessory={`${asset.files.length}`} defaultOpen={false}>
        {isBrowsable(asset.files.length) && onBrowse && (
          <Button
            size="sm"
            variant="secondary"
            leadingIcon={<FolderOpen size={12} />}
            className="mb-2 w-full justify-center"
            onClick={onBrowse}
          >
            Browse all {asset.files.length} files
          </Button>
        )}
        {onDownloadOriginal && (
          <Button
            size="sm"
            variant="secondary"
            leadingIcon={<Download size={12} />}
            className="mb-2 w-full justify-center"
            onClick={onDownloadOriginal}
          >
            Download original
          </Button>
        )}
        {onReExtract && (
          <Button
            size="sm"
            variant="secondary"
            leadingIcon={<RotateCw size={12} />}
            className="mb-2 w-full justify-center"
            disabled={reExtractPending}
            onClick={onReExtract}
          >
            {reExtractPending ? "Re-extracting…" : "Re-extract files"}
          </Button>
        )}
        <Button
          size="sm"
          variant="secondary"
          leadingIcon={<RefreshCw size={12} />}
          className="mb-2 w-full justify-center"
          disabled={rescanPending}
          onClick={onRescan}
        >
          {rescanPending ? "Rescanning…" : "Rescan package"}
        </Button>
        {rescanNote && (
          <div className="mb-2 px-1 text-label text-subtle-foreground">{rescanNote}</div>
        )}
        <Button
          size="sm"
          variant="ghost"
          leadingIcon={<RefreshCw size={12} />}
          className="mb-2 w-full justify-center"
          disabled={requeuePending}
          onClick={onRequeuePreviews}
        >
          Regenerate previews only
        </Button>
        <div className="flex items-center justify-between rounded-sm border border-border bg-muted px-2.5 py-2 text-label text-muted-foreground">
          <span>
            {asset.files.length} {asset.files.length === 1 ? "file" : "files"}
          </span>
          <span className="text-mono tabular-nums text-subtle-foreground">
            {formatBytes(asset.files.reduce((sum, f) => sum + f.size_bytes, 0))}
          </span>
        </div>
      </Section>
    </>
  );
}

// OverviewSection reads as rendered prose and flips to a form on demand: the
// description is markdown (storefront enrichment writes it that way), so a
// permanent textarea showed raw syntax for a field most users only ever read.
// An asset with no description opens straight into the form.
function OverviewSection({ asset }: { asset: AssetDetailView }) {
  // "auto" opens the form for a description-less asset without stealing focus
  // from the grid; only an explicit Edit puts the caret in the editor.
  const [editing, setEditing] = useState<"no" | "auto" | "user">(
    asset.description.trim() === "" ? "auto" : "no",
  );
  const [open, setOpen] = useState(true);

  const startEditing = () => {
    setEditing("user");
    setOpen(true);
  };

  return (
    <Section
      title="Overview"
      open={open}
      onOpenChange={setOpen}
      accessory={
        editing === "no" ? (
          <Button
            size="sm"
            variant="ghost"
            leadingIcon={<Pencil size={12} />}
            onClick={startEditing}
          >
            Edit
          </Button>
        ) : null
      }
    >
      <div className="flex flex-col gap-2.5">
        {editing === "no" ? (
          <OverviewRead asset={asset} />
        ) : (
          // Mounted on the flip into edit mode, so its fields seed from
          // whatever the server last returned — including a metadata fetch
          // that landed while the panel sat in read mode.
          <OverviewForm
            asset={asset}
            autoFocus={editing === "user"}
            onDone={() => setEditing("no")}
          />
        )}
      </div>
    </Section>
  );
}

// The collapsed Licence header carries the identifier, so the one fact people
// scan for doesn't need the section opened. Expanded, LicenceEditor shows the
// same pill, so the accessory drops away.
function LicenceSummary({ licence }: { licence: LicenceView | null }) {
  if (!licence) return <span className="text-label text-faint-foreground">Not set</span>;
  return <LicencePill status={licence.status} spdx={licence.detected_type || "Unknown"} noDot />;
}

function OverviewRead({ asset }: { asset: AssetDetailView }) {
  const source = asset.source_url.trim();
  return (
    <>
      {asset.description.trim() ? (
        <ClampedDescription>{asset.description}</ClampedDescription>
      ) : (
        <p className="text-label text-faint-foreground">No description yet.</p>
      )}
      {/* Source link and the fetch actions share one line — they're all about
          where this asset's metadata comes from. */}
      <div className="flex items-start justify-between gap-2">
        {source ? (
          <a
            href={source}
            target="_blank"
            rel="noreferrer noopener"
            title={source}
            className="inline-flex min-w-0 items-center gap-1 pt-1 text-label text-primary hover:underline"
          >
            <span className="truncate">{sourceLabel(source)}</span>
            <ExternalLink size={11} className="shrink-0" />
          </a>
        ) : (
          <span className="pt-1 text-label text-faint-foreground">No source URL</span>
        )}
        <MetadataActions asset={asset} />
      </div>
    </>
  );
}

// Store descriptions run to hundreds of lines, which buried every section below
// Overview. Collapsed height is measured in ems of the body text so the clamp
// stays five lines however the type scale moves; the toggle only appears when
// the content actually exceeds it (measured on the content, not the clamped
// box, so it stays correct while expanded).
const CLAMP_LINES = 5;
const CLAMP_LINE_HEIGHT = 1.4;
const CLAMP_EM = `${CLAMP_LINES * CLAMP_LINE_HEIGHT}em`;

function ClampedDescription({ children }: { children: string }) {
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);
  const content = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const el = content.current;
    if (!el) return;
    const measure = () => {
      const limit = parseFloat(getComputedStyle(el).fontSize) * CLAMP_LINES * CLAMP_LINE_HEIGHT;
      setOverflows(el.scrollHeight > limit + 1);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [children]);

  return (
    <div className="flex flex-col gap-1">
      <div
        className="relative overflow-hidden"
        style={expanded ? undefined : { maxHeight: CLAMP_EM }}
      >
        <Markdown ref={content}>{children}</Markdown>
        {!expanded && overflows && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-6 bg-gradient-to-t from-card to-transparent" />
        )}
      </div>
      {overflows && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="self-start text-label text-primary hover:underline"
        >
          {expanded ? "Show less" : "Read more"}
        </button>
      )}
    </div>
  );
}

function OverviewForm({
  asset,
  autoFocus,
  onDone,
}: {
  asset: AssetDetailView;
  autoFocus?: boolean;
  onDone: () => void;
}) {
  const patch = usePatchAsset();
  const [name, setName] = useState(asset.name);
  const [description, setDescription] = useState(asset.description);
  const [sourceURL, setSourceURL] = useState(asset.source_url);

  const onSave = () => {
    patch.mutate(
      {
        id: asset.id,
        name: name.trim() || asset.name,
        description,
        ...(asset.is_reference ? {} : { source_url: sourceURL.trim() }),
      },
      { onSuccess: onDone },
    );
  };

  return (
    <div className="flex flex-col gap-2.5">
      <FieldRow label="Name">
        <Input
          value={name}
          onChange={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") onSave();
          }}
        />
      </FieldRow>
      <FieldRow label="Description">
        <MarkdownEditor
          value={description}
          onChange={setDescription}
          aria-label="Description"
          placeholder="What's this asset for? Markdown is supported."
          maxHeight={320}
          autoFocus={autoFocus}
        />
      </FieldRow>
      {/* A reference asset's store link is edited in Location, which keeps it
          and the path from both being cleared. */}
      {!asset.is_reference && (
        <FieldRow label="Source URL">
          <Input
            value={sourceURL}
            onChange={(e) => setSourceURL(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") onSave();
            }}
            placeholder="https://…"
          />
        </FieldRow>
      )}
      <EnrichmentReview asset={asset} />
      <div className="flex items-center justify-end gap-1.5">
        <Button variant="ghost" size="sm" onClick={onDone} disabled={patch.isPending}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" onClick={onSave} disabled={patch.isPending}>
          {patch.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
    </div>
  );
}

function sourceLabel(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
}

function thumbSource(image: AssetImageView): BlobSource {
  return { url: `/api/thumbs/${image.object_key}`, mime: image.content_type };
}

function wrapIndex(next: number, count: number): number {
  return ((next % count) + count) % count;
}

// AssetGallery shows the storefront gallery one image at a time — prev/next
// step through it, a click opens the lightbox on the image you're looking at.
// Every image stays mounted in the sliding track so stepping doesn't re-fetch.
function AssetGallery({ images }: { images: AssetImageView[] }) {
  const urls = useBlobUrls(images.map(thumbSource));
  const [index, setIndex] = useState(0);
  const [zoomed, setZoomed] = useState(false);

  if (images.length === 0) return null;

  const current = Math.min(index, images.length - 1);
  const step = (delta: number) => setIndex(wrapIndex(current + delta, images.length));

  return (
    <div className="relative aspect-video w-full overflow-hidden rounded-md border border-border bg-muted">
      <div
        className="flex h-full w-full transition-transform duration-fast ease-standard"
        style={{ transform: `translateX(-${current * 100}%)` }}
      >
        {images.map((img, i) => (
          <button
            key={img.id}
            type="button"
            onClick={() => {
              setIndex(i);
              setZoomed(true);
            }}
            aria-label={`Enlarge preview image ${i + 1} of ${images.length}`}
            className="h-full w-full shrink-0 cursor-zoom-in"
          >
            <GalleryFrame url={urls[thumbSource(img).url]} className="object-cover" />
          </button>
        ))}
      </div>
      {images.length > 1 && (
        <>
          <GalleryArrow side="left" onClick={() => step(-1)} />
          <GalleryArrow side="right" onClick={() => step(1)} />
          <GalleryCount current={current} total={images.length} />
        </>
      )}
      {zoomed && (
        <GalleryLightbox
          images={images}
          urls={urls}
          index={current}
          onIndexChange={setIndex}
          onClose={() => setZoomed(false)}
        />
      )}
    </div>
  );
}

function GalleryFrame({ url, className }: { url?: string; className?: string }) {
  if (!url) return <div className="h-full w-full animate-pulse bg-muted" />;
  return <img src={url} alt="" className={cn("h-full w-full", className)} />;
}

function GalleryArrow({ side, onClick }: { side: "left" | "right"; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? "Previous image" : "Next image"}
      className={cn(
        "absolute top-1/2 grid h-7 w-7 -translate-y-1/2 place-items-center rounded-full",
        "bg-black/55 text-white transition-colors duration-fast ease-standard hover:bg-black/80",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        side === "left" ? "left-1.5" : "right-1.5",
      )}
    >
      {side === "left" ? <ChevronLeft size={16} /> : <ChevronRight size={16} />}
    </button>
  );
}

function GalleryCount({ current, total }: { current: number; total: number }) {
  return (
    <span className="pointer-events-none absolute bottom-1.5 right-1.5 rounded-full bg-black/60 px-1.5 py-0.5 text-mono tabular-nums text-white">
      {current + 1} / {total}
    </span>
  );
}

// GalleryLightbox is the full-screen view of the same carousel. Keys are handled
// on the dialog itself (not the document) and propagation stopped, so the page's
// own arrow/escape shortcuts don't move the grid selection behind it.
function GalleryLightbox({
  images,
  urls,
  index,
  onIndexChange,
  onClose,
}: {
  images: AssetImageView[];
  urls: Record<string, string>;
  index: number;
  onIndexChange: (next: number) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    ref.current?.focus();
  }, []);

  const step = (delta: number) => onIndexChange(wrapIndex(index + delta, images.length));

  return (
    <div
      ref={ref}
      role="dialog"
      aria-modal="true"
      aria-label={`Preview image ${index + 1} of ${images.length}`}
      tabIndex={-1}
      onClick={onClose}
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        else if (e.key === "ArrowLeft") step(-1);
        else if (e.key === "ArrowRight") step(1);
        else return;
        e.preventDefault();
        e.stopPropagation();
      }}
      style={{ zIndex: "var(--z-index-dialog)" }}
      className="fixed inset-0 flex items-center justify-center bg-black/80 p-6 focus-visible:outline-hidden"
    >
      <IconButton
        aria-label="Close preview"
        size="lg"
        onClick={onClose}
        // hover:text-white is not redundant: `ghost` carries hover:text-foreground,
        // which tailwind-merge keeps as a separate modifier, and that is near-black
        // in light theme.
        className="absolute right-4 top-4 text-white hover:bg-white/20 hover:text-white"
      >
        <X />
      </IconButton>
      <img
        src={urls[thumbSource(images[index]).url]}
        alt=""
        onClick={(e) => e.stopPropagation()}
        className="max-h-full max-w-full rounded-md object-contain"
      />
      {images.length > 1 && (
        <div onClick={(e) => e.stopPropagation()}>
          <LightboxArrow side="left" onClick={() => step(-1)} />
          <LightboxArrow side="right" onClick={() => step(1)} />
          <span className="pointer-events-none absolute bottom-6 left-1/2 -translate-x-1/2 rounded-full bg-black/60 px-2 py-0.5 text-mono tabular-nums text-white">
            {index + 1} / {images.length}
          </span>
        </div>
      )}
    </div>
  );
}

function LightboxArrow({ side, onClick }: { side: "left" | "right"; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? "Previous image" : "Next image"}
      className={cn(
        "absolute top-1/2 grid h-10 w-10 -translate-y-1/2 place-items-center rounded-full",
        "bg-black/55 text-white transition-colors duration-fast ease-standard hover:bg-black/80",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        side === "left" ? "left-4" : "right-4",
      )}
    >
      {side === "left" ? <ChevronLeft size={22} /> : <ChevronRight size={22} />}
    </button>
  );
}

function VersionRow({ assetId, version }: { assetId: number; version: AssetVersionView }) {
  const patch = usePatchVersion();
  const del = useDeleteVersion();
  const { confirm, dialog } = useConfirm();
  const [editing, setEditing] = useState(false);
  const [store, setStore] = useState(version.store_version);
  const [label, setLabel] = useState(version.version_label);

  const onSave = () => {
    patch.mutate(
      { assetId, versionId: version.id, store_version: store.trim(), version_label: label.trim() },
      { onSuccess: () => setEditing(false) },
    );
  };

  const onDelete = async () => {
    const ok = await confirm({
      title: "Delete this version?",
      description: "Its files are removed. A pinned version can't be deleted.",
      confirmLabel: "Delete version",
      tone: "destructive",
    });
    if (ok) del.mutate({ assetId, versionId: version.id });
  };

  const editAction = editing ? (
    <span className="flex items-center gap-1">
      <Input
        value={store}
        onChange={(e) => setStore(e.currentTarget.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onSave();
          if (e.key === "Escape") setEditing(false);
        }}
        placeholder="1.1"
        aria-label="Store version"
        autoFocus
        className="h-7 w-16 text-label"
      />
      <Input
        value={label}
        onChange={(e) => setLabel(e.currentTarget.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onSave();
          if (e.key === "Escape") setEditing(false);
        }}
        placeholder="label"
        aria-label="Version label"
        className="h-7 w-20 text-label"
      />
      <IconButton aria-label="Save version" size="sm" onClick={onSave} disabled={patch.isPending}>
        <Check />
      </IconButton>
      <IconButton aria-label="Cancel edit" size="sm" onClick={() => setEditing(false)}>
        <X />
      </IconButton>
    </span>
  ) : (
    <span className="flex items-center gap-1">
      <IconButton
        aria-label="Edit version"
        title="Edit store version and label"
        size="sm"
        onClick={() => {
          setStore(version.store_version);
          setLabel(version.version_label);
          setEditing(true);
        }}
      >
        <Pencil />
      </IconButton>
      <IconButton
        aria-label="Delete version"
        title="Delete this version"
        size="sm"
        onClick={onDelete}
        disabled={del.isPending}
      >
        <Trash2 />
      </IconButton>
    </span>
  );

  return (
    <>
      <VersionItem
        version={version.version_label || version.store_version || "unversioned"}
        storeVersion={version.store_version || undefined}
        date={new Date(version.import_date).toLocaleDateString()}
        meta={`${version.file_count} files · ${formatBytes(version.size_bytes)}`}
        current={version.is_current}
        notes={version.notes || undefined}
        action={editAction}
      />
      {dialog}
    </>
  );
}

function AssetAuditSection({ assetId }: { assetId: number }) {
  const { data: entries, loading } = useAssetAudit(assetId);
  if (loading) {
    return <div className="text-label text-faint-foreground">Loading…</div>;
  }
  return (
    <AuditList
      entries={entries.slice(0, 25)}
      emptyTitle="No activity yet"
      emptyDescription="Edits to this asset will show up here."
    />
  );
}
