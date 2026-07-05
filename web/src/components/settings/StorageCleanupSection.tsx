import { useState } from "react";

import { Badge, Button, Checkbox, EmptyState, Section, formatBytes } from "@/design-system";
import { useConfirm } from "@/design-system/compositions/ConfirmDialog";
import {
  useStorageCleanupDelete,
  useStorageCleanupScan,
  type StorageCleanupCandidate,
} from "@/hooks/useMutations";

const candId = (c: { prefix: string; key: string }) => `${c.prefix}|${c.key}`;

/**
 * Sysadmin-only manual orphan sweep. Scan lists objects in the store that no
 * live DB row references (and that clear the safety age window); the operator
 * reviews them — with any object metadata — and deletes a selected subset.
 * Deletion is re-verified server-side, so a race between scan and delete is
 * skipped rather than acted on.
 */
export function StorageCleanupSection() {
  const scan = useStorageCleanupScan();
  const del = useStorageCleanupDelete();
  const { confirm, dialog } = useConfirm();
  const [candidates, setCandidates] = useState<StorageCleanupCandidate[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [scanned, setScanned] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const runScan = () => {
    setNotice(null);
    scan.mutate(undefined, {
      onSuccess: (data) => {
        setCandidates(data.candidates);
        setSelected(new Set(data.candidates.map(candId)));
        setScanned(true);
      },
    });
  };

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const allSelected = candidates.length > 0 && selected.size === candidates.length;
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(candidates.map(candId)));

  const selectedList = candidates.filter((c) => selected.has(candId(c)));
  const selectedBytes = selectedList.reduce((sum, c) => sum + c.size, 0);
  const totalBytes = candidates.reduce((sum, c) => sum + c.size, 0);

  const runDelete = async () => {
    const keys = selectedList.map((c) => ({ prefix: c.prefix, key: c.key }));
    const ok = await confirm({
      title: `Delete ${keys.length} orphaned object${keys.length === 1 ? "" : "s"}?`,
      description: `This permanently removes ${formatBytes(
        selectedBytes,
      )} from storage. Each object is re-checked immediately before deletion; anything referenced again is skipped.`,
      confirmLabel: "Delete",
      tone: "danger",
    });
    if (!ok) return;
    del.mutate(keys, {
      onSuccess: (res) => {
        const skippedIds = new Set(res.skipped.map(candId));
        const deletedIds = new Set(keys.map(candId).filter((id) => !skippedIds.has(id)));
        setCandidates((prev) => prev.filter((c) => !deletedIds.has(candId(c))));
        setSelected((prev) => new Set([...prev].filter((id) => !deletedIds.has(id))));
        const parts = [`Deleted ${res.deleted_count}, freed ${formatBytes(res.freed_bytes)}`];
        if (res.skipped.length) parts.push(`${res.skipped.length} skipped`);
        setNotice(parts.join(" · "));
      },
    });
  };

  return (
    <Section
      title="Storage cleanup"
      defaultOpen={false}
      accessory={
        notice ? (
          <Badge size="sm" tone="ok">
            {notice}
          </Badge>
        ) : null
      }
    >
      <p className="text-caption mb-3">
        Find objects in the store no longer referenced by the database and remove them. Objects
        younger than the safety window are never listed. Review before deleting.
      </p>

      <div className="flex items-center gap-3">
        <Button variant="secondary" onClick={runScan} disabled={scan.isPending}>
          {scan.isPending ? "Scanning…" : "Scan for orphans"}
        </Button>
        {scanned && !scan.isPending && (
          <span className="text-caption">
            {candidates.length} orphan{candidates.length === 1 ? "" : "s"} ·{" "}
            {formatBytes(totalBytes)}
          </span>
        )}
      </div>

      {scan.error && <div className="text-caption mt-2 text-err">Scan failed. Try again.</div>}

      {scanned && candidates.length === 0 && !scan.isPending && (
        <div className="mt-3">
          <EmptyState title="No orphaned objects" />
        </div>
      )}

      {candidates.length > 0 && (
        <div className="mt-3 flex flex-col">
          <div className="text-caption flex items-center gap-3 border-b border-line py-2">
            <Checkbox aria-label="Select all" checked={allSelected} onCheckedChange={toggleAll} />
            <span className="flex-1">Object</span>
            <span className="w-24 text-right">Size</span>
            <span className="w-28 text-right">Modified</span>
          </div>
          {candidates.map((c) => {
            const id = candId(c);
            const hasMeta = c.filename || c.library_id || c.asset_id || c.version_id || c.kind;
            return (
              <div
                key={id}
                className="flex items-center gap-3 border-b border-line py-2 last:border-b-0"
              >
                <Checkbox
                  aria-label={`Select ${c.key}`}
                  checked={selected.has(id)}
                  onCheckedChange={() => toggle(id)}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <Badge size="sm" tone="neutral">
                      {c.prefix}
                    </Badge>
                    <span className="text-caption truncate font-mono text-text-2">{c.key}</span>
                  </div>
                  {hasMeta && (
                    <div className="text-caption truncate text-text-3">
                      {c.filename ?? "—"}
                      {c.library_id ? ` · lib ${c.library_id}` : ""}
                      {c.asset_id ? ` · asset ${c.asset_id}` : ""}
                      {c.version_id ? ` · v${c.version_id}` : ""}
                      {c.kind ? ` · ${c.kind}` : ""}
                    </div>
                  )}
                </div>
                <span className="text-caption w-24 text-right font-mono">
                  {formatBytes(c.size)}
                </span>
                <span className="text-caption w-28 text-right text-text-3">
                  {new Date(c.last_modified).toLocaleDateString()}
                </span>
              </div>
            );
          })}
          <div className="mt-3 flex items-center justify-between">
            <span className="text-caption">
              {selected.size} selected · {formatBytes(selectedBytes)}
            </span>
            <Button
              variant="danger"
              onClick={runDelete}
              disabled={selected.size === 0 || del.isPending}
            >
              {del.isPending ? "Deleting…" : "Delete selected"}
            </Button>
          </div>
        </div>
      )}
      {dialog}
    </Section>
  );
}
