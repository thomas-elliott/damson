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
} from "@/design-system";
import { AssetPicker } from "@/components/AssetPicker";
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
  const [sourceId, setSourceId] = useState<number | null>(null);
  const [result, setResult] = useState<MergeResponse | null>(null);
  const [adoptName, setAdoptName] = useState(true);
  const [adoptDescription, setAdoptDescription] = useState(true);

  const merge = useMergeAssets();
  const patch = usePatchAsset();

  const reset = () => {
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
      <DialogContent>
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
                <p className="text-label text-subtle-foreground">
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
                      <span className="text-label text-muted-foreground">
                        Name: <span className="text-foreground">{result.adoptable.name}</span>
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
                      <span className="text-label text-muted-foreground">
                        Description:{" "}
                        <span className="text-foreground">{result.adoptable.description}</span>
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
              <AssetPicker
                aria-label="Asset to merge in"
                placeholder="Search assets to merge in"
                selectedId={sourceId}
                onSelect={(a) => setSourceId(a.id)}
                excludeId={targetId}
              />
              {merge.error && <p className="text-label text-destructive">{merge.error.message}</p>}
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
