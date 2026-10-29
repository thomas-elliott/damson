import { useNavigate, useParams, useSearchParams } from "react-router";
import { ChevronLeft, Trash2 } from "lucide-react";

import { isNotFound } from "@/api";
import { AssetGrid } from "@/components/AssetGrid";
import { Shell } from "@/components/Shell";
import { UrlPager } from "@/components/UrlPager";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbSeparator,
  Button,
  EmptyState,
  ErrorState,
  LoadingState,
  Toolbar,
  ToolbarSpacer,
  useConfirm,
} from "@/design-system";
import { useAssets, useCollection, useLibrarySettings, useMe } from "@/hooks/useDataQueries";
import { useDeleteCollection } from "@/hooks/useMutations";
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
              <BreadcrumbItem current>{collection?.name ?? "—"}</BreadcrumbItem>
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
              description="Add assets from their detail panels."
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
        </>
      }
    />
  );
}
