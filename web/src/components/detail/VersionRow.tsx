import { useState } from "react";
import { Check, Pencil, Trash2, X } from "lucide-react";

import {
  ErrorState,
  formatBytes,
  IconButton,
  Input,
  LoadingState,
  useConfirm,
  VersionItem,
} from "@/design-system";
import { AuditList } from "@/components/AuditList";
import { useAssetAudit, type AssetVersionView } from "@/hooks/useDataQueries";
import { useDeleteVersion, usePatchVersion } from "@/hooks/useMutations";

export function VersionRow({ assetId, version }: { assetId: number; version: AssetVersionView }) {
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

export function AssetAuditSection({ assetId }: { assetId: number }) {
  const { data: entries, loading, failed, reload } = useAssetAudit(assetId);
  if (loading) {
    return <LoadingState variant="inline" />;
  }
  if (failed) {
    return <ErrorState title="Couldn't load recent activity" onRetry={reload} />;
  }
  return (
    <AuditList
      entries={entries.slice(0, 25)}
      emptyTitle="No activity yet"
      emptyDescription="Edits to this asset will show up here."
    />
  );
}
