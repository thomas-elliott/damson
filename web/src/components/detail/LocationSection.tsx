import { useState } from "react";
import { Copy, ExternalLink, Pencil } from "lucide-react";

import { Button, FieldRow, Input, Section } from "@/design-system";
import type { AssetDetailView } from "@/hooks/useDataQueries";
import { usePatchAsset } from "@/hooks/useMutations";

// LocationSection is where a reference asset lives, shown in place of its files
// and versions. Damson never reads the path: it is a label to copy, and the
// store link is a link to open.
export function LocationSection({ asset }: { asset: AssetDetailView }) {
  const [editing, setEditing] = useState(false);
  return (
    <Section
      title="Location"
      defaultOpen
      accessory={
        editing ? null : (
          <Button
            size="sm"
            variant="ghost"
            aria-label="Edit location"
            leadingIcon={<Pencil size={12} />}
            onClick={() => setEditing(true)}
          >
            Edit
          </Button>
        )
      }
    >
      {editing ? (
        <LocationForm asset={asset} onDone={() => setEditing(false)} />
      ) : (
        <LocationRead path={asset.location_path ?? ""} link={asset.source_url.trim()} />
      )}
    </Section>
  );
}

function LocationRead({ path, link }: { path: string; link: string }) {
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);

  // navigator.clipboard is undefined outside a secure context, which plain
  // http on a LAN is, so a failure says what to do rather than no-op.
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(path);
      setCopied(true);
      setCopyFailed(false);
    } catch {
      setCopied(false);
      setCopyFailed(true);
    }
  };

  return (
    <div className="flex flex-col gap-2.5">
      <FieldRow label="Path">
        <div>
          {path ? (
            <div className="flex items-start justify-between gap-2">
              <span className="min-w-0 break-all pt-1 font-mono text-label text-foreground">
                {path}
              </span>
              <Button
                size="sm"
                variant="secondary"
                leadingIcon={<Copy size={12} />}
                aria-label="Copy path"
                onClick={copy}
              >
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
          ) : (
            <span className="text-label text-faint-foreground">No path</span>
          )}
          {copyFailed && (
            <p className="mt-1 text-label text-destructive">
              Couldn&rsquo;t reach the clipboard. Select the path and copy it by hand.
            </p>
          )}
        </div>
      </FieldRow>
      <FieldRow label="Store link">
        {link ? (
          <div className="flex items-start justify-between gap-2">
            <span className="min-w-0 truncate pt-1 text-label text-muted-foreground" title={link}>
              {link}
            </span>
            <Button asChild size="sm" variant="secondary" leadingIcon={<ExternalLink size={12} />}>
              <a href={link} target="_blank" rel="noreferrer noopener" aria-label="Open store link">
                Open
              </a>
            </Button>
          </div>
        ) : (
          <span className="text-label text-faint-foreground">No store link</span>
        )}
      </FieldRow>
    </div>
  );
}

function LocationForm({ asset, onDone }: { asset: AssetDetailView; onDone: () => void }) {
  const patch = usePatchAsset();
  const [path, setPath] = useState(asset.location_path ?? "");
  const [link, setLink] = useState(asset.source_url);
  const bothBlank = path.trim() === "" && link.trim() === "";

  const onSave = () => {
    if (bothBlank) return;
    patch.mutate(
      { id: asset.id, location_path: path.trim(), source_url: link.trim() },
      { onSuccess: onDone },
    );
  };

  return (
    <div className="flex flex-col gap-2.5">
      <FieldRow label="Path">
        <Input
          value={path}
          onChange={(e) => setPath(e.currentTarget.value)}
          placeholder="/mnt/assets/Publisher/Pack"
          aria-label="Path"
          className="font-mono"
        />
      </FieldRow>
      <FieldRow label="Store link">
        <Input
          value={link}
          onChange={(e) => setLink(e.currentTarget.value)}
          placeholder="https://…"
          aria-label="Store link"
        />
      </FieldRow>
      {bothBlank && (
        <p className="text-label text-subtle-foreground">
          A reference asset keeps a path or a store link.
        </p>
      )}
      {patch.error && <p className="text-label text-destructive">{patch.error.message}</p>}
      <div className="flex items-center justify-end gap-1.5">
        <Button variant="ghost" size="sm" onClick={onDone} disabled={patch.isPending}>
          Cancel
        </Button>
        <Button
          variant="primary"
          size="sm"
          onClick={onSave}
          disabled={patch.isPending || bothBlank}
        >
          {patch.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
    </div>
  );
}
