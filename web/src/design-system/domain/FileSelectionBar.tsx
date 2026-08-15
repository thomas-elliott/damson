import { FolderPlus, Plus, X } from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { SelectionBar } from "@/design-system/compositions/SelectionBar";

export interface FileSelectionBarProps {
  count: number;
  onClear?: () => void;
  /** Add the current selection to an existing subset / version flow. */
  onCreateSubset?: () => void;
  /** Start a brand-new subset from the selection. */
  onNewSubset?: () => void;
  /** Hint text shown when drag-to-subset is available. */
  dragHint?: string;
}

/**
 * Floating selection bar for the file browser. Wraps the design-system
 * SelectionBar with file-specific actions (create / new subset). Auto-hides
 * when nothing is selected.
 */
export function FileSelectionBar({
  count,
  onClear,
  onCreateSubset,
  onNewSubset,
  dragHint,
}: FileSelectionBarProps) {
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
          {onNewSubset && (
            <Button variant="secondary" size="sm" onClick={onNewSubset}>
              <Plus size={12} /> New subset
            </Button>
          )}
          <IconButton aria-label="Clear selection" size="sm" onClick={onClear}>
            <X />
          </IconButton>
        </>
      }
    >
      {dragHint && (
        <span className="whitespace-nowrap font-mono text-small text-faint-foreground">
          {dragHint}
        </span>
      )}
    </SelectionBar>
  );
}
