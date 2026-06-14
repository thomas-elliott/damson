import { Fragment } from "react";
import { Library, LayoutGrid, List } from "lucide-react";

import { cn } from "@/design-system/utils";
import { Chip } from "@/design-system/primitives/Chip";
import { SegmentedControl } from "@/design-system/primitives/SegmentedControl";
import { Breadcrumb, BreadcrumbSeparator } from "@/design-system/compositions/Breadcrumb";
import { Toolbar, ToolbarSpacer } from "@/design-system/compositions/Toolbar";

import { fileTypeMeta, type FileType } from "./fileTypes";

export type FileBrowserView = "grid" | "list";

export interface FileBrowserToolbarProps {
  assetName: string;
  /** Current folder path; empty = pack root. */
  path: string[];
  onNavigate: (path: string[]) => void;
  onExit?: () => void;
  /** Item/match count for the current listing. */
  count: number;
  /** Flat search/filter results mode changes the breadcrumb + count wording. */
  resultsMode?: boolean;
  /** Search query shown as the trailing crumb in results mode. */
  query?: string;
  onClearResults?: () => void;

  view: FileBrowserView;
  onViewChange: (view: FileBrowserView) => void;

  activeTypes?: FileType[];
  onToggleType?: (ftype: FileType) => void;
}

const crumbButton =
  "rounded-sm text-text-2 transition-colors duration-std ease hover:text-text focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring";

/** Breadcrumb path navigation + active filter chips + grid/list toggle. */
export function FileBrowserToolbar({
  assetName,
  path,
  onNavigate,
  onExit,
  count,
  resultsMode,
  query,
  onClearResults,
  view,
  onViewChange,
  activeTypes = [],
  onToggleType,
}: FileBrowserToolbarProps) {
  const trimmedQuery = query?.trim();
  return (
    <Toolbar>
      <Breadcrumb
        count={
          resultsMode
            ? `${count} ${count === 1 ? "match" : "matches"}`
            : `${count} ${count === 1 ? "item" : "items"}`
        }
      >
        <button
          type="button"
          onClick={onExit}
          className={cn("flex items-center gap-1", crumbButton)}
        >
          <Library size={12} /> Library
        </button>
        <BreadcrumbSeparator />
        <button type="button" onClick={() => onNavigate([])} className={crumbButton}>
          {assetName}
        </button>
        {resultsMode ? (
          <>
            <BreadcrumbSeparator />
            <span className="text-text">{trimmedQuery ? `“${trimmedQuery}”` : "Filtered"}</span>
          </>
        ) : (
          path.map((seg, i) => (
            <Fragment key={i}>
              <BreadcrumbSeparator />
              <button
                type="button"
                aria-current={i === path.length - 1 ? "page" : undefined}
                onClick={() => onNavigate(path.slice(0, i + 1))}
                className={cn(crumbButton, "aria-[current=page]:text-text")}
              >
                {seg}
              </button>
            </Fragment>
          ))
        )}
      </Breadcrumb>

      {activeTypes.length > 0 && (
        <>
          <span className="h-4 w-px bg-line" aria-hidden />
          {activeTypes.map((ftype) => (
            <Chip key={ftype} label="type:" onRemove={() => onToggleType?.(ftype)}>
              {fileTypeMeta(ftype).label}
            </Chip>
          ))}
        </>
      )}

      <ToolbarSpacer />

      {resultsMode && onClearResults && (
        <button type="button" onClick={onClearResults} className={cn("text-small", crumbButton)}>
          Clear
        </button>
      )}
      <SegmentedControl
        aria-label="View mode"
        value={view}
        onValueChange={onViewChange}
        options={[
          { value: "grid", icon: <LayoutGrid size={13} />, "aria-label": "Grid view" },
          { value: "list", icon: <List size={13} />, "aria-label": "List view" },
        ]}
      />
    </Toolbar>
  );
}
