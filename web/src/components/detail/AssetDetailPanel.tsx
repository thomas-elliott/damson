import { useEffect, useState } from "react";
import { Check, Pencil, Trash2 } from "lucide-react";

import {
  Button,
  DetailPanel,
  FieldRow,
  Input,
  KeyValueList,
  LicencePill,
  Section,
  StatusDot,
  Tag,
  Textarea,
  VersionItem,
  formatBytes,
} from "@/design-system";
import {
  useAssetDetail,
  type AssetDetailView,
} from "@/hooks/useDataQueries";
import {
  useAddAssetTag,
  useDeleteAsset,
  usePatchAsset,
  useRemoveAssetTag,
  useReviewAsset,
  useReviewLicence,
} from "@/hooks/useMutations";

import { LicenceEditor } from "./LicenceEditor";

export function AssetDetailPanel({
  assetId,
  onClose,
}: {
  assetId: number | null;
  onClose: () => void;
}) {
  const { data: asset, loading } = useAssetDetail(assetId);
  if (assetId == null) return null;

  return (
    <DetailPanel
      onClose={onClose}
      title={asset?.name ?? (loading ? "Loading…" : "Asset")}
      variant="overlay"
    >
      {asset && <AssetDetailBody asset={asset} />}
    </DetailPanel>
  );
}

function AssetDetailBody({ asset }: { asset: AssetDetailView }) {
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

  // Reset edit state whenever the loaded asset changes (different asset
  // opened in the panel).
  useEffect(() => {
    setEditingName(false);
    setName(asset.name);
    setDescription(asset.description);
    setSourceURL(asset.source_url);
  }, [asset.id]);

  const onSaveHeader = () => {
    patch.mutate(
      { id: asset.id, name, description, source_url: sourceURL },
      { onSuccess: () => setEditingName(false) },
    );
  };

  const onAddTag = () => {
    const v = newTag.trim();
    if (!v) return;
    addTag.mutate(
      { assetId: asset.id, path: v },
      { onSuccess: () => setNewTag("") },
    );
  };

  return (
    <div className="flex flex-col gap-3 p-3">
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
            className="group flex items-center gap-2 text-left text-xl font-medium text-text"
          >
            <span className="truncate">{asset.name}</span>
            <Pencil size={12} className="opacity-0 transition-opacity duration-std ease group-hover:opacity-100 text-text-3" />
          </button>
        )}
        <div className="flex flex-wrap items-center gap-1.5">
          {asset.licence && (
            <LicencePill status={asset.licence.status} spdx={asset.licence.detected_type || "Unknown"} />
          )}
          {asset.needs_review && (
            <button
              type="button"
              onClick={() => review.mutate(asset.id)}
              className="inline-flex items-center gap-1 rounded-pill border border-warn-tint bg-warn-tint px-2 py-0.5 font-mono text-xs text-warn"
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
              <span aria-hidden className="ml-1 text-text-3">×</span>
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
            className="h-7 w-[180px] text-xs"
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
        <ul className="flex flex-col gap-0.5">
          {asset.files.slice(0, 50).map((f) => (
            <li
              key={f.id}
              className="flex items-center justify-between gap-3 rounded-sm px-1.5 py-1 text-xs-plus hover:bg-surface"
            >
              <span className="truncate font-mono text-text-2" title={f.relative_path}>
                {f.relative_path}
              </span>
              <span className="font-mono text-xs text-text-3">{formatBytes(f.size_bytes)}</span>
            </li>
          ))}
          {asset.files.length > 50 && (
            <li className="px-1.5 py-1 text-xs text-text-3">
              … {asset.files.length - 50} more files
            </li>
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
          <div className="text-xs-plus text-text-4">Not pinned to any project yet</div>
        ) : (
          <KeyValueList
            entries={asset.projects.map((p) => ({
              key: p.project_name,
              value: (
                <span className="font-mono text-text-2">{p.version_label}</span>
              ),
              mono: false,
            }))}
          />
        )}
      </Section>

      {/* Collections */}
      <Section title="Collections" accessory={`${asset.collections.length}`} defaultOpen={false}>
        {asset.collections.length === 0 ? (
          <div className="text-xs-plus text-text-4">Not in any collections</div>
        ) : (
          <ul className="flex flex-col gap-0.5 text-xs-plus text-text-2">
            {asset.collections.map((c) => (
              <li key={c.collection_id}>{c.collection_name}</li>
            ))}
          </ul>
        )}
      </Section>

      {/* Footer actions */}
      <div className="mt-2 flex items-center justify-between border-t border-line pt-3">
        <Button
          variant="danger"
          leadingIcon={<Trash2 size={12} />}
          onClick={() => {
            if (confirm(`Delete "${asset.name}"? This removes all versions and files.`)) {
              del.mutate(asset.id);
            }
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
    </div>
  );
}
