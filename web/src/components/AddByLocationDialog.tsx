import { useEffect, useState } from "react";
import { MapPin } from "lucide-react";

import { ApiError } from "@/api";
import { ProxiedImage } from "@/components/detail/MetadataActions";
import {
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  FieldRow,
  Input,
} from "@/design-system";
import {
  usePreviewMetadata,
  useRecordReference,
  type ReferenceRecordResult,
} from "@/hooks/useMutations";

// How long the store link must sit still before it is looked up, so typing a
// URL doesn't fire a lookup per keystroke.
const LOOKUP_DELAY_MS = 500;

function isHttpURL(s: string): boolean {
  try {
    const u = new URL(s);
    return (u.protocol === "http:" || u.protocol === "https:") && u.host !== "";
  } catch {
    return false;
  }
}

const RESULT_TITLE: Record<ReferenceRecordResult["outcome"], string> = {
  created: "Added to your library",
  updated: "Updated — that path was already recorded",
  exists: "Already in your library",
};

/**
 * AddByLocationButton opens the dialog that records an asset by where it lives
 * (a path on the NFS store and/or a store link) without importing its data.
 */
export function AddByLocationButton({ onOpenAsset }: { onOpenAsset: (id: number) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="secondary" leadingIcon={<MapPin size={12} />} onClick={() => setOpen(true)}>
        Add by location
      </Button>
      {open && (
        <AddByLocationDialog
          onOpenChange={setOpen}
          onOpenAsset={(id) => {
            setOpen(false);
            onOpenAsset(id);
          }}
        />
      )}
    </>
  );
}

export function AddByLocationDialog({
  onOpenChange,
  onOpenAsset,
}: {
  onOpenChange: (open: boolean) => void;
  onOpenAsset: (id: number) => void;
}) {
  const record = useRecordReference();
  const preview = usePreviewMetadata();
  const [path, setPath] = useState("");
  const [link, setLink] = useState("");
  const [name, setName] = useState("");
  const [needsName, setNeedsName] = useState(false);
  const [result, setResult] = useState<ReferenceRecordResult | null>(null);

  const trimmedLink = link.trim();
  const lookupURL = isHttpURL(trimmedLink) ? trimmedLink : "";
  const { mutate: lookup, reset: resetLookup } = preview;

  // The store match shows before adding: look the link up once it settles.
  useEffect(() => {
    resetLookup();
    if (!lookupURL) return;
    const t = setTimeout(() => lookup(lookupURL), LOOKUP_DELAY_MS);
    return () => clearTimeout(t);
  }, [lookupURL, lookup, resetLookup]);

  const canSubmit =
    (path.trim() !== "" || trimmedLink !== "") && (!needsName || name.trim() !== "");

  const submit = () => {
    if (!canSubmit) return;
    record.mutate(
      {
        location_path: path.trim() || undefined,
        source_url: trimmedLink || undefined,
        name: name.trim() || undefined,
      },
      {
        onSuccess: setResult,
        onError: (err) => {
          if (err instanceof ApiError && err.status === 422) setNeedsName(true);
        },
      },
    );
  };

  const error = record.error;
  const showError = error && !(error instanceof ApiError && error.status === 422);

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add by location</DialogTitle>
          <DialogDescription>
            Record an asset by where it lives. Nothing is imported, and Damson never opens the path.
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          {result ? (
            <div className="flex flex-col gap-1" role="status">
              <p className="text-ui font-medium text-foreground">{RESULT_TITLE[result.outcome]}</p>
              <p className="text-label text-muted-foreground">{result.name}</p>
            </div>
          ) : (
            <form
              className="flex flex-col gap-2.5"
              onSubmit={(e) => {
                e.preventDefault();
                submit();
              }}
            >
              <FieldRow label="Path">
                <Input
                  value={path}
                  onChange={(e) => {
                    setPath(e.currentTarget.value);
                    // The server decides afresh whether a name is needed.
                    setNeedsName(false);
                  }}
                  placeholder="/mnt/assets/Publisher/Pack.unitypackage"
                  aria-label="Path"
                  className="font-mono"
                  autoFocus
                />
              </FieldRow>
              <FieldRow label="Store link">
                <Input
                  value={link}
                  onChange={(e) => {
                    setLink(e.currentTarget.value);
                    setNeedsName(false);
                  }}
                  placeholder="https://assetstore.unity.com/packages/…"
                  aria-label="Store link"
                />
              </FieldRow>
              <StoreMatch
                pending={preview.isPending}
                failed={preview.isError}
                title={preview.data?.title}
                publisher={preview.data?.publisher}
                cover={preview.data?.cover_url}
              />
              {needsName && (
                <>
                  <FieldRow label="Name">
                    <Input
                      value={name}
                      onChange={(e) => setName(e.currentTarget.value)}
                      aria-label="Name"
                      autoFocus
                    />
                  </FieldRow>
                  <p className="text-label text-subtle-foreground">
                    Neither the path nor the store link gives a name, so name it here.
                  </p>
                </>
              )}
              {showError && <p className="text-label text-destructive">{error.message}</p>}
              {/* Lets Enter submit from any field. */}
              <button type="submit" hidden />
            </form>
          )}
        </DialogBody>
        <DialogFooter>
          {result ? (
            <>
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Done
              </Button>
              <Button variant="primary" onClick={() => onOpenAsset(result.asset_id)}>
                Open asset
              </Button>
            </>
          ) : (
            <>
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button variant="primary" onClick={submit} disabled={!canSubmit || record.isPending}>
                {record.isPending ? "Adding…" : "Add"}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function StoreMatch({
  pending,
  failed,
  title,
  publisher,
  cover,
}: {
  pending: boolean;
  failed: boolean;
  title?: string;
  publisher?: string;
  cover?: string;
}) {
  if (pending) {
    return <p className="text-label text-subtle-foreground">Looking up the store link…</p>;
  }
  if (failed) {
    return (
      <p className="text-label text-subtle-foreground">
        Couldn&rsquo;t read that link. It will still be recorded.
      </p>
    );
  }
  if (!title && !cover) return null;
  return (
    <div
      className="flex items-center gap-2 rounded-md border border-border bg-muted p-2"
      aria-label="Store match"
    >
      {cover && <ProxiedImage src={cover} className="h-12 w-12 shrink-0 rounded object-cover" />}
      <div className="min-w-0">
        {title && <div className="truncate text-ui font-medium text-foreground">{title}</div>}
        {publisher && <div className="truncate text-label text-muted-foreground">{publisher}</div>}
      </div>
    </div>
  );
}
