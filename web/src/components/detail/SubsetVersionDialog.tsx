import { useEffect, useMemo, useState } from "react";

import {
  Button,
  Checkbox,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  FieldRow,
  Input,
  ScrollArea,
  formatBytes,
} from "@/design-system";
import { type AssetFileView } from "@/hooks/useDataQueries";
import { useCreateSubsetVersion } from "@/hooks/useMutations";

/**
 * SubsetVersionDialog lets the user pick a subset of files from the current
 * asset version and creates a new version pointing at those same blobs (no
 * re-upload — see internal/handlers/api/subset.go).
 *
 * The picker is a flat list rather than a directory tree; file paths are
 * already relative and grouping costs more in dialog complexity than it
 * saves in scan effort. Cap the rendered list with a scroll area so very
 * large packages don't blow up the dialog height.
 */
export function SubsetVersionDialog({
  open,
  onOpenChange,
  assetId,
  versionId,
  files,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  assetId: number;
  /** Source version id — must be the current version since `files` is scoped to it. */
  versionId: number | null;
  files: AssetFileView[];
}) {
  const create = useCreateSubsetVersion();
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [label, setLabel] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");

  // Reset state every time the dialog re-opens or the source files change.
  useEffect(() => {
    if (open) {
      setSelected(new Set());
      setLabel("");
      setError(null);
      setFilter("");
    }
  }, [open, versionId]);

  const visibleFiles = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return files;
    return files.filter((f) => f.relative_path.toLowerCase().includes(q));
  }, [files, filter]);

  const allVisibleSelected =
    visibleFiles.length > 0 && visibleFiles.every((f) => selected.has(f.id));
  const toggleAllVisible = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const f of visibleFiles) next.delete(f.id);
      } else {
        for (const f of visibleFiles) next.add(f.id);
      }
      return next;
    });
  };

  const submit = () => {
    if (!versionId) return;
    if (selected.size === 0) {
      setError("Pick at least one file.");
      return;
    }
    setError(null);
    create.mutate(
      {
        assetId,
        versionId,
        file_ids: Array.from(selected),
        version_label: label.trim() || undefined,
      },
      {
        onSuccess: () => onOpenChange(false),
        onError: (e: unknown) =>
          setError(e instanceof Error ? e.message : "Failed"),
      },
    );
  };

  const totalSelectedBytes = useMemo(() => {
    let n = 0;
    for (const f of files) if (selected.has(f.id)) n += f.size_bytes;
    return n;
  }, [files, selected]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[640px]">
        <DialogHeader>
          <DialogTitle>Create subset version</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <p className="text-ui text-text-2">
            New versions reference the same content-addressed blobs — no
            re-upload, no extra storage. Pick which files belong to the new
            subset.
          </p>
          <FieldRow label="Filter">
            <Input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Path contains…"
            />
          </FieldRow>
          <div className="flex items-center justify-between gap-3 py-1 text-caption">
            <button
              type="button"
              onClick={toggleAllVisible}
              className="text-text-2 hover:underline"
            >
              {allVisibleSelected
                ? `Deselect ${visibleFiles.length}`
                : `Select all ${visibleFiles.length}`}
            </button>
            <span className="text-text-3">
              {selected.size} selected · {formatBytes(totalSelectedBytes)}
            </span>
          </div>
          <ScrollArea className="h-[280px] rounded-sm border border-line bg-panel">
            {visibleFiles.length === 0 ? (
              <div className="p-3 text-caption">No files match.</div>
            ) : (
              <ul className="flex flex-col">
                {visibleFiles.map((f) => {
                  const checked = selected.has(f.id);
                  return (
                    <li
                      key={f.id}
                      className="flex items-center gap-2 border-b border-line px-2 py-1.5 last:border-b-0 hover:bg-surface"
                    >
                      <Checkbox
                        aria-label={f.relative_path}
                        checked={checked}
                        onCheckedChange={(v) => {
                          setSelected((prev) => {
                            const next = new Set(prev);
                            if (v) next.add(f.id);
                            else next.delete(f.id);
                            return next;
                          });
                        }}
                      />
                      <span
                        className="min-w-0 flex-1 truncate font-mono text-small text-text-2"
                        title={f.relative_path}
                      >
                        {f.relative_path}
                      </span>
                      <span className="text-mono text-text-3">
                        {formatBytes(f.size_bytes)}
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
          </ScrollArea>
          <FieldRow
            label="Version label"
            hint="Optional. Auto-numbered when blank."
          >
            <Input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="v1.subset"
            />
          </FieldRow>
          {error && <p className="text-small text-err">{error}</p>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={selected.size === 0 || create.isPending || !versionId}
            onClick={submit}
          >
            {create.isPending ? "Creating…" : "Create subset version"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
