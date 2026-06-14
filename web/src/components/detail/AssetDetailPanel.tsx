import { useEffect, useRef, useState } from "react";
import { Check, FolderOpen, Maximize2, Minimize2, Pencil, Trash2 } from "lucide-react";

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
import { useAssetAudit, useAssetDetail, type AssetDetailView } from "@/hooks/useDataQueries";
import {
  useAddAssetTag,
  useDeleteAsset,
  usePatchAsset,
  useRemoveAssetTag,
  useReviewAsset,
  useReviewLicence,
} from "@/hooks/useMutations";

import { LicenceEditor } from "./LicenceEditor";

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

  const [editingName, setEditingName] = useState(false);
  const [name, setName] = useState(asset.name);
  const [description, setDescription] = useState(asset.description);
  const [sourceURL, setSourceURL] = useState(asset.source_url);
  const [newTag, setNewTag] = useState("");
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
            <VersionItem
              key={v.id}
              version={v.version_label}
              date={new Date(v.import_date).toLocaleDateString()}
              meta={`${v.file_count} files · ${formatBytes(v.size_bytes)}`}
              current={v.is_current}
              notes={v.notes || undefined}
            />
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
        <ul className="flex flex-col gap-0.5">
          {asset.files.slice(0, 50).map((f) => (
            <li
              key={f.id}
              className="flex items-center justify-between gap-3 rounded-sm px-1.5 py-1 text-small hover:bg-surface"
            >
              <span className="truncate font-mono text-text-2" title={f.relative_path}>
                {f.relative_path}
              </span>
              <span className="text-mono text-text-3">{formatBytes(f.size_bytes)}</span>
            </li>
          ))}
          {asset.files.length > 50 && (
            <li className="px-1.5 py-1 text-caption">… {asset.files.length - 50} more files</li>
          )}
        </ul>
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
        {editingName && (
          <Button variant="primary" onClick={onSaveHeader}>
            Save
          </Button>
        )}
      </div>
      {confirmDialog}
    </div>
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
