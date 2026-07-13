import { useState } from "react";
import { Download, Search } from "lucide-react";

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
  Input,
} from "@/design-system";
import type { AssetDetailView, MetadataCandidate, MetadataConflict } from "@/hooks/useDataQueries";
import { useApplyMetadata, useFetchMetadata, useSearchMetadata } from "@/hooks/useMutations";

function truncate(s: string, n = 80): string {
  const t = s.trim();
  return t.length > n ? `${t.slice(0, n)}…` : t;
}

function candidateSubtitle(c: MetadataCandidate): string {
  const price = c.price ? `${c.currency ? `${c.currency} ` : ""}${c.price}` : "";
  return [c.store, c.publisher, price].filter(Boolean).join(" · ");
}

// MetadataActions renders the "Fetch" (from source URL) and "Find listing"
// (name search) actions, the match-picker dialog, and the overwrite-confirm
// dialog for conflicting fields.
export function MetadataActions({ asset }: { asset: AssetDetailView }) {
  const fetchMeta = useFetchMetadata();
  const applyMeta = useApplyMetadata();
  const search = useSearchMetadata();

  const [pickerOpen, setPickerOpen] = useState(false);
  const [conflicts, setConflicts] = useState<MetadataConflict[] | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [note, setNote] = useState<string | null>(null);

  const runFetch = (url?: string) => {
    setNote(null);
    fetchMeta.mutate(
      { id: asset.id, url },
      {
        onSuccess: (res) => {
          if (res.conflicts.length > 0) {
            setConflicts(res.conflicts);
            setSelected(Object.fromEntries(res.conflicts.map((c) => [c.field, true])));
          } else {
            setNote(
              res.applied.length > 0
                ? `Filled: ${res.applied.join(", ")}`
                : "Fetched — nothing new to fill",
            );
          }
        },
      },
    );
  };

  const onApply = () => {
    const fields = Object.entries(selected)
      .filter(([, v]) => v)
      .map(([k]) => k);
    if (fields.length === 0) {
      setConflicts(null);
      return;
    }
    applyMeta.mutate(
      { id: asset.id, fields },
      {
        onSuccess: (res) => {
          setConflicts(null);
          setNote(`Applied: ${res.applied.join(", ")}`);
        },
      },
    );
  };

  const hasSource = asset.source_url.trim() !== "";

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-center gap-1.5">
        <Button
          size="sm"
          onClick={() => runFetch()}
          disabled={!hasSource || fetchMeta.isPending}
          title={hasSource ? "Fetch metadata from the source URL" : "Set a source URL first"}
        >
          <Download size={12} /> Fetch
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => {
            setPickerOpen(true);
            search.mutate(asset.name);
          }}
        >
          <Search size={12} /> Find listing
        </Button>
        {fetchMeta.isError && <span className="text-mono text-err">Fetch failed</span>}
        {note && <span className="text-mono text-text-3">{note}</span>}
      </div>

      <MatchPickerDialog
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        assetName={asset.name}
        search={search}
        onPick={(url) => {
          setPickerOpen(false);
          runFetch(url);
        }}
      />

      <ConflictDialog
        conflicts={conflicts}
        selected={selected}
        setSelected={setSelected}
        onClose={() => setConflicts(null)}
        onApply={onApply}
        pending={applyMeta.isPending}
      />
    </div>
  );
}

function MatchPickerDialog({
  open,
  onOpenChange,
  assetName,
  search,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  assetName: string;
  search: ReturnType<typeof useSearchMetadata>;
  onPick: (url: string) => void;
}) {
  const [q, setQ] = useState(assetName);

  const candidates = search.data?.candidates ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={560}>
        <DialogHeader>
          <DialogTitle>Find listing</DialogTitle>
          <DialogDescription>Search storefronts and pick the matching listing.</DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div className="flex gap-1.5">
            <Input
              value={q}
              onChange={(e) => setQ(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") search.mutate(q);
              }}
              placeholder="Asset name"
              autoFocus
            />
            <Button onClick={() => search.mutate(q)} disabled={search.isPending}>
              Search
            </Button>
          </div>
          {search.isPending && <p className="text-caption text-text-3">Searching…</p>}
          {search.isError && <p className="text-caption text-err">Search failed</p>}
          {!search.isPending && search.data && candidates.length === 0 && (
            <p className="text-caption text-text-3">No matches found.</p>
          )}
          <ul className="flex flex-col gap-1">
            {candidates.map((c, i) => (
              <li key={`${c.listing_url}-${i}`}>
                <button
                  type="button"
                  onClick={() => onPick(c.listing_url)}
                  className="flex w-full items-center gap-2 rounded-md border border-line bg-surface p-2 text-left transition-colors duration-std ease hover:border-line-2 hover:bg-surface-2"
                >
                  {c.cover_url ? (
                    <img
                      src={c.cover_url}
                      alt=""
                      className="h-10 w-10 shrink-0 rounded object-cover"
                      loading="lazy"
                    />
                  ) : (
                    <div className="h-10 w-10 shrink-0 rounded bg-bg" />
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-ui text-text">{c.title}</div>
                    <div className="truncate text-mono text-text-3">{candidateSubtitle(c)}</div>
                  </div>
                </button>
              </li>
            ))}
          </ul>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

function ConflictDialog({
  conflicts,
  selected,
  setSelected,
  onClose,
  onApply,
  pending,
}: {
  conflicts: MetadataConflict[] | null;
  selected: Record<string, boolean>;
  setSelected: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
  onClose: () => void;
  onApply: () => void;
  pending: boolean;
}) {
  return (
    <Dialog open={conflicts !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent width={520}>
        <DialogHeader>
          <DialogTitle>Confirm overwrites</DialogTitle>
          <DialogDescription>
            These fields already have values. Choose which the fetched data should replace.
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          {(conflicts ?? []).map((c) => (
            <label key={c.field} className="flex items-start gap-2">
              <Checkbox
                checked={!!selected[c.field]}
                onCheckedChange={(v) => setSelected((s) => ({ ...s, [c.field]: v === true }))}
                aria-label={`Overwrite ${c.field}`}
                className="mt-0.5"
              />
              <div className="min-w-0 flex-1">
                <div className="text-ui capitalize text-text">{c.field}</div>
                <div className="text-mono text-text-3">
                  current: <span className="text-text-2">{truncate(c.current) || "—"}</span>
                </div>
                <div className="text-mono text-text-3">
                  new: <span className="text-text-2">{truncate(c.proposed)}</span>
                </div>
              </div>
            </label>
          ))}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={onApply} disabled={pending}>
            Apply selected
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
