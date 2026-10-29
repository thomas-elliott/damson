import { useState } from "react";
import { useNavigate } from "react-router";
import { Plus } from "lucide-react";

import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  Button,
  CollectionCard,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  ErrorState,
  FieldRow,
  Input,
  LoadingState,
  PageContent,
  Textarea,
  Toolbar,
  ToolbarSpacer,
} from "@/design-system";
import { useCollections } from "@/hooks/useDataQueries";
import { useCreateCollection } from "@/hooks/useMutations";

export function CollectionsPage() {
  const { data: collections, loading, failed, reload } = useCollections();
  const navigate = useNavigate();
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${collections.length}`}>
              <BreadcrumbItem current>Collections</BreadcrumbItem>
            </Breadcrumb>
            <ToolbarSpacer />
            <Button
              variant="primary"
              leadingIcon={<Plus size={12} />}
              onClick={() => setDialogOpen(true)}
            >
              New collection
            </Button>
          </Toolbar>

          {loading ? (
            <LoadingState />
          ) : failed ? (
            <ErrorState title="Couldn't load collections" onRetry={reload} />
          ) : collections.length === 0 ? (
            <EmptyState
              icon={<Plus size={28} />}
              title="No collections yet"
              description="Collections are curated groupings — like bookmarks for the library. An asset can live in multiple collections."
              action={
                <Button
                  variant="primary"
                  leadingIcon={<Plus size={12} />}
                  onClick={() => setDialogOpen(true)}
                >
                  New collection
                </Button>
              }
            />
          ) : (
            <PageContent>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
                {collections.map((c) => (
                  <CollectionCard
                    key={c.id}
                    name={c.name}
                    count={c.asset_count}
                    onSelect={() => navigate(`/collections/${c.id}`)}
                  />
                ))}
              </div>
            </PageContent>
          )}

          <NewCollectionDialog open={dialogOpen} onOpenChange={setDialogOpen} />
        </>
      }
    />
  );
}

function NewCollectionDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const create = useCreateCollection();
  const navigate = useNavigate();

  const submit = () => {
    setError(null);
    create.mutate(
      { name: name.trim(), description },
      {
        onSuccess: (c) => {
          onOpenChange(false);
          setName("");
          setDescription("");
          navigate(`/collections/${c.id}`);
        },
        onError: (err) => setError(err instanceof Error ? err.message : "failed"),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New collection</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <FieldRow label="Name">
            <Input
              value={name}
              autoFocus
              onChange={(e) => setName(e.currentTarget.value)}
              placeholder="Boss fight audio"
            />
          </FieldRow>
          <FieldRow label="Description">
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.currentTarget.value)}
              placeholder="Optional"
              rows={2}
            />
          </FieldRow>
          {error && <div className="text-label text-destructive">{error}</div>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!name.trim() || create.isPending}>
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
