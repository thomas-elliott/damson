import { useEffect, useMemo, useRef, useState } from "react";
import { Check, FolderTree, Package } from "lucide-react";

import {
  buildFileTree,
  cn,
  countFiles,
  FileBrowserList,
  FileBrowserToolbar,
  FileCard,
  FileDetailPanel,
  FileSelectionBar,
  FileTreeSidebar,
  flattenFiles,
  FolderCard,
  IconButton,
  LoadingState,
  loneRootPath,
  nodesAtPath,
  pathKey,
  type FileBrowserView,
  type FileNode,
  type FileType,
  type FlatFile,
  type FolderNode,
  typeCounts,
} from "@/design-system";
import { useAssetDetail } from "@/hooks/useDataQueries";
import { useCreateSubsetVersion, useRequeueFilePreview } from "@/hooks/useMutations";
import { useKeyboardShortcuts, type KeyBinding } from "@/hooks/useKeyboardShortcuts";
import { useSavedFlash } from "@/hooks/useSavedFlash";

import { FilePreviewPane } from "./FilePreviewPane";
import { downloadFile, useFileThumbUrl } from "./useFileObjectUrl";

// Flat search/filter results are capped so a pathological pack can't render
// tens of thousands of cards. Folder navigation is naturally bounded by the
// tree, so only results mode needs the guard. The cap is surfaced in the UI.
const RESULTS_CAP = 300;

/**
 * The files of one asset: folder tree, file grid or list, and file detail. It
 * fills its container, and below 768px of container width the tree becomes a
 * drawer and the file detail covers the listing.
 */
export function FileBrowser({
  assetId,
  onExit,
  onStepPackage,
}: {
  assetId: number;
  /** Escape at the top of the pack, with nothing open, leaves the browser. */
  onExit?: () => void;
  /** The arrow keys step to the previous or next package while no file is open. */
  onStepPackage?: (delta: -1 | 1) => void;
}) {
  const { data: asset, loading } = useAssetDetail(assetId);

  // Null until the user navigates: the tree's lone-root path stands in.
  const [chosenPath, setPath] = useState<string[] | null>(null);
  const [treeOpen, setTreeOpen] = useState(false);
  const [view, setView] = useState<FileBrowserView>("grid");
  const [query, setQuery] = useState("");
  const [types, setTypes] = useState<Set<FileType>>(() => new Set());
  const [openKey, setOpenKey] = useState<string | undefined>();
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  const tree = useMemo(
    () =>
      buildFileTree(
        (asset?.files ?? []).map((f) => ({
          id: f.id,
          relative_path: f.relative_path,
          size_bytes: f.size_bytes,
          object_key: f.object_key,
          detected_type: f.detected_type,
          preview_object_key: f.preview_object_key,
          preview_status: f.preview_status,
        })),
      ),
    [asset?.files],
  );
  const path = useMemo(() => chosenPath ?? loneRootPath(tree), [chosenPath, tree]);
  const allFiles = useMemo(() => flattenFiles(tree), [tree]);
  const counts = useMemo(() => typeCounts(tree), [tree]);
  const byKey = useMemo(() => {
    const map = new Map<string, FlatFile>();
    for (const file of allFiles) map.set(pathKey(file.dir, file.name), file);
    return map;
  }, [allFiles]);

  const resultsMode = query.trim() !== "" || types.size > 0;

  // The ordered files for the current view — drives the grid/list and the
  // detail-panel prev/next stepping.
  const { folders, listingFiles, truncated } = useMemo(() => {
    if (resultsMode) {
      const q = query.trim().toLowerCase();
      let matches = allFiles;
      if (q) matches = matches.filter((f) => f.name.toLowerCase().includes(q));
      if (types.size > 0) matches = matches.filter((f) => types.has(f.ftype));
      return {
        folders: [] as FolderNode[],
        listingFiles: matches.slice(0, RESULTS_CAP),
        truncated: matches.length > RESULTS_CAP,
      };
    }
    const here = nodesAtPath(tree, path);
    return {
      folders: here.filter((n): n is FolderNode => n.type === "folder"),
      listingFiles: here
        .filter((n): n is FileNode => n.type === "file")
        .map((n) => ({ ...n, dir: path })),
      truncated: false,
    };
  }, [resultsMode, query, types, allFiles, tree, path]);

  const openFile = openKey ? byKey.get(openKey) : undefined;
  const openIndex = openFile
    ? listingFiles.findIndex((f) => pathKey(f.dir, f.name) === openKey)
    : -1;

  const create = useCreateSubsetVersion();
  const requeueFile = useRequeueFilePreview();
  const { saved, flash } = useSavedFlash(2200);

  // ---- Handlers ---------------------------------------------------------

  const navigate = (next: string[]) => {
    setPath(next);
    setQuery("");
    setTypes(new Set());
    setOpenKey(undefined);
    setTreeOpen(false);
  };
  const toggleType = (ftype: FileType) => {
    setOpenKey(undefined);
    setTypes((prev) => {
      const next = new Set(prev);
      if (next.has(ftype)) next.delete(ftype);
      else next.add(ftype);
      return next;
    });
  };
  const clearResults = () => {
    setQuery("");
    setTypes(new Set());
  };
  const open = (file: { name: string; dir: string[] }) => setOpenKey(pathKey(file.dir, file.name));
  const toggleSelect = (file: { name: string; dir: string[] }) =>
    setSelected((prev) => {
      const next = new Set(prev);
      const key = pathKey(file.dir, file.name);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const step = (delta: number) => {
    if (openIndex < 0 || listingFiles.length === 0) return;
    const next = Math.min(listingFiles.length - 1, Math.max(0, openIndex + delta));
    const file = listingFiles[next];
    setOpenKey(pathKey(file.dir, file.name));
  };

  const createSubset = () => {
    if (!asset?.current_version_id) return;
    const fileIds = [...selected]
      .map((key) => byKey.get(key)?.id)
      .filter((id): id is number => id != null);
    if (fileIds.length === 0) return;
    create.mutate(
      { assetId, versionId: asset.current_version_id, file_ids: fileIds },
      {
        onSuccess: () => {
          setSelected(new Set());
          flash();
        },
      },
    );
  };

  // ---- Keyboard: Esc steps up the hierarchy; arrows step open files -----

  const ctx = useRef({
    treeOpen,
    openKey,
    resultsMode,
    path,
    step,
    clearResults,
    onExit,
    onStepPackage,
  });
  useEffect(() => {
    ctx.current = {
      treeOpen,
      openKey,
      resultsMode,
      path,
      step,
      clearResults,
      onExit,
      onStepPackage,
    };
  });
  const bindings = useMemo<KeyBinding[]>(
    () => [
      {
        key: "Escape",
        handler: () => {
          const c = ctx.current;
          if (c.treeOpen) setTreeOpen(false);
          else if (c.openKey) setOpenKey(undefined);
          else if (c.resultsMode) c.clearResults();
          else if (c.path.length > 0) setPath(c.path.slice(0, -1));
          else c.onExit?.();
        },
      },
      {
        key: "ArrowDown",
        handler: (e) => {
          if (ctx.current.openKey) {
            e.preventDefault();
            ctx.current.step(1);
          }
        },
      },
      {
        key: "ArrowUp",
        handler: (e) => {
          if (ctx.current.openKey) {
            e.preventDefault();
            ctx.current.step(-1);
          }
        },
      },
      {
        key: "ArrowRight",
        handler: (e) => {
          if (ctx.current.openKey) {
            e.preventDefault();
            ctx.current.step(1);
          } else if (ctx.current.onStepPackage) {
            e.preventDefault();
            ctx.current.onStepPackage(1);
          }
        },
      },
      {
        key: "ArrowLeft",
        handler: (e) => {
          if (ctx.current.openKey) {
            e.preventDefault();
            ctx.current.step(-1);
          } else if (ctx.current.onStepPackage) {
            e.preventDefault();
            ctx.current.onStepPackage(-1);
          }
        },
      },
    ],
    [],
  );
  useKeyboardShortcuts(bindings);

  if (!asset && loading) {
    return <LoadingState className="h-full" />;
  }
  if (!asset) {
    return (
      <div className="grid h-full place-items-center text-ui text-subtle-foreground">
        Asset not found.
      </div>
    );
  }

  return (
    <div className="@container/files relative flex h-full min-h-0">
      {/* A column from 768px of container width; below it, a drawer over the
          listing that the toolbar's tree button opens. The drawer covers that
          button, so the backdrop and Escape close it. */}
      {treeOpen && (
        <div
          aria-hidden
          onClick={() => setTreeOpen(false)}
          className="absolute inset-0 z-detail bg-black/40 @3xl/files:hidden"
        />
      )}
      <div
        className={cn(
          "w-[260px] shrink-0 border-r border-border bg-card",
          "@max-3xl/files:absolute @max-3xl/files:inset-y-0 @max-3xl/files:left-0 @max-3xl/files:z-detail @max-3xl/files:max-w-full @max-3xl/files:shadow-lg",
          !treeOpen && "@max-3xl/files:hidden",
        )}
      >
        <FileTreeSidebar
          assetName={asset.name}
          assetIcon={<Package size={13} className="text-subtle-foreground" />}
          query={query}
          onQueryChange={setQuery}
          tree={tree}
          totalFiles={allFiles.length}
          path={path}
          onNavigate={navigate}
          typeCounts={counts}
          activeTypes={types}
          onToggleType={toggleType}
        />
      </div>

      <div className="relative flex min-w-0 flex-1 flex-col">
        <FileBrowserToolbar
          leading={
            <IconButton
              aria-label="Folders and search"
              aria-expanded={treeOpen}
              size="sm"
              className="@3xl/files:hidden"
              onClick={() => setTreeOpen((v) => !v)}
            >
              <FolderTree />
            </IconButton>
          }
          assetName={asset.name}
          path={path}
          onNavigate={navigate}
          count={resultsMode ? listingFiles.length : folders.length + listingFiles.length}
          resultsMode={resultsMode}
          query={query}
          onClearResults={clearResults}
          view={view}
          onViewChange={setView}
          activeTypes={[...types]}
          onToggleType={toggleType}
        />

        <div className="min-h-0 flex-1 overflow-auto p-3.5">
          {folders.length === 0 && listingFiles.length === 0 ? (
            <div className="grid h-full place-items-center text-center">
              <div>
                <div className="text-ui text-muted-foreground">
                  {resultsMode
                    ? "No files match"
                    : path.length === 0
                      ? "No files in this pack"
                      : "This folder is empty"}
                </div>
                <div className="mt-1 text-label text-faint-foreground">
                  {resultsMode
                    ? "Try a different search or clear the file-type filter."
                    : path.length === 0
                      ? "This asset has no files to show."
                      : "Navigate up to find files."}
                </div>
              </div>
            </div>
          ) : view === "grid" ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
              {folders.map((folder) => (
                <FolderCard
                  key={`f/${folder.name}`}
                  name={folder.name}
                  count={countFiles(folder)}
                  onOpen={() => navigate([...path, folder.name])}
                />
              ))}
              {listingFiles.map((file) => {
                const key = pathKey(file.dir, file.name);
                return (
                  <BrowserFileCard
                    key={key}
                    file={file}
                    selected={selected.has(key)}
                    open={openKey === key}
                    pathLabel={resultsMode ? file.dir.join("/") : undefined}
                    onOpen={() => open(file)}
                    onSelect={() => toggleSelect(file)}
                  />
                );
              })}
            </div>
          ) : (
            <FileBrowserList
              folders={resultsMode ? [] : folders}
              files={listingFiles}
              showPath={resultsMode}
              selected={selected}
              onOpenFolder={(name) => navigate([...path, name])}
              onOpenFile={(file) => open(file)}
              onSelectFile={(file) => toggleSelect(file)}
            />
          )}
          {truncated && (
            <div className="mt-3 text-center text-label text-faint-foreground">
              Showing the first {RESULTS_CAP} matches — refine your search to narrow the results.
            </div>
          )}
        </div>

        <FileSelectionBar
          count={selected.size}
          onClear={() => setSelected(new Set())}
          onCreateSubset={createSubset}
        />

        {saved && (
          <div className="absolute bottom-4 left-1/2 z-selbar flex -translate-x-1/2 items-center gap-2 rounded-full border border-border-strong bg-popover px-3.5 py-1.5 text-ui text-foreground shadow-lg">
            <Check size={13} className="text-success" /> Subset version created
          </div>
        )}
      </div>

      {openFile && (
        <div className="w-[380px] shrink-0 @max-3xl/files:absolute @max-3xl/files:inset-0 @max-3xl/files:z-detail @max-3xl/files:w-full">
          <FileDetailPanel
            assetName={asset.name}
            node={openFile}
            dir={openFile.dir}
            index={openIndex < 0 ? 0 : openIndex}
            total={listingFiles.length}
            onPrev={() => step(-1)}
            onNext={() => step(1)}
            onClose={() => setOpenKey(undefined)}
            onDownload={() => {
              if (openFile.objectKey) {
                void downloadFile(openFile.objectKey, openFile.name).catch(() => {});
              }
            }}
            previewStatus={openFile.previewStatus}
            onRequeuePreview={
              openFile.id != null && openFile.previewStatus !== "unsupported"
                ? () => requeueFile.mutate({ assetId, fileId: openFile.id! })
                : undefined
            }
            requeuePending={requeueFile.isPending}
            preview={<FilePreviewPane node={openFile} />}
          />
        </div>
      )}
    </div>
  );
}

// BrowserFileCard resolves a file's thumbnail (backend derivative or native
// image) to an authenticated object URL and renders the grid tile. Kept as its
// own component so the per-file blob hook has a stable place to run; files with
// no previewable thumbnail fetch nothing and show their type placeholder.
function BrowserFileCard({
  file,
  selected,
  open: isOpen,
  pathLabel,
  onOpen,
  onSelect,
}: {
  file: FlatFile;
  selected: boolean;
  open: boolean;
  pathLabel?: string;
  onOpen: () => void;
  onSelect: () => void;
}) {
  const imageUrl = useFileThumbUrl(file);
  return (
    <FileCard
      node={file}
      imageUrl={imageUrl}
      selected={selected}
      open={isOpen}
      pathLabel={pathLabel}
      onOpen={onOpen}
      onSelect={onSelect}
    />
  );
}
