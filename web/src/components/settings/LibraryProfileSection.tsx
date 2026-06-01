import { useState } from "react";
import { useNavigate } from "react-router";
import { Trash2 } from "lucide-react";

import {
  Badge,
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  FieldRow,
  Input,
  Section,
  Textarea,
} from "@/design-system";
import { type MeLibrary } from "@/hooks/useDataQueries";
import { useDeleteLibrary, usePatchLibrary } from "@/hooks/useMutations";
import { useSavedFlash } from "@/hooks/useSavedFlash";

/**
 * Library name + description editor. Admins can save changes; everyone else
 * sees the values rendered as a read-only summary. The library description
 * isn't loaded into `MeLibrary` today; pull it from elsewhere when we add
 * a /api/libraries/:id GET. For now the editor opens with whatever is in
 * the membership view's name and lets admins update both fields blind —
 * it's still better than the no-editor M5 baseline.
 *
 * Admins also see a danger zone with the library deletion action.
 */
export function LibraryProfileSection({
  library,
  isAdmin,
}: {
  library: MeLibrary;
  isAdmin: boolean;
}) {
  // `name` seeds from the library and is reset across libraries by the
  // key={library.id} the parent puts on this section (see SettingsPage), so no
  // sync effect is needed here.
  const patch = usePatchLibrary();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState("");
  const { saved, flash } = useSavedFlash();

  const submit = () => {
    patch.mutate(
      {
        libraryId: library.id,
        name: name.trim() || undefined,
        description,
      },
      { onSuccess: flash },
    );
  };

  return (
    <>
      <Section
        title="Library profile"
        defaultOpen
        accessory={
          saved ? (
            <Badge size="sm" tone="ok">
              Saved
            </Badge>
          ) : null
        }
      >
        <FieldRow label="Name">
          <Input value={name} onChange={(e) => setName(e.target.value)} disabled={!isAdmin} />
        </FieldRow>
        <FieldRow label="Description" hint="Optional. Visible to all members.">
          <Textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            disabled={!isAdmin}
          />
        </FieldRow>
        {isAdmin && (
          <div className="flex justify-end">
            <Button variant="primary" disabled={patch.isPending || !name.trim()} onClick={submit}>
              Save
            </Button>
          </div>
        )}
      </Section>
      {isAdmin && <DangerZone library={library} />}
    </>
  );
}

function DangerZone({ library }: { library: MeLibrary }) {
  const [open, setOpen] = useState(false);
  return (
    <Section title="Danger zone" defaultOpen={false}>
      <div className="flex items-start justify-between gap-4 rounded-md border border-err/40 bg-err-tint/30 p-3">
        <div className="flex flex-col">
          <span className="text-ui text-text">Delete this library</span>
          <span className="text-caption">
            Removes every asset, project, collection, tag, setting, and member of{" "}
            <span className="font-mono">{library.name}</span>. Blob storage is untouched. This can't
            be undone.
          </span>
        </div>
        <Button variant="danger" leadingIcon={<Trash2 size={12} />} onClick={() => setOpen(true)}>
          Delete library
        </Button>
      </div>
      <DeleteLibraryDialog library={library} open={open} onOpenChange={setOpen} />
    </Section>
  );
}

function DeleteLibraryDialog({
  library,
  open,
  onOpenChange,
}: {
  library: MeLibrary;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  // The form's confirm/error state lives in DeleteLibraryForm, which Radix
  // only mounts while the dialog is open — so it starts fresh on every open
  // without a reset effect.
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DeleteLibraryForm library={library} onOpenChange={onOpenChange} />
      </DialogContent>
    </Dialog>
  );
}

function DeleteLibraryForm({
  library,
  onOpenChange,
}: {
  library: MeLibrary;
  onOpenChange: (v: boolean) => void;
}) {
  const navigate = useNavigate();
  const del = useDeleteLibrary();
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);

  const matches = confirm === library.name;
  const submit = () => {
    if (!matches) return;
    setError(null);
    del.mutate(library.id, {
      onSuccess: () => {
        onOpenChange(false);
        // Server cleared current_library_id; bouncing the user to the root
        // re-runs the boot-up flow (no library → bootstrap or login).
        navigate("/", { replace: true });
      },
      onError: (e: unknown) => {
        setError(e instanceof Error ? e.message : "Delete failed");
      },
    });
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>Delete library?</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <p className="text-ui text-text-2">
          All assets, projects, collections, tags, settings, and member roles for{" "}
          <span className="font-mono">{library.name}</span> will be removed. Members will be signed
          out of any open sessions.
        </p>
        <FieldRow label="Type the library name to confirm" hint="Case-sensitive.">
          <Input
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoFocus
            placeholder={library.name}
          />
        </FieldRow>
        {error && <p className="text-small text-err">{error}</p>}
      </DialogBody>
      <DialogFooter>
        <Button variant="ghost" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant="danger" disabled={!matches || del.isPending} onClick={submit}>
          {del.isPending ? "Deleting…" : "Delete library"}
        </Button>
      </DialogFooter>
    </>
  );
}
