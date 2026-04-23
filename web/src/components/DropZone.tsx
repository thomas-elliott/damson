import { useCallback, useState } from "react";
import { useDropzone } from "react-dropzone";
import { useQueryClient } from "@tanstack/react-query";

import { uploadFile } from "@/api";
import { queryKeys } from "@/lib/queryKeys";

interface UploadState {
  filename: string;
  progress: number; // 0–1
  error?: string;
  done?: boolean;
}

// Wraps its children with a drop target. Drops anywhere inside fire uploads;
// the target is also clickable. Minimal progress tray shown inline; this is
// first-pass UX — a persistent global tray is post-MVP polish.
export function DropZone({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const [uploads, setUploads] = useState<UploadState[]>([]);

  const onDrop = useCallback(
    (files: File[]) => {
      if (files.length === 0) return;
      setUploads((prev) => [
        ...prev,
        ...files.map((f) => ({ filename: f.name, progress: 0 })),
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
      className={`relative flex min-h-full flex-1 flex-col outline-none ${
        isDragActive ? "bg-accent/5" : ""
      }`}
    >
      <input {...getInputProps()} />
      {children}
      {isDragActive && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center border-2 border-dashed border-accent bg-background/70 text-sm">
          Drop to import
        </div>
      )}
      {uploads.length > 0 && (
        <ul className="absolute bottom-4 right-4 flex max-w-xs flex-col gap-1 rounded-md border border-border bg-elevated p-2 text-xs shadow-lg">
          {uploads.map((u, i) => (
            <li key={`${u.filename}-${i}`} className="flex flex-col gap-0.5">
              <div className="flex items-center justify-between gap-4">
                <span className="truncate font-mono">{u.filename}</span>
                <span className="text-muted">
                  {u.error ? "error" : u.done ? "done" : `${Math.round(u.progress * 100)}%`}
                </span>
              </div>
              {!u.error && (
                <div className="h-1 w-full overflow-hidden rounded bg-panel">
                  <div
                    className="h-full bg-accent transition-all"
                    style={{ width: `${u.progress * 100}%` }}
                  />
                </div>
              )}
              {u.error && <span className="text-licence-violation">{u.error}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
