import { extOf, fileTypeFromName, type FileType } from "./fileTypes";
import type { ThumbDescriptor } from "./AssetThumb";

/**
 * Node model for the scoped file browser. A multi-file asset's manifest is
 * grouped into this folder/file tree (see fileTreeBuild — Phase 2). Components
 * in this folder render against these types so the tree source (real manifest
 * vs. fixtures) stays interchangeable.
 */
export interface FileNode {
  type: "file";
  /** Backend asset_files row id, when sourced from a real manifest. */
  id?: number;
  /** Leaf filename, e.g. "SM_Bld_Castle_01.fbx". */
  name: string;
  /** Lowercase extension including the dot, e.g. ".fbx". "" when none. */
  ext: string;
  /** Display type derived from the extension (drives icon/filter/preview). */
  ftype: FileType;
  /** Size in bytes. */
  size: number;
  /** ISO date (yyyy-mm-dd) or undefined when not tracked. */
  modified?: string;
  /** Object key for fetching/previewing via GET /api/files/:objectKey. */
  objectKey?: string;
  /** Backend-rendered preview derivative, served from GET /api/previews/:key. */
  previewObjectKey?: string;
  /** First-class preview state: unsupported | queued | processing | done | failed. */
  previewStatus?: string;
  /** Optional procedural thumbnail descriptor (fixtures/preview). */
  thumb?: ThumbDescriptor;
}

export interface FolderNode {
  type: "folder";
  name: string;
  children: TreeNode[];
}

export type TreeNode = FileNode | FolderNode;

/** A file paired with its folder path, used in flat (search/filter) results. */
export interface FlatFile extends FileNode {
  dir: string[];
}

/** Recursively count the files contained under a node. */
export function countFiles(node: TreeNode): number {
  if (node.type === "file") return 1;
  return node.children.reduce((sum, child) => sum + countFiles(child), 0);
}

/** Join a folder path + filename into a stable "/"-delimited key. */
export function pathKey(dir: string[], name: string): string {
  return [...dir, name].join("/");
}

/** A pack is browsable when its current version holds more than one file. */
export function isBrowsable(fileCount: number): boolean {
  return fileCount > 1;
}

/** A raw manifest entry — the subset of AssetFileView the browser needs. */
export interface ManifestEntry {
  id?: number;
  relative_path: string;
  size_bytes: number;
  object_key?: string;
  detected_type?: string;
  preview_object_key?: string;
  preview_status?: string;
  modified?: string;
}

/**
 * Group a flat manifest into a folder/file tree by splitting each entry's
 * relative path on "/". Folders sort before files; both are sorted by name so
 * the tree is stable across renders regardless of manifest order.
 */
export function buildFileTree(entries: ManifestEntry[]): TreeNode[] {
  const root: FolderNode = { type: "folder", name: "", children: [] };

  for (const entry of entries) {
    const parts = entry.relative_path.split("/").filter(Boolean);
    if (parts.length === 0) continue;
    const fileName = parts[parts.length - 1];
    let cursor = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const segment = parts[i];
      let child = cursor.children.find(
        (n): n is FolderNode => n.type === "folder" && n.name === segment,
      );
      if (!child) {
        child = { type: "folder", name: segment, children: [] };
        cursor.children.push(child);
      }
      cursor = child;
    }
    cursor.children.push({
      type: "file",
      id: entry.id,
      name: fileName,
      ext: extOf(fileName),
      ftype: fileTypeFromName(fileName, entry.detected_type),
      size: entry.size_bytes,
      modified: entry.modified,
      objectKey: entry.object_key,
      previewObjectKey: entry.preview_object_key,
      previewStatus: entry.preview_status,
    });
  }

  sortTree(root.children);
  return root.children;
}

function sortTree(nodes: TreeNode[]): void {
  nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
  for (const node of nodes) {
    if (node.type === "folder") sortTree(node.children);
  }
}

/** Return the children at a folder path (empty array when not found). */
export function nodesAtPath(tree: TreeNode[], path: string[]): TreeNode[] {
  let nodes = tree;
  for (const segment of path) {
    const found = nodes.find((n): n is FolderNode => n.type === "folder" && n.name === segment);
    if (!found) return [];
    nodes = found.children;
  }
  return nodes;
}

/** Flatten every file in the tree, each paired with its folder path. */
export function flattenFiles(tree: TreeNode[], parent: string[] = []): FlatFile[] {
  const out: FlatFile[] = [];
  for (const node of tree) {
    if (node.type === "file") out.push({ ...node, dir: parent });
    else out.push(...flattenFiles(node.children, [...parent, node.name]));
  }
  return out;
}

/** Count files by display type across the whole tree. */
export function typeCounts(tree: TreeNode[]): Partial<Record<FileType, number>> {
  const counts: Partial<Record<FileType, number>> = {};
  for (const file of flattenFiles(tree)) {
    counts[file.ftype] = (counts[file.ftype] ?? 0) + 1;
  }
  return counts;
}
