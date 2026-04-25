import { useEffect, useState } from "react";

import {
  Badge,
  Button,
  FieldRow,
  Input,
  Section,
  Textarea,
} from "@/design-system";
import { type MeLibrary } from "@/hooks/useDataQueries";
import { usePatchLibrary } from "@/hooks/useMutations";

/**
 * Library name + description editor. Admins can save changes; everyone else
 * sees the values rendered as a read-only summary. The library description
 * isn't loaded into `MeLibrary` today; pull it from elsewhere when we add
 * a /api/libraries/:id GET. For now the editor opens with whatever is in
 * the membership view's name and lets admins update both fields blind —
 * it's still better than the no-editor M5 baseline.
 */
export function LibraryProfileSection({
  library,
  isAdmin,
}: {
  library: MeLibrary;
  isAdmin: boolean;
}) {
  const patch = usePatchLibrary();
  const [name, setName] = useState(library.name);
  const [description, setDescription] = useState("");
  const [savedAt, setSavedAt] = useState<number | null>(null);

  useEffect(() => {
    setName(library.name);
  }, [library.name]);

  useEffect(() => {
    if (!patch.isSuccess) return;
    setSavedAt(Date.now());
    const t = window.setTimeout(() => setSavedAt(null), 1500);
    return () => window.clearTimeout(t);
  }, [patch.submittedAt, patch.isSuccess]);

  const submit = () => {
    patch.mutate({
      libraryId: library.id,
      name: name.trim() || undefined,
      description,
    });
  };

  return (
    <Section
      title="Library profile"
      defaultOpen
      accessory={savedAt ? <Badge size="sm" tone="ok">Saved</Badge> : null}
    >
      <FieldRow label="Name">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={!isAdmin}
        />
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
          <Button
            variant="primary"
            disabled={patch.isPending || !name.trim()}
            onClick={submit}
          >
            Save
          </Button>
        </div>
      )}
    </Section>
  );
}
