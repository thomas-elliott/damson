import { useState } from "react";
import { ChevronDown, Download, Search } from "lucide-react";

import {
  Button,
  Checkbox,
  cn,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  IconButton,
  Input,
  LoadingState,
} from "@/design-system";
import { useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";
import type { AssetDetailView, MetadataCandidate, MetadataConflict } from "@/hooks/useDataQueries";
import {
  useApplyMetadata,
  useFetchMetadata,
  usePreviewMetadata,
  useSearchMetadata,
} from "@/hooks/useMutations";

function truncate(s: string, n = 80): string {
  const t = s.trim();
  return t.length > n ? `${t.slice(0, n)}…` : t;
}

function candidateSubtitle(c: MetadataCandidate): string {
  const price = c.price ? `${c.currency ? `${c.currency} ` : ""}${c.price}` : "";
  return [c.store, c.publisher, price].filter(Boolean).join(" · ");
}

// ProxiedImage renders a remote listing image through the SSRF-guarded proxy,
// since the CSP forbids store CDN origins.
export function ProxiedImage({ src, className }: { src: string; className?: string }) {
  const url = useBlobUrl(`/api/metadata/image?url=${encodeURIComponent(src)}`);
  if (!url) return <div className={cn("animate-pulse bg-background", className)} />;
  return <img src={url} alt="" loading="lazy" className={className} />;
}

function CandidateRow({ c, onPick }: { c: MetadataCandidate; onPick: () => void }) {
  const [expanded, setExpanded] = useState(false);
  const preview = usePreviewMetadata();

  const toggle = () => {
    const next = !expanded;
    setExpanded(next);
    if (next && !preview.data && !preview.isPending) {
      preview.mutate(c.listing_url);
    }
  };

  return (
    <div className="rounded-md border border-border bg-muted transition-colors duration-fast ease-standard hover:border-border-strong">
      <div className="flex items-center gap-2 p-2">
        {c.cover_url ? (
          <ProxiedImage src={c.cover_url} className="h-10 w-10 shrink-0 rounded object-cover" />
        ) : (
          <div className="h-10 w-10 shrink-0 rounded bg-background" />
        )}
        <button type="button" onClick={onPick} className="min-w-0 flex-1 text-left">
          <div className="truncate text-ui text-foreground">{c.title}</div>
          <div className="truncate text-mono text-subtle-foreground">{candidateSubtitle(c)}</div>
        </button>
        <IconButton
          aria-label={expanded ? "Collapse details" : "Expand details"}
          size="sm"
          onClick={toggle}
        >
          <ChevronDown
            className={`transition-transform duration-fast ease-standard ${expanded ? "rotate-180" : ""}`}
          />
        </IconButton>
      </div>
      {expanded && (
        <div className="flex flex-col gap-1.5 border-t border-border p-2">
          {preview.isPending && <LoadingState variant="inline" />}
          {preview.isError && (
            <p className="text-caption text-destructive">Couldn't load details</p>
          )}
          {preview.data && (
            <>
              {preview.data.description && (
                <p className="max-h-32 overflow-y-auto whitespace-pre-line text-caption text-muted-foreground">
                  {preview.data.description}
                </p>
              )}
              {(preview.data.image_urls?.length ?? 0) > 0 && (
                <div className="flex gap-1.5 overflow-x-auto pb-1">
                  {preview.data.image_urls!.slice(0, 8).map((src) => (
                    <ProxiedImage
                      key={src}
                      src={src}
                      className="h-16 w-24 shrink-0 rounded-md border border-border object-cover"
                    />
                  ))}
                </div>
              )}
              <Button variant="secondary" size="sm" onClick={onPick}>
                Use this listing
              </Button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

// useMetadataFetch owns the fetch → conflict → apply cycle for one asset and
// returns the confirm dialog as an element for the caller to render.
function useMetadataFetch(assetId: number) {
  const fetchMeta = useFetchMetadata();
  const applyMeta = useApplyMetadata();
  const [conflicts, setConflicts] = useState<MetadataConflict[] | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [note, setNote] = useState<string | null>(null);

  const runFetch = (url?: string) => {
    setNote(null);
    fetchMeta.mutate(
      { id: assetId, url },
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
      { id: assetId, fields },
      {
        onSuccess: (res) => {
          setConflicts(null);
          setNote(`Applied: ${res.applied.join(", ")}`);
        },
      },
    );
  };

  return {
    runFetch,
    note,
    pending: fetchMeta.isPending,
    failed: fetchMeta.isError,
    dialog: (
      <ConflictDialog
        conflicts={conflicts}
        selected={selected}
        setSelected={setSelected}
        onClose={() => setConflicts(null)}
        onApply={onApply}
        pending={applyMeta.isPending}
      />
    ),
  };
}

// FindListingButton is the standalone name-search entry point — picker plus
// fetch, for callers with no room for the full panel (the import queue card).
export function FindListingButton({
  assetId,
  assetName,
  label = "Find listing",
}: {
  assetId: number;
  assetName: string;
  label?: string;
}) {
  const search = useSearchMetadata();
  const meta = useMetadataFetch(assetId);
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        size="sm"
        variant="secondary"
        disabled={meta.pending}
        onClick={() => {
          setOpen(true);
          search.mutate(assetName);
        }}
      >
        <Search size={12} /> {meta.pending ? "Fetching…" : label}
      </Button>
      <MatchPickerDialog
        open={open}
        onOpenChange={setOpen}
        assetName={assetName}
        search={search}
        onPick={(url) => {
          setOpen(false);
          meta.runFetch(url);
        }}
      />
      {meta.dialog}
    </>
  );
}

// MetadataActions renders the "Fetch" (from source URL) and "Find listing"
// (name search) actions, the match-picker dialog, and the overwrite-confirm
// dialog for conflicting fields. Right-aligned so it can sit opposite the
// source-URL link on one line.
export function MetadataActions({ asset }: { asset: AssetDetailView }) {
  const search = useSearchMetadata();
  const meta = useMetadataFetch(asset.id);
  const [pickerOpen, setPickerOpen] = useState(false);

  const hasSource = asset.source_url.trim() !== "";

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-1.5">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => meta.runFetch()}
          disabled={!hasSource || meta.pending}
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
      </div>
      {meta.failed && <span className="text-mono text-destructive">Fetch failed</span>}
      {meta.note && (
        <span className="text-right text-mono text-subtle-foreground">{meta.note}</span>
      )}

      <MatchPickerDialog
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        assetName={asset.name}
        search={search}
        onPick={(url) => {
          setPickerOpen(false);
          meta.runFetch(url);
        }}
      />

      {meta.dialog}
    </div>
  );
}

// EnrichmentReview surfaces the persisted enrichment provenance (which store,
// when, matched vs abstained) and any stored candidates for one-click
// correction when auto-enrich picked wrong or abstained. Provenance is
// housekeeping rather than content, so the panel only shows it in edit mode.
export function EnrichmentReview({ asset }: { asset: AssetDetailView }) {
  const meta = useMetadataFetch(asset.id);
  const sm = asset.source_metadata;
  if (!sm) return null;

  const when = sm.fetched_at ? new Date(sm.fetched_at) : null;
  const whenLabel = when && !Number.isNaN(when.getTime()) ? when.toLocaleDateString() : null;
  const candidates = sm.candidates ?? [];

  return (
    <div className="flex flex-col gap-1.5 rounded-md border border-border bg-muted p-2">
      <div className="flex flex-wrap items-center gap-x-1.5 text-mono text-subtle-foreground">
        <span>{sm.matched ? `Metadata from ${sm.store || "source"}` : "No confident match"}</span>
        {whenLabel && <span>· {whenLabel}</span>}
        {sm.matched && sm.listing_url && (
          <>
            <span>·</span>
            <a
              href={sm.listing_url}
              target="_blank"
              rel="noreferrer"
              className="text-primary-text hover:underline"
            >
              listing
            </a>
          </>
        )}
      </div>
      {candidates.length > 0 && (
        <div className="flex flex-col gap-1">
          <div className="text-mono text-subtle-foreground">
            {sm.matched ? "Other candidates" : "Suggested listings — pick the match:"}
          </div>
          <ul className="flex flex-col gap-1">
            {candidates.map((c, i) => (
              <li key={`${c.listing_url}-${i}`}>
                <CandidateRow c={c} onPick={() => meta.runFetch(c.listing_url)} />
              </li>
            ))}
          </ul>
        </div>
      )}
      {meta.failed && <span className="text-mono text-destructive">Fetch failed</span>}
      {meta.note && <span className="text-mono text-subtle-foreground">{meta.note}</span>}
      {meta.dialog}
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
      <DialogContent>
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
            <Button
              variant="secondary"
              onClick={() => search.mutate(q)}
              disabled={search.isPending}
            >
              Search
            </Button>
          </div>
          {search.isPending && <p className="text-caption text-subtle-foreground">Searching…</p>}
          {search.isError && <p className="text-caption text-destructive">Search failed</p>}
          {!search.isPending && search.data && candidates.length === 0 && (
            <p className="text-caption text-subtle-foreground">No matches found.</p>
          )}
          <ul className="flex flex-col gap-1">
            {candidates.map((c, i) => (
              <li key={`${c.listing_url}-${i}`}>
                <CandidateRow c={c} onPick={() => onPick(c.listing_url)} />
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
      <DialogContent>
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
                <div className="text-ui capitalize text-foreground">{c.field}</div>
                <div className="text-mono text-subtle-foreground">
                  current:{" "}
                  <span className="text-muted-foreground">{truncate(c.current) || "—"}</span>
                </div>
                <div className="text-mono text-subtle-foreground">
                  new: <span className="text-muted-foreground">{truncate(c.proposed)}</span>
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
