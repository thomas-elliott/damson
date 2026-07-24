import { useEffect, useRef, useState } from "react";
import {
  Check,
  Download,
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

import {
  Button,
  DetailPanel,
  FieldRow,
  IconButton,
  Input,
  KeyValueList,
  LicencePill,
  Section,
  StatusDot,
  Tag,
  Textarea,
  VersionItem,
  formatBytes,
  isBrowsable,
  useConfirm,
} from "@/design-system";
import { AuditList } from "@/components/AuditList";
import { downloadFile, useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";
import {
  useAssetAudit,
  useAssetDetail,
  type AssetDetailView,
  type AssetImageView,
  type AssetVersionView,
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

import { LicenceEditor } from "./LicenceEditor";
import { MergeDialog } from "./MergeDialog";
import { MetadataActions } from "./MetadataActions";

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
}: {
  assetId: number | null;
  onClose: () => void;
  /** Open the scoped file browser for this asset (browsable packs only). */
  onBrowse?: (id: number) => void;
}) {
  const { data: asset, loading } = useAssetDetail(assetId);
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

  if (assetId == null) return null;

  const isFullscreen = userFullscreen || autoFullscreen;
  // Clamp to the available space at render time (window resize, sidebar
  // widened) while preserving the user's chosen width in state, so it springs
  // back when room reappears.
  const effectiveWidth = Math.min(Math.max(MIN_WIDTH, width), maxWidth);

  const canBrowse = !!asset && isBrowsable(asset.files.length);
  const headerActions = (
    <>
      {canBrowse && onBrowse && asset && (
        <IconButton
          aria-label="Browse files"
          title="Browse files"
          onClick={() => onBrowse(asset.id)}
        >
          <FolderOpen size={14} />
        </IconButton>
      )}
      {!autoFullscreen && (
        <IconButton
          aria-label={userFullscreen ? "Exit fullscreen" : "Fullscreen"}
          onClick={() => setUserFullscreen((v) => !v)}
        >
          {userFullscreen ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
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
      {asset && (
        <AssetDetailBody
          key={asset.id}
          asset={asset}
          onBrowse={onBrowse ? () => onBrowse(asset.id) : undefined}
        />
      )}
    </DetailPanel>
  );
}

function AssetDetailBody({ asset, onBrowse }: { asset: AssetDetailView; onBrowse?: () => void }) {
  const patch = usePatchAsset();
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

  const [editingName, setEditingName] = useState(false);
  const [name, setName] = useState(asset.name);
  const [description, setDescription] = useState(asset.description);
  const [sourceURL, setSourceURL] = useState(asset.source_url);
  const [newTag, setNewTag] = useState("");
  const [mergeOpen, setMergeOpen] = useState(false);
  const { confirm, dialog: confirmDialog } = useConfirm();

  const onSaveHeader = () => {
    patch.mutate(
      { id: asset.id, name, description, source_url: sourceURL },
      { onSuccess: () => setEditingName(false) },
    );
  };

  const onAddTag = () => {
    const v = newTag.trim();
    if (!v) return;
    addTag.mutate({ assetId: asset.id, path: v }, { onSuccess: () => setNewTag("") });
  };

  return (
    <div className="@container flex flex-col gap-3 p-3">
      {/* Hero: name + licence + needs-review. */}
      <div className="flex flex-col gap-2">
        {editingName ? (
          <FieldRow label="Name">
            <Input
              value={name}
              onChange={(e) => setName(e.currentTarget.value)}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter") onSaveHeader();
                if (e.key === "Escape") setEditingName(false);
              }}
            />
          </FieldRow>
        ) : (
          <button
            type="button"
            onClick={() => setEditingName(true)}
            className="group flex items-center gap-2 text-left text-heading-medium text-text"
          >
            <span className="truncate">{asset.name}</span>
            <Pencil
              size={12}
              className="opacity-0 transition-opacity duration-std ease group-hover:opacity-100 text-text-3"
            />
          </button>
        )}
        <div className="flex flex-wrap items-center gap-1.5">
          {asset.licence && (
            <LicencePill
              status={asset.licence.status}
              spdx={asset.licence.detected_type || "Unknown"}
            />
          )}
          {asset.needs_review && (
            <button
              type="button"
              onClick={() => review.mutate(asset.id)}
              className="inline-flex items-center gap-1 rounded-pill border border-warn-tint bg-warn-tint px-2 py-0.5 text-mono text-warn"
            >
              <StatusDot tone="warn" /> Needs review
              <Check size={11} />
            </button>
          )}
        </div>
      </div>

      {/* Description + source url. */}
      <Section title="Overview" defaultOpen>
        <div className="flex flex-col gap-2.5">
          <FieldRow label="Description">
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.currentTarget.value)}
              onBlur={() => {
                if (description !== asset.description) {
                  patch.mutate({ id: asset.id, description });
                }
              }}
              placeholder="What's this asset for?"
              rows={3}
            />
          </FieldRow>
          <FieldRow label="Source URL">
            <Input
              value={sourceURL}
              onChange={(e) => setSourceURL(e.currentTarget.value)}
              onBlur={() => {
                if (sourceURL !== asset.source_url) {
                  patch.mutate({ id: asset.id, source_url: sourceURL });
                }
              }}
              placeholder="https://…"
            />
          </FieldRow>
          <MetadataActions asset={asset} />
          <GalleryStrip images={asset.images} />
        </div>
      </Section>

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
              <span aria-hidden className="ml-1 text-text-3">
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
            className="h-7 w-[180px] text-small"
          />
        </div>
      </Section>

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
          disabled={rescan.isPending}
          onClick={onRescan}
        >
          {rescan.isPending ? "Rescanning…" : "Rescan package"}
        </Button>
        {rescanNote && <div className="mb-2 px-1 text-small text-text-3">{rescanNote}</div>}
        <Button
          size="sm"
          variant="ghost"
          leadingIcon={<RefreshCw size={12} />}
          className="mb-2 w-full justify-center"
          disabled={requeuePreviews.isPending}
          onClick={() => requeuePreviews.mutate(asset.id)}
        >
          Regenerate previews only
        </Button>
        <div className="flex items-center justify-between rounded-sm border border-line bg-surface px-2.5 py-2 text-small text-text-2">
          <span>
            {asset.files.length} {asset.files.length === 1 ? "file" : "files"}
          </span>
          <span className="text-mono tabular-nums text-text-3">
            {formatBytes(asset.files.reduce((sum, f) => sum + f.size_bytes, 0))}
          </span>
        </div>
      </Section>

      {/* Licence */}
      <Section title="Licence" defaultOpen>
        <LicenceEditor
          assetId={asset.id}
          existing={asset.licence ?? null}
          onReviewed={() => reviewLicence.mutate(asset.id)}
        />
      </Section>

      {/* Projects using this asset */}
      <Section title="Projects" accessory={`${asset.projects.length}`} defaultOpen={false}>
        {asset.projects.length === 0 ? (
          <div className="text-small text-text-4">Not pinned to any project yet</div>
        ) : (
          <KeyValueList
            entries={asset.projects.map((p) => ({
              key: p.project_name,
              value: <span className="font-mono text-text-2">{p.version_label}</span>,
              mono: false,
            }))}
          />
        )}
      </Section>

      {/* Collections */}
      <Section title="Collections" accessory={`${asset.collections.length}`} defaultOpen={false}>
        {asset.collections.length === 0 ? (
          <div className="text-small text-text-4">Not in any collections</div>
        ) : (
          <ul className="flex flex-col gap-0.5 text-small text-text-2">
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
      <div className="mt-2 flex items-center justify-between border-t border-line pt-3">
        <div className="flex items-center gap-2">
          <Button
            variant="danger"
            leadingIcon={<Trash2 size={12} />}
            onClick={async () => {
              const ok = await confirm({
                title: `Delete "${asset.name}"?`,
                description: "This removes all versions and files.",
                confirmLabel: "Delete asset",
                tone: "danger",
              });
              if (ok) del.mutate(asset.id);
            }}
          >
            Delete asset
          </Button>
          <Button
            variant="ghost"
            leadingIcon={<GitMerge size={12} />}
            onClick={() => setMergeOpen(true)}
          >
            Merge…
          </Button>
        </div>
        {editingName && (
          <Button variant="primary" onClick={onSaveHeader}>
            Save
          </Button>
        )}
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

function GalleryThumb({ image, onOpen }: { image: AssetImageView; onOpen: () => void }) {
  const url = useBlobUrl(`/api/thumbs/${image.object_key}`, image.content_type);
  return (
    <button
      type="button"
      onClick={onOpen}
      className="block h-16 w-24 shrink-0 cursor-zoom-in overflow-hidden rounded-md border border-line bg-surface"
      aria-label="Enlarge preview image"
    >
      {url ? (
        <img src={url} alt="" className="h-full w-full object-cover" loading="lazy" />
      ) : (
        <div className="h-full w-full animate-pulse bg-surface-2" />
      )}
    </button>
  );
}

function GalleryLightbox({ image, onClose }: { image: AssetImageView; onClose: () => void }) {
  const url = useBlobUrl(`/api/thumbs/${image.object_key}`, image.content_type);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Preview image"
      onClick={onClose}
      style={{ zIndex: "var(--z-index-dialog)" }}
      className="fixed inset-0 flex items-center justify-center bg-black/80 p-6"
    >
      <IconButton
        aria-label="Close preview"
        onClick={onClose}
        className="absolute right-4 top-4 text-white"
      >
        <X size={18} />
      </IconButton>
      {url && (
        <img
          src={url}
          alt=""
          onClick={(e) => e.stopPropagation()}
          className="max-h-full max-w-full rounded-md object-contain"
        />
      )}
    </div>
  );
}

function GalleryStrip({ images }: { images: AssetImageView[] }) {
  const [openId, setOpenId] = useState<number | null>(null);
  if (images.length === 0) return null;
  const open = images.find((img) => img.id === openId) ?? null;
  return (
    <>
      <div className="flex gap-1.5 overflow-x-auto pb-1">
        {images.map((img) => (
          <GalleryThumb key={img.id} image={img} onOpen={() => setOpenId(img.id)} />
        ))}
      </div>
      {open && <GalleryLightbox image={open} onClose={() => setOpenId(null)} />}
    </>
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
      tone: "danger",
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
        className="h-6 w-16 text-small"
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
        className="h-6 w-20 text-small"
      />
      <IconButton aria-label="Save version" onClick={onSave} disabled={patch.isPending}>
        <Check size={12} />
      </IconButton>
      <IconButton aria-label="Cancel edit" onClick={() => setEditing(false)}>
        <X size={12} />
      </IconButton>
    </span>
  ) : (
    <span className="flex items-center gap-1">
      <IconButton
        aria-label="Edit version"
        title="Edit store version and label"
        onClick={() => {
          setStore(version.store_version);
          setLabel(version.version_label);
          setEditing(true);
        }}
      >
        <Pencil size={12} />
      </IconButton>
      <IconButton
        aria-label="Delete version"
        title="Delete this version"
        onClick={onDelete}
        disabled={del.isPending}
      >
        <Trash2 size={12} />
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
    return <div className="text-small text-text-4">Loading…</div>;
  }
  return (
    <AuditList
      entries={entries.slice(0, 25)}
      emptyTitle="No activity yet"
      emptyDescription="Edits to this asset will show up here."
    />
  );
}
