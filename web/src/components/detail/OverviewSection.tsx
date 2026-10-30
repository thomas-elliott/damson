import { useEffect, useRef, useState } from "react";
import { ExternalLink, Pencil } from "lucide-react";

import {
  Button,
  FieldRow,
  Input,
  LicencePill,
  Markdown,
  MarkdownEditor,
  Section,
} from "@/design-system";
import type { AssetDetailView, LicenceView } from "@/hooks/useDataQueries";
import { usePatchAsset } from "@/hooks/useMutations";

import { MetadataActions } from "./MetadataActions";
import { sourceLabel } from "./packageSource";

// OverviewSection reads as rendered prose and flips to a form on demand: the
// description is markdown (storefront enrichment writes it that way), so a
// permanent textarea showed raw syntax for a field most users only ever read.
// It always opens reading, even with no description: an empty form first was
// what an asset opened into, ahead of the path and link people came for.
export function OverviewSection({ asset }: { asset: AssetDetailView }) {
  const [editing, setEditing] = useState(false);
  const [open, setOpen] = useState(true);

  const startEditing = () => {
    setEditing(true);
    setOpen(true);
  };

  return (
    <Section
      title="Overview"
      open={open}
      onOpenChange={setOpen}
      accessory={
        !editing ? (
          <Button
            size="sm"
            variant="ghost"
            leadingIcon={<Pencil size={12} />}
            onClick={startEditing}
          >
            Edit
          </Button>
        ) : null
      }
    >
      <div className="flex flex-col gap-2.5">
        {!editing ? (
          <OverviewRead asset={asset} />
        ) : (
          // Mounted on the flip into edit mode, so its fields seed from
          // whatever the server last returned — including a metadata fetch
          // that landed while the panel sat in read mode.
          <OverviewForm asset={asset} onDone={() => setEditing(false)} />
        )}
      </div>
    </Section>
  );
}

// The collapsed Licence header carries the identifier, so the one fact people
// scan for doesn't need the section opened. Expanded, LicenceEditor shows the
// same pill, so the accessory drops away.
export function LicenceSummary({ licence }: { licence: LicenceView | null }) {
  if (!licence) return <span className="text-label text-subtle-foreground">Not set</span>;
  return <LicencePill status={licence.status} spdx={licence.detected_type || "Unknown"} />;
}

function OverviewRead({ asset }: { asset: AssetDetailView }) {
  const source = asset.source_url.trim();
  return (
    <>
      {asset.description.trim() ? (
        <ClampedDescription>{asset.description}</ClampedDescription>
      ) : (
        <p className="text-label text-subtle-foreground">No description yet.</p>
      )}
      {/* Source link and the fetch actions share one line — they're all about
          where this asset's metadata comes from. */}
      <div className="flex items-start justify-between gap-2">
        {source ? (
          <a
            href={source}
            target="_blank"
            rel="noreferrer noopener"
            title={source}
            className="inline-flex min-w-0 items-center gap-1 pt-1 text-label text-primary-text hover:underline"
          >
            <span className="truncate">{sourceLabel(source)}</span>
            <ExternalLink size={11} className="shrink-0" />
          </a>
        ) : (
          <span className="pt-1 text-label text-subtle-foreground">No source URL</span>
        )}
        <MetadataActions asset={asset} />
      </div>
    </>
  );
}

// Store descriptions run to hundreds of lines, which buried every section below
// Overview. Collapsed height is measured in ems of the body text so the clamp
// stays five lines however the type scale moves; the toggle only appears when
// the content actually exceeds it (measured on the content, not the clamped
// box, so it stays correct while expanded).
const CLAMP_LINES = 5;
const CLAMP_LINE_HEIGHT = 1.4;
const CLAMP_EM = `${CLAMP_LINES * CLAMP_LINE_HEIGHT}em`;

export function ClampedDescription({ children }: { children: string }) {
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);
  const content = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const el = content.current;
    if (!el) return;
    const measure = () => {
      const limit = parseFloat(getComputedStyle(el).fontSize) * CLAMP_LINES * CLAMP_LINE_HEIGHT;
      setOverflows(el.scrollHeight > limit + 1);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [children]);

  return (
    <div className="flex flex-col gap-1">
      <div
        className="relative overflow-hidden"
        style={expanded ? undefined : { maxHeight: CLAMP_EM }}
      >
        <Markdown ref={content}>{children}</Markdown>
        {!expanded && overflows && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-6 bg-gradient-to-t from-card to-transparent" />
        )}
      </div>
      {overflows && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="self-start text-label text-primary-text hover:underline"
        >
          {expanded ? "Show less" : "Read more"}
        </button>
      )}
    </div>
  );
}

function OverviewForm({ asset, onDone }: { asset: AssetDetailView; onDone: () => void }) {
  const patch = usePatchAsset();
  const [name, setName] = useState(asset.name);
  const [description, setDescription] = useState(asset.description);
  const [sourceURL, setSourceURL] = useState(asset.source_url);

  const onSave = () => {
    patch.mutate(
      {
        id: asset.id,
        name: name.trim() || asset.name,
        description,
        ...(asset.is_reference ? {} : { source_url: sourceURL.trim() }),
      },
      { onSuccess: onDone },
    );
  };

  return (
    <div className="flex flex-col gap-2.5">
      <FieldRow label="Name">
        <Input
          value={name}
          onChange={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") onSave();
          }}
        />
      </FieldRow>
      <FieldRow label="Description">
        <MarkdownEditor
          value={description}
          onChange={setDescription}
          aria-label="Description"
          placeholder="What's this asset for? Markdown is supported."
          maxHeight={320}
          autoFocus
        />
      </FieldRow>
      {/* A reference asset's store link is edited in Location, which keeps it
          and the path from both being cleared. */}
      {!asset.is_reference && (
        <FieldRow label="Source URL">
          <Input
            value={sourceURL}
            onChange={(e) => setSourceURL(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") onSave();
            }}
            placeholder="https://…"
          />
        </FieldRow>
      )}
      <div className="flex items-center justify-end gap-1.5">
        <Button variant="ghost" size="sm" onClick={onDone} disabled={patch.isPending}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" onClick={onSave} disabled={patch.isPending}>
          {patch.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
    </div>
  );
}
