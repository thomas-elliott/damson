import { useState } from "react";
import { Download, ExternalLink } from "lucide-react";

import { Badge, Button, formatBytes, Input, Pill, Section, Tag } from "@/design-system";
import { downloadFile } from "@/components/filebrowser/useFileObjectUrl";
import type { AssetDetailView } from "@/hooks/useDataQueries";
import { useAddAssetTag, useRemoveAssetTag } from "@/hooks/useMutations";

import { LocationSection } from "./LocationSection";
import { downloadSource, originalFileName, sourceLabel } from "./packageSource";

// The sections the quick-look panel and the package page share.

/**
 * The asset's status, read-only. Needs review is a status, not a button:
 * marking an asset reviewed is a decision made in the triage queue, so a
 * click here can't clear it by accident.
 */
export function AssetStatusChips({ asset }: { asset: AssetDetailView }) {
  if (!asset.needs_review && !asset.is_reference && !asset.publisher) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {asset.needs_review && (
        <Badge tone="warn" size="md">
          Needs review
        </Badge>
      )}
      {asset.is_reference && <Pill tone="neutral">Reference · not stored</Pill>}
      {asset.publisher && (
        <span className="text-label text-muted-foreground">{asset.publisher}</span>
      )}
    </div>
  );
}

/**
 * Where the asset is and how to get it, which is what opening one is for. A
 * reference asset has a path and a store link; a stored one has its original
 * archive and the listing it came from.
 */
export function GetItSection({ asset }: { asset: AssetDetailView }) {
  if (asset.is_reference) return <LocationSection asset={asset} />;
  return <StoredGetIt asset={asset} />;
}

function StoredGetIt({ asset }: { asset: AssetDetailView }) {
  const original = downloadSource(asset);
  const originalKey = original?.original_package_object_key;
  const source = asset.source_url.trim();
  const current = asset.versions.find((v) => v.is_current);
  const size = asset.files.reduce((sum, f) => sum + f.size_bytes, 0);

  return (
    <Section title="Package" defaultOpen>
      <div className="flex flex-col gap-2.5">
        <p className="text-label text-muted-foreground">
          {asset.files.length} {asset.files.length === 1 ? "file" : "files"} ·{" "}
          <span className="tabular-nums">{formatBytes(size)}</span>
          {current && (current.version_label || current.store_version) && (
            <> · version {current.version_label || current.store_version}</>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-1.5">
          {original && originalKey && (
            <Button
              size="sm"
              variant="secondary"
              leadingIcon={<Download size={12} />}
              onClick={() =>
                void downloadFile(originalKey, originalFileName(asset, original)).catch(() => {})
              }
            >
              Download original
            </Button>
          )}
          {source && (
            <Button asChild size="sm" variant="secondary" leadingIcon={<ExternalLink size={12} />}>
              <a href={source} target="_blank" rel="noreferrer noopener" title={source}>
                {sourceLabel(source)}
              </a>
            </Button>
          )}
        </div>
      </div>
    </Section>
  );
}

/** The asset's tags, removable, with a field to add one by path. */
export function TagsSection({ asset }: { asset: AssetDetailView }) {
  const addTag = useAddAssetTag();
  const removeTag = useRemoveAssetTag();
  const [newTag, setNewTag] = useState("");

  const onAddTag = () => {
    const v = newTag.trim();
    if (!v) return;
    addTag.mutate({ assetId: asset.id, path: v }, { onSuccess: () => setNewTag("") });
  };

  return (
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
          // Tags are store categories, two to four levels deep.
          placeholder="3d/environment/trees"
          aria-label="New tag path"
          className="h-7 w-[200px] text-label"
        />
      </div>
    </Section>
  );
}
