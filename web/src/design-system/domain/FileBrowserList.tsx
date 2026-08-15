import { Folder } from "lucide-react";

import { cn } from "@/design-system/utils";
import { formatBytes } from "@/design-system/utils/format";

import { FileGlyph } from "./FileGlyph";
import { fileTypeMeta } from "./fileTypes";
import { countFiles, type FlatFile, type FolderNode } from "./fileTree";

export interface FileBrowserListProps {
  /** Folders in the current directory (hidden in results mode). */
  folders?: FolderNode[];
  /** Files to list — each carries its folder path for results mode. */
  files: FlatFile[];
  /** Show the folder-path column (search/filter results mode). */
  showPath?: boolean;
  /** Path keys of selected files. */
  selected?: Set<string>;
  onOpenFolder?: (name: string) => void;
  onOpenFile?: (file: FlatFile) => void;
  onSelectFile?: (file: FlatFile) => void;
  className?: string;
}

/** Dense list/table view of a folder's contents. */
export function FileBrowserList({
  folders = [],
  files,
  showPath,
  selected,
  onOpenFolder,
  onOpenFile,
  onSelectFile,
  className,
}: FileBrowserListProps) {
  return (
    <table className={cn("w-full border-collapse", className)}>
      <thead className="sticky top-0 z-10 bg-background">
        <tr className="text-table-header text-subtle-foreground">
          <th className="px-2.5 py-1.5 text-left">Name</th>
          <th className="px-2.5 py-1.5 text-left">Type</th>
          {showPath && <th className="px-2.5 py-1.5 text-left">Folder</th>}
          <th className="px-2.5 py-1.5 text-right">Size</th>
          <th className="px-2.5 py-1.5 text-left">Modified</th>
        </tr>
      </thead>
      <tbody>
        {folders.map((folder) => (
          <tr
            key={`f/${folder.name}`}
            onClick={() => onOpenFolder?.(folder.name)}
            className="cursor-pointer text-list-row text-muted-foreground transition-colors duration-fast ease-standard hover:bg-card hover:text-foreground"
          >
            <td className="px-2.5 py-1.5">
              <div className="flex items-center gap-2">
                <Folder size={14} strokeWidth={1.5} className="shrink-0 text-subtle-foreground" />
                <span className="truncate font-medium text-foreground">{folder.name}</span>
              </div>
            </td>
            <td className="px-2.5 py-1.5">Folder</td>
            {showPath && <td className="px-2.5 py-1.5" />}
            <td className="px-2.5 py-1.5 text-right font-mono tabular-nums">
              {countFiles(folder)} items
            </td>
            <td className="px-2.5 py-1.5" />
          </tr>
        ))}
        {files.map((file) => {
          const key = [...file.dir, file.name].join("/");
          const isSelected = selected?.has(key) ?? false;
          return (
            <tr
              key={`x/${key}`}
              aria-selected={isSelected || undefined}
              onClick={(e) => {
                if (e.shiftKey || e.metaKey || e.ctrlKey) onSelectFile?.(file);
                else onOpenFile?.(file);
              }}
              className={cn(
                "cursor-pointer text-list-row text-muted-foreground transition-colors duration-fast ease-standard",
                "hover:bg-card hover:text-foreground",
                "aria-selected:bg-primary-soft aria-selected:text-foreground",
              )}
            >
              <td className="px-2.5 py-1.5">
                <div className="flex items-center gap-2">
                  <FileGlyph
                    ftype={file.ftype}
                    size={14}
                    strokeWidth={1.5}
                    className="shrink-0 text-subtle-foreground"
                  />
                  <span className="truncate font-medium text-foreground">{file.name}</span>
                </div>
              </td>
              <td className="px-2.5 py-1.5">{fileTypeMeta(file.ftype).label}</td>
              {showPath && (
                <td className="px-2.5 py-1.5 font-mono text-label text-faint-foreground">
                  {file.dir.join("/") || "—"}
                </td>
              )}
              <td className="px-2.5 py-1.5 text-right font-mono tabular-nums">
                {formatBytes(file.size)}
              </td>
              <td className="px-2.5 py-1.5 font-mono text-label">{file.modified ?? "—"}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
