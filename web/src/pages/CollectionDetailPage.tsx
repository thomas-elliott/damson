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
  Toolbar,
  ToolbarSpacer,
  useConfirm,
} from "@/design-system";
import { useAssets, useCollection, useLibrarySettings } from "@/hooks/useDataQueries";
import { useDeleteCollection } from "@/hooks/useMutations";
import { parsePaging } from "@/lib/libraryFilters";

export function CollectionDetailPage() {
  const { id } = useParams();
  const collectionId = id ? Number(id) : null;
  const { data: collection, loading, error, failed, reload } = useCollection(collectionId);
  const { data: settings } = useLibrarySettings();
  const [search] = useSearchParams();
  const { page, pageSize } = parsePaging(search);
  const {
    data: assets,
    total,
    loading: assetsLoading,
    placeholder,
    error: assetsError,
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

          {loading ? (
            <div className="flex flex-1 items-center justify-center text-ui text-subtle-foreground">
              Loading…
            </div>
          ) : failed && !isNotFound(error) ? (
            <ErrorState title="Couldn't load this collection" onRetry={reload} />
          ) : !collection ? (
            <EmptyState title="Collection not found" />
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
                className="shrink-0 justify-end border-t border-border bg-background px-3.5 py-1.5"
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
