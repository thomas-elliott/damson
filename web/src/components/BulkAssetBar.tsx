import { useState } from "react";

import { useNotifications } from "@/components/notifications/NotificationProvider";
import {
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  SelectionBar,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
  useConfirm,
} from "@/design-system";
import { useCollections } from "@/hooks/useDataQueries";
import { useBulkAssets, type BulkAssetAction } from "@/hooks/useMutations";

type OpenDialog = "tag" | "collection" | "licence" | null;

export function BulkAssetBar({ assetIds, onClear }: { assetIds: number[]; onClear: () => void }) {
  const bulk = useBulkAssets();
  const { notify } = useNotifications();
  const { confirm, dialog: confirmDialog } = useConfirm();
  const { data: collections } = useCollections(true);

  const [dialog, setDialog] = useState<OpenDialog>(null);
  const [tagPath, setTagPath] = useState("");
  const [collectionId, setCollectionId] = useState("");
  const [licenceText, setLicenceText] = useState("");

  const count = assetIds.length;

  const run = (action: BulkAssetAction, verb: string) => {
    bulk.mutate(
      { asset_ids: assetIds, ...action },
      {
        onSuccess: (r) => {
          notify({
            tone: "success",
            title: `${verb} ${r.affected} asset${r.affected === 1 ? "" : "s"}`,
          });
          setDialog(null);
          setTagPath("");
          setCollectionId("");
          setLicenceText("");
          onClear();
        },
        onError: (e) => {
          notify({
            tone: "error",
            title: "Bulk action failed",
            message: e instanceof Error ? e.message : "request failed",
          });
        },
      },
    );
  };

  const deleteSelected = async () => {
    const ok = await confirm({
      title: `Delete ${count} asset${count === 1 ? "" : "s"}?`,
      description: "This removes the assets and their versions. Stored blobs are reclaimed later.",
      confirmLabel: "Delete",
      tone: "destructive",
    });
    if (ok) run({ action: "delete" }, "Deleted");
  };

  return (
    <>
      <SelectionBar
        count={count}
        onDismiss={onClear}
        actions={
          <>
            <Button size="sm" variant="ghost" onClick={() => run({ action: "review" }, "Reviewed")}>
              Mark reviewed
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => run({ action: "unreview" }, "Flagged")}
            >
              Needs review
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setDialog("tag")}>
              Tag
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setDialog("collection")}>
              Collection
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setDialog("licence")}>
              Licence
            </Button>
            <Button size="sm" variant="destructive" onClick={() => void deleteSelected()}>
              Delete
            </Button>
            <Button size="sm" variant="ghost" onClick={onClear}>
              Clear
            </Button>
          </>
        }
      />

      <Dialog open={dialog === "tag"} onOpenChange={(o) => !o && setDialog(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Tag {count} assets</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <Input
              value={tagPath}
              onChange={(e) => setTagPath(e.currentTarget.value)}
              placeholder="e.g. environment/rocks"
              aria-label="Tag path"
            />
          </DialogBody>
          <DialogFooter>
            <Button
              variant="ghost"
              disabled={!tagPath.trim()}
              onClick={() => run({ action: "remove_tag", path: tagPath.trim() }, "Untagged")}
            >
              Remove tag
            </Button>
            <Button
              variant="primary"
              disabled={!tagPath.trim()}
              onClick={() => run({ action: "add_tag", path: tagPath.trim() }, "Tagged")}
            >
              Add tag
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialog === "collection"} onOpenChange={(o) => !o && setDialog(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add {count} assets to a collection</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <Select value={collectionId} onValueChange={setCollectionId}>
              <SelectTrigger aria-label="Collection">
                <SelectValue placeholder="Choose a collection" />
              </SelectTrigger>
              <SelectContent>
                {collections.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </DialogBody>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialog(null)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              disabled={!collectionId}
              onClick={() =>
                run({ action: "add_to_collection", collection_id: Number(collectionId) }, "Added")
              }
            >
              Add to collection
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialog === "licence"} onOpenChange={(o) => !o && setDialog(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Set licence on {count} assets</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <Textarea
              value={licenceText}
              onChange={(e) => setLicenceText(e.currentTarget.value)}
              placeholder="Paste the licence text…"
              rows={8}
              aria-label="Licence text"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialog(null)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              disabled={!licenceText.trim()}
              onClick={() =>
                run(
                  { action: "set_licence", original_text: licenceText, use_detected: true },
                  "Licensed",
                )
              }
            >
              Apply licence
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {confirmDialog}
    </>
  );
}
