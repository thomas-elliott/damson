import { useNavigate, useParams } from "react-router";
import { ChevronLeft, Trash2 } from "lucide-react";

import { AssetGrid } from "@/components/AssetGrid";
import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbSeparator,
  Button,
  EmptyState,
  Toolbar,
  ToolbarSpacer,
  useConfirm,
} from "@/design-system";
import { useAssets, useCollection } from "@/hooks/useDataQueries";
import { useDeleteCollection } from "@/hooks/useMutations";

export function CollectionDetailPage() {
  const { id } = useParams();
  const collectionId = id ? Number(id) : null;
  const { data: collection, loading } = useCollection(collectionId);
  const { data: assets } = useAssets(
    collectionId ? { collection_id: collectionId } : undefined,
    collectionId != null,
  );
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
              variant="danger"
              leadingIcon={<Trash2 size={12} />}
              onClick={async () => {
                if (!collection) return;
                const ok = await confirm({
                  title: `Delete collection "${collection.name}"?`,
                  description: "Assets stay in your library — only the collection is removed.",
                  confirmLabel: "Delete collection",
                  tone: "danger",
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
            <div className="flex flex-1 items-center justify-center text-ui text-text-3">
              Loading…
            </div>
          ) : !collection ? (
            <EmptyState title="Collection not found" />
          ) : assets.length === 0 ? (
            <EmptyState
              title="No assets in this collection"
              description="Add assets from their detail panels."
            />
          ) : (
            <AssetGrid assets={assets} onOpen={(id) => navigate(`/library/${id}`)} />
          )}
          {confirmDialog}
        </>
      }
    />
  );
}
