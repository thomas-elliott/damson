import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { ChevronLeft, Plus, Trash2 } from "lucide-react";

import { isNotFound } from "@/api";
import { AssetGrid } from "@/components/AssetGrid";
import { AssetPicker } from "@/components/AssetPicker";
import { Shell } from "@/components/Shell";
import { UrlPager } from "@/components/UrlPager";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbSeparator,
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  ErrorState,
  LoadingState,
  Toolbar,
  ToolbarSpacer,
  useConfirm,
} from "@/design-system";
import {
  useAssets,
  useCollection,
  useLibrarySettings,
  useMe,
  type AssetRow,
} from "@/hooks/useDataQueries";
import { useAddAssetToCollection, useDeleteCollection } from "@/hooks/useMutations";
import { parsePaging } from "@/lib/libraryFilters";

export function CollectionDetailPage() {
  const { id } = useParams();
  const collectionId = id ? Number(id) : null;
  const { data: collection, loading, error, failed, reload } = useCollection(collectionId);
  const { data: me } = useMe();
  const { data: settings, loading: settingsLoading } = useLibrarySettings(me?.current_library?.id);
  const [search] = useSearchParams();
  const { page, pageSize } = parsePaging(search);
  const {
    data: assets,
    total,
    loading: assetsLoading,
    placeholder,
    error: assetsError,
    failed: assetsFailed,
    reload: reloadAssets,
  } = useAssets(collectionId ? { collection_id: collectionId } : undefined, collectionId != null, {
    limit: pageSize,
    offset: (page - 1) * pageSize,
  });
  const navigate = useNavigate();
  const del = useDeleteCollection();
  const { confirm, dialog: confirmDialog } = useConfirm();
  const [addOpen, setAddOpen] = useState(false);

  if (!collectionId) return null;

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Button
              variant="ghost"
              size="sm"
              leadingIcon={<ChevronLeft size={12} />}
              onClick={() => navigate("/collections")}
            >
              Collections
            </Button>
            <Breadcrumb>
              <BreadcrumbItem>Collections</BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem current heading>
                {collection?.name ?? "—"}
              </BreadcrumbItem>
            </Breadcrumb>
            <ToolbarSpacer />
            <Button
              variant="destructive"
              leadingIcon={<Trash2 size={12} />}
              onClick={async () => {
                if (!collection) return;
                const ok = await confirm({
                  title: `Delete collection "${collection.name}"?`,
                  description: "Assets stay in your library — only the collection is removed.",
                  confirmLabel: "Delete collection",
                  tone: "destructive",
                });
                if (ok) {
                  del.mutate(collection.id, {
                    onSuccess: () => navigate("/collections"),
                  });
                }
              }}
            >
              Delete
            </Button>
          </Toolbar>

          {/* Only the grid waits for the settings, so it first draws at the
              library's density; if they fail it draws at medium. */}
          {loading ? (
            <LoadingState />
          ) : failed && !isNotFound(error) ? (
            <ErrorState title="Couldn't load this collection" onRetry={reload} />
          ) : !collection ? (
            <EmptyState title="Collection not found" />
          ) : assetsLoading || (settingsLoading && !assetsFailed && total > 0) ? (
            <LoadingState />
          ) : assetsFailed ? (
            <ErrorState title="Couldn't load this collection's assets" onRetry={reloadAssets} />
          ) : total === 0 ? (
            <EmptyState
              title="No assets in this collection"
              description="Pick assets from your library to add them here. In the Library, you can also select several and press Collection in the bar at the bottom."
              action={
                <Button
                  variant="primary"
                  leadingIcon={<Plus size={12} />}
                  onClick={() => setAddOpen(true)}
                >
                  Add asset
                </Button>
              }
            />
          ) : (
            <>
              <AssetGrid
                // A new page starts scrolled to the top.
                key={`${page}/${pageSize}`}
                assets={assets}
                onOpen={(id) => navigate(`/library/${id}`)}
                density={settings?.grid_density_default}
              />
              <UrlPager
                aria-label="Collection pages"
                total={total}
                query={{ loading: assetsLoading, placeholder, error: assetsError }}
              />
            </>
          )}
          {confirmDialog}
          <AddAssetDialog
            open={addOpen}
            onOpenChange={setAddOpen}
            collectionId={collectionId}
            onAdded={reloadAssets}
          />
        </>
      }
    />
  );
}

function AddAssetDialog({
  open,
  onOpenChange,
  collectionId,
  onAdded,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  collectionId: number;
  /** The collection's asset list is its own query, so the caller reloads it. */
  onAdded: () => unknown;
}) {
  const add = useAddAssetToCollection();
  const [picked, setPicked] = useState<AssetRow | null>(null);

  const close = (next: boolean) => {
    if (!next) {
      setPicked(null);
      add.reset();
    }
    onOpenChange(next);
  };

  const submit = () => {
    if (!picked) return;
    add.mutate(
      { collectionId, asset_id: picked.id },
      {
        onSuccess: () => {
          void onAdded();
          close(false);
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add an asset</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <AssetPicker
            aria-label="Asset"
            placeholder="Search assets to add"
            selectedId={picked?.id ?? null}
            onSelect={(a) => {
              setPicked(a);
              add.reset();
            }}
          />
          {picked && (
            <p className="text-label text-muted-foreground">
              Adding <span className="text-foreground">{picked.name}</span>
            </p>
          )}
          {add.error && <p className="text-label text-destructive">{add.error.message}</p>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => close(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!picked || add.isPending}>
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
