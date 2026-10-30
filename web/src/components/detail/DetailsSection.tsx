import { useState } from "react";
import { ExternalLink, Pencil } from "lucide-react";

import { Button, FieldRow, Input, Pill, Section } from "@/design-system";
import type { AssetDetailView, MetadataAttribute } from "@/hooks/useDataQueries";
import { usePatchAsset } from "@/hooks/useMutations";
import { storeLabel } from "@/lib/stores";

// Day precision only: these dates carry a time that would render as midnight.
function formatDate(iso?: string): string | undefined {
  if (!iso) return undefined;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

function formatDateInput(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 10);
}

function formatPrice(price?: string, currency?: string): string | undefined {
  if (!price) return undefined;
  const n = Number(price);
  if (Number.isFinite(n) && n === 0) return "Free";
  return currency ? `${price} ${currency}` : price;
}

function formatRating(rating?: number, count?: number): string | undefined {
  if (rating === undefined) return undefined;
  const stars = `${rating.toFixed(2).replace(/\.?0+$/, "")} / 5`;
  return count ? `${stars} (${count.toLocaleString()})` : stars;
}

function hasAnyDetail(asset: AssetDetailView): boolean {
  return Boolean(
    asset.publisher ||
    asset.store ||
    asset.store_deprecated ||
    asset.released_at ||
    asset.store_updated_at ||
    asset.engine ||
    asset.min_engine_version ||
    asset.price ||
    asset.rating !== undefined ||
    asset.latest_store_version ||
    asset.store_product_id,
  );
}

// DetailsSection mirrors Overview's read/edit flip: the typed storefront fields
// read as a spec sheet and only become inputs on demand. Below them sit the
// store's own attributes, which are read-only — a re-fetch re-derives them, so
// there is nothing to preserve and no merge semantics to design.
export function DetailsSection({ asset }: { asset: AssetDetailView }) {
  const [editing, setEditing] = useState(false);
  const [open, setOpen] = useState(false);
  const attributes = asset.source_metadata?.metadata?.attributes ?? [];

  return (
    <Section
      title="Details"
      open={open}
      onOpenChange={setOpen}
      accessory={
        editing ? null : (
          <div className="flex items-center gap-1.5">
            {asset.update_available && asset.latest_store_version && (
              <Pill tone="accent" title="A newer version is available from the store">
                update: {asset.latest_store_version}
              </Pill>
            )}
            {!open && storeLabel(asset.store) && (
              <Pill tone="neutral">{storeLabel(asset.store)}</Pill>
            )}
            {/* Overview has its own Edit button, so this one carries a
                distinct accessible name. */}
            <Button
              size="sm"
              variant="ghost"
              aria-label="Edit details"
              leadingIcon={<Pencil size={12} />}
              onClick={() => {
                setEditing(true);
                setOpen(true);
              }}
            >
              Edit
            </Button>
          </div>
        )
      }
    >
      {editing ? (
        <DetailsForm asset={asset} onDone={() => setEditing(false)} />
      ) : (
        <div className="flex flex-col gap-2.5">
          <DetailsRead asset={asset} />
          {attributes.length > 0 && <SpecSheet attributes={attributes} />}
        </div>
      )}
    </Section>
  );
}

function DetailsRead({ asset }: { asset: AssetDetailView }) {
  if (!hasAnyDetail(asset)) {
    return (
      <p className="text-label text-faint-foreground">
        Nothing yet — fetch storefront metadata from Overview, or add these by hand.
      </p>
    );
  }
  const rows: { label: string; value?: string; href?: string }[] = [
    { label: "Publisher", value: asset.publisher, href: asset.publisher_url },
    { label: "Store", value: storeLabel(asset.store) },
    { label: "Listing", value: asset.store_deprecated ? "Deprecated on the store" : undefined },
    { label: "Released", value: formatDate(asset.released_at) },
    { label: "Store updated", value: formatDate(asset.store_updated_at) },
    { label: "Store version", value: asset.latest_store_version },
    { label: "Engine", value: asset.engine },
    { label: "Minimum version", value: asset.min_engine_version },
    { label: "Price", value: formatPrice(asset.price, asset.price_currency) },
    { label: "Rating", value: formatRating(asset.rating, asset.rating_count) },
  ].filter((r) => Boolean(r.value));

  return <DefinitionList rows={rows} />;
}

// SpecSheet renders whatever the store itself listed, in the store's own order.
function SpecSheet({ attributes }: { attributes: MetadataAttribute[] }) {
  return (
    <div className="flex flex-col gap-1.5 border-t border-border pt-2.5">
      <div className="text-label text-subtle-foreground">From the store listing</div>
      <DefinitionList rows={attributes.map((a) => ({ label: a.label, value: a.value }))} />
    </div>
  );
}

function DefinitionList({ rows }: { rows: { label: string; value?: string; href?: string }[] }) {
  return (
    <dl className="grid grid-cols-[minmax(0,9rem)_minmax(0,1fr)] gap-x-3 gap-y-1 text-label">
      {rows.map((r) => (
        <div key={r.label} className="col-span-2 grid grid-cols-subgrid">
          <dt className="min-w-0 truncate text-subtle-foreground">{r.label}</dt>
          <dd className="min-w-0 break-words whitespace-pre-line text-muted-foreground">
            {r.href ? (
              <a
                href={r.href}
                target="_blank"
                rel="noreferrer noopener"
                className="inline-flex items-center gap-1 text-primary-text hover:underline"
              >
                <span className="min-w-0 truncate">{r.value}</span>
                <ExternalLink size={10} className="shrink-0" />
              </a>
            ) : (
              r.value
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function DetailsForm({ asset, onDone }: { asset: AssetDetailView; onDone: () => void }) {
  const patch = usePatchAsset();
  const [fields, setFields] = useState({
    publisher: asset.publisher ?? "",
    publisher_url: asset.publisher_url ?? "",
    store: asset.store ?? "",
    store_product_id: asset.store_product_id ?? "",
    released_at: formatDateInput(asset.released_at),
    store_updated_at: formatDateInput(asset.store_updated_at),
    latest_store_version: asset.latest_store_version ?? "",
    engine: asset.engine ?? "",
    min_engine_version: asset.min_engine_version ?? "",
    price: asset.price ?? "",
    price_currency: asset.price_currency ?? "",
    rating: asset.rating !== undefined ? String(asset.rating) : "",
    rating_count: asset.rating_count !== undefined ? String(asset.rating_count) : "",
  });

  const set = (key: keyof typeof fields) => (value: string) =>
    setFields((prev) => ({ ...prev, [key]: value }));

  const onSave = () => patch.mutate({ id: asset.id, ...fields }, { onSuccess: onDone });

  const textFields: { key: keyof typeof fields; label: string; placeholder?: string }[] = [
    { key: "publisher", label: "Publisher" },
    { key: "publisher_url", label: "Publisher URL", placeholder: "https://…" },
    { key: "store", label: "Store", placeholder: "unity, sketchfab, itch…" },
    { key: "store_product_id", label: "Store product id" },
    { key: "released_at", label: "Released", placeholder: "YYYY-MM-DD" },
    { key: "store_updated_at", label: "Store updated", placeholder: "YYYY-MM-DD" },
    { key: "latest_store_version", label: "Store version", placeholder: "1.24" },
    { key: "engine", label: "Engine", placeholder: "unity" },
    { key: "min_engine_version", label: "Minimum version", placeholder: "2020.3" },
    { key: "price", label: "Price" },
    { key: "price_currency", label: "Currency", placeholder: "USD" },
    { key: "rating", label: "Rating", placeholder: "0–5" },
    { key: "rating_count", label: "Rating count" },
  ];

  return (
    <div className="flex flex-col gap-2.5">
      {textFields.map((f) => (
        <FieldRow key={f.key} label={f.label}>
          <Input
            value={fields[f.key]}
            placeholder={f.placeholder}
            onChange={(e) => set(f.key)(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") onSave();
            }}
          />
        </FieldRow>
      ))}
      <p className="text-label text-faint-foreground">Clear a field to remove it.</p>
      {patch.error && (
        <p className="text-label text-destructive">{(patch.error as Error).message}</p>
      )}
      <div className="flex items-center justify-end gap-1.5">
        <Button variant="ghost" size="sm" onClick={onDone} disabled={patch.isPending}>
          Cancel
        </Button>
        <Button variant="secondary" size="sm" onClick={onSave} disabled={patch.isPending}>
          {patch.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
    </div>
  );
}
