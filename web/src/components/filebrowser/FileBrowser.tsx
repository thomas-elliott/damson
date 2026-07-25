import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Package } from "lucide-react";

import {
  FileBrowserList,
  FileBrowserToolbar,
  FileCard,
  FileDetailPanel,
  FileSelectionBar,
  FileTreeSidebar,
  FolderCard,
  buildFileTree,
  countFiles,
  flattenFiles,
  nodesAtPath,
  pathKey,
  typeCounts,
  type FileBrowserView,
  type FileNode,
  type FlatFile,
  type FolderNode,
  type FileType,
} from "@/design-system";
import { useAssetDetail } from "@/hooks/useDataQueries";
import {
  useCreateSubsetVersion,
  useReExtractVersion,
  useRequeueAssetPreviews,
  useRequeueFilePreview,
} from "@/hooks/useMutations";
import { useKeyboardShortcuts, type KeyBinding } from "@/hooks/useKeyboardShortcuts";
import { useSavedFlash } from "@/hooks/useSavedFlash";

import { FilePreviewPane } from "./FilePreviewPane";
import { downloadFile, useFileThumbUrl } from "./useFileObjectUrl";

// Flat search/filter results are capped so a pathological pack can't render
// tens of thousands of cards. Folder navigation is naturally bounded by the
// tree, so only results mode needs the guard. The cap is surfaced in the UI.
const RESULTS_CAP = 300;

export function FileBrowser({ assetId, onExit }: { assetId: number; onExit: () => void }) {
  const { data: asset, loading } = useAssetDetail(assetId);

  const [path, setPath] = useState<string[]>([]);
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
  const reextract = useReExtractVersion();
  const requeuePreviews = useRequeueAssetPreviews();
  const { saved, flash } = useSavedFlash(2200);

  // ---- Handlers ---------------------------------------------------------

  const navigate = (next: string[]) => {
    setPath(next);
    setQuery("");
    setTypes(new Set());
    setOpenKey(undefined);
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

  const ctx = useRef({ openKey, resultsMode, path, step, clearResults, onExit });
  useEffect(() => {
    ctx.current = { openKey, resultsMode, path, step, clearResults, onExit };
  });
  const bindings = useMemo<KeyBinding[]>(
    () => [
      {
        key: "Escape",
        handler: () => {
          const c = ctx.current;
          if (c.openKey) setOpenKey(undefined);
          else if (c.resultsMode) c.clearResults();
          else if (c.path.length > 0) setPath(c.path.slice(0, -1));
          else c.onExit();
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
          }
        },
      },
      {
        key: "ArrowLeft",
        handler: (e) => {
          if (ctx.current.openKey) {
            e.preventDefault();
            ctx.current.step(-1);
          }
        },
      },
    ],
    [],
  );
  useKeyboardShortcuts(bindings);

  if (!asset && loading) {
    return <div className="grid h-full place-items-center text-ui text-text-3">Loading files…</div>;
  }
  if (!asset) {
    return (
      <div className="grid h-full place-items-center text-ui text-text-3">Asset not found.</div>
    );
  }

  // Download/re-extract target the latest version (by import date) that still
  // retains its original archive — the current version may be a subset or an
  // earlier re-extract that no longer has one. Download works for any retained
  // package (unitypackage or zip); re-extract is Unity-only.
  const byNewestImport = (a: { import_date: string }, b: { import_date: string }) =>
    b.import_date.localeCompare(a.import_date);
  const downloadSource = [...asset.versions]
    .filter((v) => v.has_original && v.original_package_object_key)
    .sort(byNewestImport)[0];
  const downloadKey = downloadSource?.original_package_object_key;
  const downloadOriginal =
    downloadSource && downloadKey
      ? () => {
          const ext = downloadSource.package_kind === "zip" ? "zip" : "unitypackage";
          void downloadFile(downloadKey, `${asset.name}.${ext}`).catch(() => {});
        }
      : undefined;

  // Re-extract rebuilds the file manifest from the retained original — only
  // possible for Unity packages with a retained archive. It doesn't queue
  // previews itself, so chain a requeue once the new version lands.
  const reExtractSource = [...asset.versions]
    .filter((v) => v.package_kind === "unitypackage" && v.has_original)
    .sort(byNewestImport)[0];
  const reExtract = reExtractSource
    ? () =>
        reextract.mutate(
          { assetId, versionId: reExtractSource.id },
          { onSuccess: () => requeuePreviews.mutate(assetId) },
        )
    : undefined;

  return (
    <div className="flex h-full min-h-0">
      <div className="w-[260px] shrink-0 border-r border-line bg-panel">
        <FileTreeSidebar
          assetName={asset.name}
          assetIcon={<Package size={13} className="text-text-3" />}
          onExit={onExit}
          query={query}
          onQueryChange={setQuery}
          tree={tree}
          totalFiles={allFiles.length}
          path={path}
          onNavigate={navigate}
          openFileKey={openKey}
          onOpenFile={(node, dir) => open({ name: node.name, dir })}
          typeCounts={counts}
          activeTypes={types}
          onToggleType={toggleType}
        />
      </div>

      <div className="relative flex min-w-0 flex-1 flex-col">
        <FileBrowserToolbar
          assetName={asset.name}
          path={path}
          onNavigate={navigate}
          onExit={onExit}
          count={resultsMode ? listingFiles.length : folders.length + listingFiles.length}
          resultsMode={resultsMode}
          query={query}
          onClearResults={clearResults}
          view={view}
          onViewChange={setView}
          activeTypes={[...types]}
          onToggleType={toggleType}
          onDownloadOriginal={downloadOriginal}
          onReExtract={reExtract}
          reExtractPending={reextract.isPending || requeuePreviews.isPending}
        />

        <div className="min-h-0 flex-1 overflow-auto p-3.5">
          {folders.length === 0 && listingFiles.length === 0 ? (
            <div className="grid h-full place-items-center text-center">
              <div>
                <div className="text-ui text-text-2">
                  {resultsMode
                    ? "No files match"
                    : path.length === 0
                      ? "No files in this pack"
                      : "This folder is empty"}
                </div>
                <div className="mt-1 text-small text-text-4">
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
            <div className="mt-3 text-center text-small text-text-4">
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
          <div className="absolute bottom-4 left-1/2 z-selbar flex -translate-x-1/2 items-center gap-2 rounded-pill border border-line-2 bg-surface-2 px-3.5 py-1.5 text-ui text-text shadow-elevated">
            <Check size={13} className="text-ok" /> Subset version created
          </div>
        )}
      </div>

      {openFile && (
        <div className="w-[380px] shrink-0">
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
