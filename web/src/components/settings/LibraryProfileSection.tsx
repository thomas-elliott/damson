import { useState } from "react";

import { Badge, Button, FieldRow, Input, Section, Textarea } from "@/design-system";
import { type MeLibrary } from "@/hooks/useDataQueries";
import { usePatchLibrary } from "@/hooks/useMutations";
import { useSavedFlash } from "@/hooks/useSavedFlash";

/**
 * Library name + description editor. Admins can save changes; everyone else
 * sees the values rendered as a read-only summary.
 */
export function LibraryProfileSection({
  library,
  isAdmin,
}: {
  library: MeLibrary;
  isAdmin: boolean;
}) {
  // `name` and `description` seed from the library and are reset across
  // libraries by the key={library.id} the parent puts on this section (see
  // SettingsPage), so no sync effect is needed here.
  const patch = usePatchLibrary();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState(library.description);
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
  );
}
