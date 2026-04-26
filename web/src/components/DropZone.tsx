import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import { useDropzone } from "react-dropzone";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, Check, UploadCloud } from "lucide-react";

import { uploadFile } from "@/api";
import {
  DropOverlay,
  ProgressBar,
  cn,
  formatBytes,
} from "@/design-system";
import { queryKeys } from "@/lib/queryKeys";

interface UploadState {
  filename: string;
  size: number;
  progress: number; // 0–1
  error?: string;
  done?: boolean;
}

interface DropZoneContextValue {
  /** Trigger the OS file picker. Wired to the Upload toolbar button. */
  open: () => void;
}

const DropZoneContext = createContext<DropZoneContextValue | null>(null);

/** Consumer hook for the Upload button in the toolbar. Returns null outside
 * a DropZone, so callers can feature-detect. */
export function useDropZoneOpen() {
  return useContext(DropZoneContext);
}

/**
 * Drop-target wrapper for the library grid. Drops anywhere inside fire
 * uploads. Click-to-open is explicit — children (grid cards, detail panel,
 * etc.) keep their own click behaviour; the Upload toolbar button opens the
 * picker via `useDropZoneOpen()`.
 */
export function DropZone({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [uploads, setUploads] = useState<UploadState[]>([]);

  const onDrop = useCallback(
    (files: File[]) => {
      if (files.length === 0) return;
      setUploads((prev) => [
        ...prev,
        ...files.map((f) => ({ filename: f.name, size: f.size, progress: 0 })),
      ]);
      files.forEach(async (file, idx) => {
        const myIndex = uploads.length + idx;
        try {
          await uploadFile("/api/imports", file, (p) => {
            setUploads((prev) => {
              const next = [...prev];
              if (next[myIndex]) next[myIndex] = { ...next[myIndex], progress: p };
              return next;
            });
          });
          setUploads((prev) => {
            const next = [...prev];
            if (next[myIndex])
              next[myIndex] = { ...next[myIndex], progress: 1, done: true };
            return next;
          });
          void queryClient.invalidateQueries({ queryKey: queryKeys.assets() });
        } catch (err) {
          setUploads((prev) => {
            const next = [...prev];
            if (next[myIndex])
              next[myIndex] = {
                ...next[myIndex],
                error: err instanceof Error ? err.message : "upload failed",
              };
            return next;
          });
        }
      });
    },
    [queryClient, uploads.length],
  );

  // noClick: children handle their own clicks (AssetCard etc.). The
  // toolbar Upload button calls `open` from context to trigger the picker
  // explicitly.
  const { getRootProps, getInputProps, isDragActive, open } = useDropzone({
    onDrop,
    noClick: true,
    noKeyboard: true,
  });

  return (
    <DropZoneContext.Provider value={{ open }}>
      <div
        {...getRootProps()}
        className={cn(
          "relative flex min-h-full flex-1 flex-col outline-hidden",
        )}
      >
        <input {...getInputProps()} />
        {children}
        <DropOverlay active={isDragActive} />
        {uploads.length > 0 && (
          <ul className="absolute bottom-4 right-4 z-selbar flex w-[280px] flex-col gap-2 rounded-md border border-line bg-panel p-2.5 shadow-elevated">
            {uploads.map((u, i) => (
              <li key={`${u.filename}-${i}`} className="flex flex-col gap-1">
                <div className="flex items-center justify-between gap-3 text-small">
                  <span className="truncate font-medium text-text">
                    {u.filename}
                  </span>
                  <span
                    className={cn(
                      "flex items-center gap-1 text-mono tabular-nums",
                      u.error ? "text-err" : u.done ? "text-ok" : "text-text-3",
                    )}
                  >
                    {u.error ? (
                      <AlertCircle size={12} />
                    ) : u.done ? (
                      <Check size={12} />
                    ) : (
                      <UploadCloud size={12} />
                    )}
                    {u.error
                      ? "error"
                      : u.done
                        ? "done"
                        : `${Math.round(u.progress * 100)}%`}
                  </span>
                </div>
                {!u.error && (
                  <ProgressBar
                    value={u.progress * 100}
                    aria-label={`${u.filename} ${Math.round(u.progress * 100)}%`}
                    tone={u.done ? "ok" : "accent"}
                  />
                )}
                {u.error && (
                  <span className="text-small text-err">{u.error}</span>
                )}
                <span className="text-mono text-text-3">
                  {formatBytes(u.size)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </DropZoneContext.Provider>
  );
}
