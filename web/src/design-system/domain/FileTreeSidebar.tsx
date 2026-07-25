import { useState, type ReactNode } from "react";
import { Boxes, ChevronDown, ChevronLeft, ChevronRight, Folder, FolderOpen } from "lucide-react";

import { cn } from "@/design-system/utils";
import { SearchInput } from "@/design-system/primitives/SearchInput";
import { FilterRow, FilterSidebar } from "@/design-system/compositions/FilterSidebar";
import { Section } from "@/design-system/compositions/Section";

import { FileGlyph } from "./FileGlyph";
import { fileTypeMeta, type FileType } from "./fileTypes";
import { countFiles, folderKeysToDepth, type FolderNode, type TreeNode } from "./fileTree";

/** Folders shallower than this are expanded on first render. */
const DEFAULT_EXPANDED_DEPTH = 3;

export interface FileTreeSidebarProps {
  /** Scope header — the asset being browsed. */
  assetName: string;
  assetIcon?: ReactNode;
  onExit?: () => void;

  query: string;
  onQueryChange: (next: string) => void;

  tree: TreeNode[];
  /** Total files in the pack — shown against the "All files" root row. */
  totalFiles: number;
  /** Current folder path; navigating reveals it. Empty = root. */
  path: string[];
  onNavigate: (path: string[]) => void;

  /** File counts per type, for the filter group. */
  typeCounts: Partial<Record<FileType, number>>;
  activeTypes: Set<FileType>;
  onToggleType: (ftype: FileType) => void;

  className?: string;
}

/** Collect the open-by-default folder keys (every ancestor of the path). */
function ancestorKeys(path: string[]): string[] {
  const keys: string[] = [];
  const acc: string[] = [];
  for (const seg of path) {
    acc.push(seg);
    keys.push(acc.join("/"));
  }
  return keys;
}

export function FileTreeSidebar({
  assetName,
  assetIcon,
  onExit,
  query,
  onQueryChange,
  tree,
  totalFiles,
  path,
  onNavigate,
  typeCounts,
  activeTypes,
  onToggleType,
  className,
}: FileTreeSidebarProps) {
  const [openFolders, setOpenFolders] = useState<Set<string>>(
    () => new Set([...folderKeysToDepth(tree, DEFAULT_EXPANDED_DEPTH), ...ancestorKeys(path)]),
  );

  const pathKey = path.join("/");
  const [prevPathKey, setPrevPathKey] = useState(pathKey);
  if (prevPathKey !== pathKey) {
    setPrevPathKey(pathKey);
    setOpenFolders((prev) => {
      const next = new Set(prev);
      for (const key of ancestorKeys(path)) next.add(key);
      return next;
    });
  }

  const toggleFolder = (key: string) =>
    setOpenFolders((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const types = (Object.keys(typeCounts) as FileType[]).sort(
    (a, b) => (typeCounts[b] ?? 0) - (typeCounts[a] ?? 0),
  );
  const folders = tree.filter((n): n is FolderNode => n.type === "folder");

  return (
    <FilterSidebar
      className={className}
      header={
        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={onExit}
            title="Back to library"
            className="flex items-center gap-1.5 rounded-sm text-ui font-medium text-text-2 transition-colors duration-std ease hover:text-text focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring"
          >
            <ChevronLeft size={13} className="shrink-0 text-text-3" />
            {assetIcon}
            <span className="truncate">{assetName}</span>
          </button>
          <SearchInput
            placeholder="Search files in this pack…"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
          />
        </div>
      }
    >
      <Section title="Folders">
        <TreeRow
          icon={<Boxes size={12} strokeWidth={1.5} />}
          label="All files"
          count={totalFiles}
          depth={0}
          active={path.length === 0}
          onActivate={() => onNavigate([])}
        />
        {folders.map((node) => (
          <TreeNavNode
            key={node.name}
            node={node}
            parent={[]}
            depth={0}
            openFolders={openFolders}
            toggleFolder={toggleFolder}
            activePathKey={pathKey}
            onNavigate={onNavigate}
          />
        ))}
      </Section>

      <Section title="File type">
        {types.map((ftype) => (
          <FilterRow
            key={ftype}
            icon={<FileGlyph ftype={ftype} size={13} strokeWidth={1.6} />}
            label={fileTypeMeta(ftype).label}
            count={typeCounts[ftype]}
            active={activeTypes.has(ftype)}
            onClick={() => onToggleType(ftype)}
          />
        ))}
      </Section>
    </FilterSidebar>
  );
}

interface TreeNavNodeProps {
  node: FolderNode;
  parent: string[];
  depth: number;
  openFolders: Set<string>;
  toggleFolder: (key: string) => void;
  activePathKey: string;
  onNavigate: (path: string[]) => void;
}

/**
 * One folder in the tree. Files are deliberately absent — the grid opens
 * files; the tree only navigates folders.
 */
function TreeNavNode({
  node,
  parent,
  depth,
  openFolders,
  toggleFolder,
  activePathKey,
  onNavigate,
}: TreeNavNodeProps) {
  const here = [...parent, node.name];
  const key = here.join("/");
  const childFolders = node.children.filter((n): n is FolderNode => n.type === "folder");
  const isOpen = openFolders.has(key);
  return (
    <>
      <TreeRow
        icon={
          isOpen && childFolders.length > 0 ? (
            <FolderOpen size={12} strokeWidth={1.5} />
          ) : (
            <Folder size={12} strokeWidth={1.5} />
          )
        }
        label={node.name}
        count={countFiles(node)}
        depth={depth}
        active={activePathKey === key}
        onActivate={() => onNavigate(here)}
        expandable={childFolders.length > 0}
        expanded={isOpen}
        onToggle={() => toggleFolder(key)}
      />
      {isOpen &&
        childFolders.map((child) => (
          <TreeNavNode
            key={child.name}
            node={child}
            parent={here}
            depth={depth + 1}
            openFolders={openFolders}
            toggleFolder={toggleFolder}
            activePathKey={activePathKey}
            onNavigate={onNavigate}
          />
        ))}
    </>
  );
}

interface TreeRowProps {
  icon: ReactNode;
  label: ReactNode;
  count?: number;
  depth: number;
  active?: boolean;
  onActivate?: () => void;
  /** Folder rows render a sibling expand/collapse toggle. */
  expandable?: boolean;
  expanded?: boolean;
  onToggle?: () => void;
}

/**
 * One tree row. The expand/collapse toggle and the navigate target are
 * sibling buttons inside a non-interactive container — never nested — so the
 * row stays free of the axe "nested-interactive" violation.
 */
function TreeRow({
  icon,
  label,
  count,
  depth,
  active,
  onActivate,
  expandable,
  expanded,
  onToggle,
}: TreeRowProps) {
  return (
    <div
      style={{ paddingLeft: 8 + depth * 13 }}
      className={cn(
        "flex items-center gap-1.5 rounded-sm pr-2 text-ui transition-colors duration-std ease",
        active ? "bg-accent-tint text-text" : "text-text-2 hover:bg-surface",
      )}
    >
      {expandable ? (
        <button
          type="button"
          aria-label={expanded ? "Collapse folder" : "Expand folder"}
          aria-expanded={expanded}
          onClick={onToggle}
          className="grid h-3.5 w-3.5 shrink-0 place-items-center rounded-[3px] text-text-3 transition-colors duration-std ease hover:text-text focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring"
        >
          {expanded ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
        </button>
      ) : (
        <span className="h-3.5 w-3.5 shrink-0" aria-hidden />
      )}
      <button
        type="button"
        aria-current={active ? "true" : undefined}
        onClick={onActivate}
        className="flex min-w-0 flex-1 items-center gap-1.5 py-1 text-left text-text-2 transition-colors duration-std ease hover:text-text focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-accent-ring aria-[current=true]:text-text"
      >
        <span className="grid place-items-center text-text-3">{icon}</span>
        <span className="flex-1 truncate">{label}</span>
        {count !== undefined && <span className="text-mono tabular-nums text-text-3">{count}</span>}
      </button>
    </div>
  );
}
