import { FolderPlus, X } from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { SelectionBar } from "@/design-system/compositions/SelectionBar";

export interface FileSelectionBarProps {
  count: number;
  onClear?: () => void;
  /** Add the current selection to an existing subset / version flow. */
  onCreateSubset?: () => void;
}

/**
 * Floating selection bar for the file browser. Wraps the design-system
 * SelectionBar with file-specific actions. Auto-hides when nothing is selected.
 */
export function FileSelectionBar({ count, onClear, onCreateSubset }: FileSelectionBarProps) {
  if (count <= 0) return null;
  return (
    <SelectionBar
      count={count}
      actions={
        <>
          {onCreateSubset && (
            <Button variant="secondary" size="sm" onClick={onCreateSubset}>
              <FolderPlus size={12} /> Create subset
            </Button>
          )}
          <IconButton aria-label="Clear selection" size="sm" onClick={onClear}>
            <X />
          </IconButton>
        </>
      }
    />
  );
}
