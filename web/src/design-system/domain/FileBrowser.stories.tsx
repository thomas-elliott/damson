import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import { FileBrowserList } from "./FileBrowserList";
import { FileBrowserToolbar, type FileBrowserView } from "./FileBrowserToolbar";
import { FileCard, FolderCard } from "./FileCard";
import { FileDetailPanel } from "./FileDetailPanel";
import { FileSelectionBar } from "./FileSelectionBar";
import { FilePlaceholder, FileThumb } from "./FileThumb";
import { FileTreeSidebar } from "./FileTreeSidebar";
import { FileGlyph } from "./FileGlyph";
import { FILE_TYPE_META, type FileType } from "./fileTypes";
import { countFiles, pathKey, type FileNode, type FlatFile, type TreeNode } from "./fileTree";

const meta: Meta = {
  title: "Domain/FileBrowser",
};
export default meta;
type Story = StoryObj;

// ---- Fixtures -----------------------------------------------------------

function f(
  name: string,
  ftype: FileType,
  size: number,
  modified?: string,
  thumb?: FileNode["thumb"],
): FileNode {
  const ext = (name.match(/\.[a-z0-9]+$/i)?.[0] ?? "").toLowerCase();
  return { type: "file", name, ext, ftype, size, modified, thumb };
}

const TREE: TreeNode[] = [
  {
    type: "folder",
    name: "Assets",
    children: [
      {
        type: "folder",
        name: "Prefabs",
        children: [
          f("SM_Bld_Castle_01.prefab", "prefab", 41_000, "2026-03-02"),
          f("SM_Bld_House_02.prefab", "prefab", 22_400, "2026-03-04"),
          f("SM_Env_Tree_01.prefab", "prefab", 18_900, "2026-02-28"),
        ],
      },
      {
        type: "folder",
        name: "Textures",
        children: [
          f("PolygonFantasy_01.png", "texture", 2_400_000, "2026-02-20", {
            kind: "sprite",
            palette: ["#7d9266", "#b8a375", "#5a6b52", "#d4c28b"],
            pattern: "landscape",
          }),
          f("PolygonFantasy_02.png", "texture", 1_800_000, "2026-02-22", {
            kind: "sprite",
            palette: ["#3a3147", "#6b5477", "#9b8aa8"],
            pattern: "tiles",
          }),
        ],
      },
      {
        type: "folder",
        name: "Scripts",
        children: [
          f("DemoCameraController.cs", "script", 8_200, "2026-02-15"),
          f("DayNight.cs", "script", 4_100, "2026-02-16"),
        ],
      },
      f("README.md", "doc", 4_000, "2026-02-10"),
      f("LICENSE.txt", "doc", 2_000, "2026-01-30"),
    ],
  },
];

function flatten(tree: TreeNode[], dir: string[] = []): FlatFile[] {
  const out: FlatFile[] = [];
  for (const node of tree) {
    if (node.type === "file") out.push({ ...node, dir });
    else out.push(...flatten(node.children, [...dir, node.name]));
  }
  return out;
}

const ALL_FILES = flatten(TREE);
const TOTAL = ALL_FILES.length;

function typeCounts(files: FlatFile[]): Partial<Record<FileType, number>> {
  const counts: Partial<Record<FileType, number>> = {};
  for (const file of files) counts[file.ftype] = (counts[file.ftype] ?? 0) + 1;
  return counts;
}

// ---- Stories ------------------------------------------------------------

export const Glyphs: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-4 gap-3">
      {(Object.keys(FILE_TYPE_META) as FileType[]).map((ftype) => (
        <div key={ftype} className="flex items-center gap-2 text-ui text-text-2">
          <FileGlyph ftype={ftype} size={16} strokeWidth={1.6} className="text-text-3" />
          {FILE_TYPE_META[ftype].label}
        </div>
      ))}
    </div>
  ),
};

export const Thumbnails: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-4 gap-3">
      {[
        ALL_FILES.find((x) => x.ftype === "texture")!,
        ALL_FILES.find((x) => x.ftype === "prefab")!,
        ALL_FILES.find((x) => x.ftype === "script")!,
        ALL_FILES.find((x) => x.ftype === "doc")!,
      ].map((node) => (
        <div key={node.name} className="flex flex-col gap-1.5">
          <div className="aspect-square overflow-hidden rounded-md border border-line">
            {node.thumb ? <FileThumb node={node} /> : <FilePlaceholder node={node} />}
          </div>
          <div className="text-mono text-text-3">{node.ext || "file"}</div>
        </div>
      ))}
    </div>
  ),
};

export const Cards: Story = {
  parameters: { layout: "padded" },
  render: function CardsStory() {
    const [selected, setSelected] = useState<Set<string>>(new Set());
    const files = ALL_FILES.filter((x) => x.dir.join("/").startsWith("Assets"));
    const toggle = (key: string) =>
      setSelected((prev) => {
        const next = new Set(prev);
        if (next.has(key)) next.delete(key);
        else next.add(key);
        return next;
      });
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
        <FolderCard name="Prefabs" count={3} />
        <FolderCard name="Textures" count={2} />
        {files.slice(0, 6).map((node) => {
          const key = pathKey(node.dir, node.name);
          return (
            <FileCard
              key={key}
              node={node}
              selected={selected.has(key)}
              onSelect={() => toggle(key)}
            />
          );
        })}
      </div>
    );
  },
};

export const ListView: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <FileBrowserList
      folders={[
        TREE[0].type === "folder"
          ? (TREE[0].children.find((c) => c.type === "folder" && c.name === "Prefabs") as never)
          : (undefined as never),
      ].filter(Boolean)}
      files={ALL_FILES.slice(0, 6)}
      showPath
    />
  ),
};

export const TreeSidebar: Story = {
  parameters: { layout: "fullscreen" },
  render: function TreeSidebarStory() {
    const [query, setQuery] = useState("");
    const [path, setPath] = useState<string[]>(["Assets", "Prefabs"]);
    const [types, setTypes] = useState<Set<FileType>>(new Set());
    const toggleType = (ftype: FileType) =>
      setTypes((prev) => {
        const next = new Set(prev);
        if (next.has(ftype)) next.delete(ftype);
        else next.add(ftype);
        return next;
      });
    return (
      <div className="h-[560px] w-[280px] border-r border-line bg-panel">
        <FileTreeSidebar
          assetName="PolygonFantasyKingdom"
          query={query}
          onQueryChange={setQuery}
          tree={TREE}
          totalFiles={TOTAL}
          path={path}
          onNavigate={setPath}
          onOpenFile={(_, fileDir) => setPath(fileDir)}
          typeCounts={typeCounts(ALL_FILES)}
          activeTypes={types}
          onToggleType={toggleType}
        />
      </div>
    );
  },
};

export const BrowserToolbar: Story = {
  parameters: { layout: "fullscreen" },
  render: function ToolbarStory() {
    const [path, setPath] = useState<string[]>(["Assets", "Prefabs"]);
    const [view, setView] = useState<FileBrowserView>("grid");
    const [types, setTypes] = useState<FileType[]>(["texture"]);
    return (
      <FileBrowserToolbar
        assetName="PolygonFantasyKingdom"
        path={path}
        onNavigate={setPath}
        count={3}
        view={view}
        onViewChange={setView}
        activeTypes={types}
        onToggleType={(t) => setTypes((prev) => prev.filter((x) => x !== t))}
      />
    );
  },
};

export const DetailPanel: Story = {
  parameters: { layout: "fullscreen" },
  render: function DetailPanelStory() {
    const files = ALL_FILES;
    const [index, setIndex] = useState(files.findIndex((x) => x.ftype === "texture"));
    const node = files[index];
    return (
      <div className="h-[600px] w-[420px]">
        <FileDetailPanel
          assetName="PolygonFantasyKingdom"
          node={node}
          dir={node.dir}
          index={index}
          total={files.length}
          onPrev={() => setIndex((i) => Math.max(0, i - 1))}
          onNext={() => setIndex((i) => Math.min(files.length - 1, i + 1))}
          onDownload={() => {}}
          onAddToSubset={() => {}}
        />
      </div>
    );
  },
};

export const SelectionBar: Story = {
  parameters: { layout: "fullscreen" },
  render: function SelectionBarStory() {
    const [count, setCount] = useState(3);
    return (
      <div className="relative h-[200px] bg-bg">
        <FileSelectionBar
          count={count}
          onClear={() => setCount(0)}
          onCreateSubset={() => {}}
          onNewSubset={() => {}}
          dragHint="or drag onto a subset"
        />
      </div>
    );
  },
};

export const FullLayout: Story = {
  parameters: { layout: "fullscreen" },
  render: function FullLayoutStory() {
    const [query, setQuery] = useState("");
    const [path, setPath] = useState<string[]>(["Assets"]);
    const [view, setView] = useState<FileBrowserView>("grid");
    const [types, setTypes] = useState<Set<FileType>>(new Set());
    const [openKey, setOpenKey] = useState<string | undefined>();
    const [selected, setSelected] = useState<Set<string>>(new Set());

    const here = nodesAtPath(TREE, path);
    const folders = here.filter(
      (n): n is Extract<TreeNode, { type: "folder" }> => n.type === "folder",
    );
    const fileNodes = here.filter((n): n is FileNode => n.type === "file");
    const openNode = ALL_FILES.find((x) => pathKey(x.dir, x.name) === openKey);

    const toggleType = (ftype: FileType) =>
      setTypes((prev) => {
        const next = new Set(prev);
        if (next.has(ftype)) next.delete(ftype);
        else next.add(ftype);
        return next;
      });
    const toggleSelect = (key: string) =>
      setSelected((prev) => {
        const next = new Set(prev);
        if (next.has(key)) next.delete(key);
        else next.add(key);
        return next;
      });

    return (
      <div className="flex h-[640px] bg-bg text-text">
        <div className="w-[260px] border-r border-line bg-panel">
          <FileTreeSidebar
            assetName="PolygonFantasyKingdom"
            query={query}
            onQueryChange={setQuery}
            tree={TREE}
            totalFiles={TOTAL}
            path={path}
            onNavigate={setPath}
            openFileKey={openKey}
            onOpenFile={(node, fileDir) => {
              setPath(fileDir);
              setOpenKey(pathKey(fileDir, node.name));
            }}
            typeCounts={typeCounts(ALL_FILES)}
            activeTypes={types}
            onToggleType={toggleType}
          />
        </div>
        <div className="relative flex min-w-0 flex-1 flex-col">
          <FileBrowserToolbar
            assetName="PolygonFantasyKingdom"
            path={path}
            onNavigate={setPath}
            count={here.length}
            view={view}
            onViewChange={setView}
            activeTypes={[...types]}
            onToggleType={toggleType}
          />
          <div className="min-h-0 flex-1 overflow-auto p-3.5">
            {view === "grid" ? (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
                {folders.map((folder) => (
                  <FolderCard
                    key={folder.name}
                    name={folder.name}
                    count={countFiles(folder)}
                    onOpen={() => setPath([...path, folder.name])}
                  />
                ))}
                {fileNodes.map((node) => {
                  const key = pathKey(path, node.name);
                  return (
                    <FileCard
                      key={key}
                      node={node}
                      selected={selected.has(key)}
                      open={openKey === key}
                      onOpen={() => setOpenKey(key)}
                      onSelect={() => toggleSelect(key)}
                    />
                  );
                })}
              </div>
            ) : (
              <FileBrowserList
                folders={folders}
                files={fileNodes.map((n) => ({ ...n, dir: path }))}
                selected={selected}
                onOpenFolder={(name) => setPath([...path, name])}
                onOpenFile={(file) => setOpenKey(pathKey(file.dir, file.name))}
                onSelectFile={(file) => toggleSelect(pathKey(file.dir, file.name))}
              />
            )}
          </div>
          <FileSelectionBar
            count={selected.size}
            onClear={() => setSelected(new Set())}
            onCreateSubset={() => {}}
            onNewSubset={() => {}}
            dragHint="or drag onto a subset"
          />
        </div>
        {openNode && (
          <div className="w-[380px]">
            <FileDetailPanel
              assetName="PolygonFantasyKingdom"
              node={openNode}
              dir={openNode.dir}
              index={ALL_FILES.indexOf(openNode)}
              total={ALL_FILES.length}
              onClose={() => setOpenKey(undefined)}
              onDownload={() => {}}
              onAddToSubset={() => {}}
            />
          </div>
        )}
      </div>
    );
  },
};

// Local copy of the Phase-2 helper so the story is self-contained.
function nodesAtPath(tree: TreeNode[], path: string[]): TreeNode[] {
  let nodes = tree;
  for (const seg of path) {
    const found = nodes.find((n) => n.type === "folder" && n.name === seg);
    if (!found || found.type !== "folder") return [];
    nodes = found.children;
  }
  return nodes;
}
