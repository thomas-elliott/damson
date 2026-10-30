import { useState } from "react";
import { Link } from "react-router";
import { GitMerge, RefreshCw, RotateCw, Trash2, Wrench } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Section,
  TypesetCover,
  useConfirm,
} from "@/design-system";
import type { AssetDetailView } from "@/hooks/useDataQueries";
import {
  useDeleteAsset,
  useReExtractVersion,
  useRemoveAssetFromCollection,
  useRequeueAssetPreviews,
  useRescanAsset,
  useReviewLicence,
} from "@/hooks/useMutations";

import { AssetGallery } from "./AssetGallery";
import { AssetStatusChips, GetItSection, TagsSection } from "./AssetSections";
import { DetailsSection } from "./DetailsSection";
import { LicenceEditor } from "./LicenceEditor";
import { MergeDialog } from "./MergeDialog";
import { StoreSuggestions } from "./MetadataActions";
import { LicenceSummary, OverviewSection } from "./OverviewSection";
import { reExtractSource } from "./packageSource";
import { AssetAuditSection, VersionRow } from "./VersionRow";

const linkClass = "truncate text-primary-text hover:underline";

/**
 * Everything about one asset except its files: the package page's left column,
 * or the whole page for a reference asset, which has no files.
 */
export function PackageSummary({
  asset,
  onDeleted,
}: {
  asset: AssetDetailView;
  onDeleted: () => void;
}) {
  const del = useDeleteAsset();
  const removeFromCollection = useRemoveAssetFromCollection();
  const reviewLicence = useReviewLicence();
  const [licenceOpen, setLicenceOpen] = useState(false);
  const [mergeOpen, setMergeOpen] = useState(false);
  const { confirm, dialog: confirmDialog } = useConfirm();

  return (
    <div className="@container flex flex-col gap-3 p-3">
      <PackageCover asset={asset} />
      <AssetStatusChips asset={asset} />
      <GetItSection asset={asset} />
      <StoreSuggestions asset={asset} />
      <OverviewSection asset={asset} />
      <DetailsSection asset={asset} />
      <TagsSection asset={asset} />

      {!asset.is_reference && (
        <Section title="Versions" accessory={`${asset.versions.length}`} defaultOpen>
          <div className="flex flex-col gap-0.5">
            {asset.versions.map((v) => (
              <VersionRow key={v.id} assetId={asset.id} version={v} />
            ))}
          </div>
        </Section>
      )}

      <Section
        title="Licence"
        open={licenceOpen}
        onOpenChange={setLicenceOpen}
        accessory={licenceOpen ? null : <LicenceSummary licence={asset.licence ?? null} />}
      >
        <LicenceEditor
          assetId={asset.id}
          existing={asset.licence ?? null}
          onReviewed={() => reviewLicence.mutate(asset.id)}
        />
      </Section>

      <Section title="Projects" accessory={`${asset.projects.length}`} defaultOpen={false}>
        {asset.projects.length === 0 ? (
          <div className="text-label text-subtle-foreground">Not pinned to any project yet</div>
        ) : (
          <ul className="flex flex-col gap-1 text-label">
            {asset.projects.map((p) => (
              <li key={p.project_id} className="flex items-center justify-between gap-2">
                <Link to={`/projects/${p.project_id}`} className={linkClass}>
                  {p.project_name}
                </Link>
                <span className="shrink-0 font-mono text-muted-foreground">
                  {/* A reference asset's pin carries no version. */}
                  {asset.is_reference ? "Reference" : p.version_label}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Collections" accessory={`${asset.collections.length}`} defaultOpen={false}>
        {asset.collections.length === 0 ? (
          <div className="text-label text-subtle-foreground">Not in any collections</div>
        ) : (
          <ul className="flex flex-col gap-0.5 text-label">
            {asset.collections.map((c) => (
              <li key={c.collection_id} className="flex items-center justify-between gap-2">
                <Link to={`/collections/${c.collection_id}`} className={linkClass}>
                  {c.collection_name}
                </Link>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label={`Remove from ${c.collection_name}`}
                  disabled={removeFromCollection.isPending}
                  onClick={() =>
                    removeFromCollection.mutate({
                      collectionId: c.collection_id,
                      assetId: asset.id,
                    })
                  }
                >
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Audit" defaultOpen={false}>
        <AssetAuditSection assetId={asset.id} />
      </Section>

      <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-border pt-3">
        <Button
          variant="destructive"
          leadingIcon={<Trash2 size={12} />}
          onClick={async () => {
            const ok = await confirm({
              title: `Delete "${asset.name}"?`,
              description: asset.is_reference
                ? "This removes the record. Nothing at its path is touched."
                : "This removes all versions and files.",
              confirmLabel: "Delete asset",
              tone: "destructive",
            });
            if (ok) del.mutate(asset.id, { onSuccess: onDeleted });
          }}
        >
          Delete asset
        </Button>
        {/* Merging moves versions, which a reference asset has none of. */}
        {!asset.is_reference && (
          <>
            <Button
              variant="ghost"
              leadingIcon={<GitMerge size={12} />}
              onClick={() => setMergeOpen(true)}
            >
              Merge…
            </Button>
            <MaintenanceMenu asset={asset} />
          </>
        )}
      </div>
      <MergeDialog
        targetId={asset.id}
        targetName={asset.name}
        open={mergeOpen}
        onOpenChange={setMergeOpen}
      />
      {confirmDialog}
    </div>
  );
}

// The cover: the storefront gallery when there is one, else the typeset cover
// the library card shows, so the page reads as the same asset.
function PackageCover({ asset }: { asset: AssetDetailView }) {
  if (asset.images.length > 0) return <AssetGallery images={asset.images} />;
  return (
    <div className="aspect-video w-full overflow-hidden rounded-md">
      <TypesetCover name={asset.name} publisher={asset.publisher} />
    </div>
  );
}

// MaintenanceMenu holds the three ways to rebuild what Damson derived from a
// package. They used to be three stacked buttons that read as one; in a menu
// each says what it redoes.
function MaintenanceMenu({ asset }: { asset: AssetDetailView }) {
  const requeuePreviews = useRequeueAssetPreviews();
  const reextract = useReExtractVersion();
  const rescan = useRescanAsset();
  const [note, setNote] = useState<string | null>(null);

  const reExtractFrom = reExtractSource(asset);
  const pending = reextract.isPending || requeuePreviews.isPending || rescan.isPending;

  const onRescan = () => {
    setNote(null);
    rescan.mutate(asset.id, {
      onSuccess: (res) => {
        const parts: string[] = [];
        if (res.thumbnail_regenerated) parts.push("thumbnail refreshed");
        if (res.previews_queued > 0) parts.push(`${res.previews_queued} previews queued`);
        if (res.licence_updated) parts.push(`licence ${res.detected_type || "detected"}`);
        setNote(
          parts.length > 0 ? `Rescanned: ${parts.join(" · ")}` : "Rescanned. Nothing to update.",
        );
      },
    });
  };

  const onReExtract = () => {
    if (!reExtractFrom) return;
    setNote(null);
    reextract.mutate(
      { assetId: asset.id, versionId: reExtractFrom.id },
      {
        onSuccess: () =>
          requeuePreviews.mutate(asset.id, { onSuccess: () => setNote("Files re-extracted.") }),
      },
    );
  };

  const onRequeue = () => {
    setNote(null);
    requeuePreviews.mutate(asset.id, { onSuccess: () => setNote("Previews queued.") });
  };

  return (
    <div className="flex min-w-0 items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" leadingIcon={<Wrench size={12} />} disabled={pending}>
            {pending ? "Working…" : "Maintenance"}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onSelect={onRescan}>
            <RefreshCw size={12} /> Rescan package
          </DropdownMenuItem>
          {reExtractFrom && (
            <DropdownMenuItem onSelect={onReExtract}>
              <RotateCw size={12} /> Re-extract files
            </DropdownMenuItem>
          )}
          <DropdownMenuItem onSelect={onRequeue}>
            <RefreshCw size={12} /> Regenerate previews
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {note && (
        <span role="status" className="truncate text-label text-subtle-foreground">
          {note}
        </span>
      )}
    </div>
  );
}
