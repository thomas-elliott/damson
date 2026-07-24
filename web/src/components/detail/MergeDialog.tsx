import { useState } from "react";

import {
  Button,
  Checkbox,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  SearchInput,
  cn,
} from "@/design-system";
import { useAssets } from "@/hooks/useDataQueries";
import { useMergeAssets, usePatchAsset, type MergeResponse } from "@/hooks/useMutations";

export function MergeDialog({
  targetId,
  targetName,
  open,
  onOpenChange,
}: {
  targetId: number;
  targetName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [q, setQ] = useState("");
  const [sourceId, setSourceId] = useState<number | null>(null);
  const [result, setResult] = useState<MergeResponse | null>(null);
  const [adoptName, setAdoptName] = useState(true);
  const [adoptDescription, setAdoptDescription] = useState(true);

  const { data: assets, loading } = useAssets({ q: q || undefined }, open);
  const merge = useMergeAssets();
  const patch = usePatchAsset();

  const candidates = assets.filter((a) => a.id !== targetId);

  const candidateRows = () => {
    if (loading) return <li className="px-2 py-3 text-small text-text-4">Searching…</li>;
    if (candidates.length === 0)
      return <li className="px-2 py-3 text-small text-text-4">No other assets</li>;
    return candidates.map((a) => (
      <li key={a.id}>
        <button
          type="button"
          onClick={() => setSourceId(a.id)}
          className={cn(
            "w-full truncate rounded-md px-2 py-1.5 text-left text-small text-text-2 transition-colors duration-std ease hover:bg-surface",
            sourceId === a.id && "bg-surface text-text",
          )}
        >
          {a.name}
        </button>
      </li>
    ));
  };

  const reset = () => {
    setQ("");
    setSourceId(null);
    setResult(null);
    setAdoptName(true);
    setAdoptDescription(true);
    merge.reset();
    patch.reset();
  };

  const onMerge = () => {
    if (!sourceId) return;
    merge.mutate({ targetId, source_asset_id: sourceId }, { onSuccess: (r) => setResult(r) });
  };

  const onAdopt = () => {
    const adoptable = result?.adoptable;
    const body: { id: number; name?: string; description?: string } = { id: targetId };
    if (adoptName && adoptable?.name) body.name = adoptable.name;
    if (adoptDescription && adoptable?.description) body.description = adoptable.description;
    if (body.name === undefined && body.description === undefined) {
      onOpenChange(false);
      return;
    }
    patch.mutate(body, { onSuccess: () => onOpenChange(false) });
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent width={480}>
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle>Merged {result.merged_versions} version(s)</DialogTitle>
              <DialogDescription>
                Keep {targetName}&rsquo;s details, or adopt any of the merged asset&rsquo;s below.
              </DialogDescription>
            </DialogHeader>
            <DialogBody>
              {!result.adoptable.name && !result.adoptable.description ? (
                <p className="text-small text-text-3">
                  The merged asset had no extra name or description to adopt.
                </p>
              ) : (
                <ul className="flex flex-col gap-2.5">
                  {result.adoptable.name && (
                    <li className="flex items-start gap-2">
                      <Checkbox
                        checked={adoptName}
                        onCheckedChange={(v) => setAdoptName(v === true)}
                        aria-label="Adopt name"
                      />
                      <span className="text-small text-text-2">
                        Name: <span className="text-text">{result.adoptable.name}</span>
                      </span>
                    </li>
                  )}
                  {result.adoptable.description && (
                    <li className="flex items-start gap-2">
                      <Checkbox
                        checked={adoptDescription}
                        onCheckedChange={(v) => setAdoptDescription(v === true)}
                        aria-label="Adopt description"
                      />
                      <span className="text-small text-text-2">
                        Description:{" "}
                        <span className="text-text">{result.adoptable.description}</span>
                      </span>
                    </li>
                  )}
                </ul>
              )}
            </DialogBody>
            <DialogFooter>
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Keep current
              </Button>
              <Button variant="primary" onClick={onAdopt} disabled={patch.isPending}>
                Done
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Merge another asset into {targetName}</DialogTitle>
              <DialogDescription>
                Its versions, tags, collections and pins move here; the other asset is removed.
              </DialogDescription>
            </DialogHeader>
            <DialogBody>
              <SearchInput
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Search assets to merge in"
                autoFocus
              />
              {merge.error && <p className="text-small text-err">{merge.error.message}</p>}
              <ul className="flex max-h-64 flex-col gap-0.5 overflow-y-auto">{candidateRows()}</ul>
            </DialogBody>
            <DialogFooter>
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={onMerge}
                disabled={sourceId === null || merge.isPending}
              >
                Merge
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
