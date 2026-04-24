import { useCallback, useState } from "react";
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

/**
 * Drop-target wrapper for the library grid. Drops anywhere inside fire
 * uploads; the target is also clickable. A compact progress tray shows
 * per-file status in the bottom-right while uploads are in flight.
 */
export function DropZone({ children }: { children: React.ReactNode }) {
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

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    noClick: false,
    noKeyboard: false,
  });

  return (
    <div
      {...getRootProps()}
      className={cn(
        "relative flex min-h-full flex-1 flex-col outline-none",
        "focus-visible:ring-2 focus-visible:ring-accent-ring",
      )}
    >
      <input {...getInputProps()} />
      {children}
      <DropOverlay active={isDragActive} />
      {uploads.length > 0 && (
        <ul className="absolute bottom-4 right-4 z-selbar flex w-[280px] flex-col gap-2 rounded-md border border-line bg-panel p-2.5 shadow-elevated">
          {uploads.map((u, i) => (
            <li key={`${u.filename}-${i}`} className="flex flex-col gap-1">
              <div className="flex items-center justify-between gap-3 text-xs-plus">
                <span className="truncate font-medium text-text">
                  {u.filename}
                </span>
                <span
                  className={cn(
                    "flex items-center gap-1 font-mono text-xs tabular-nums",
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
                <span className="text-xs text-err">{u.error}</span>
              )}
              <span className="font-mono text-2xs text-text-3">
                {formatBytes(u.size)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
