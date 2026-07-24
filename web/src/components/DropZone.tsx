import { createContext, useCallback, useContext, type ReactNode } from "react";
import { useDropzone } from "react-dropzone";

import { useNotifications } from "@/components/notifications/NotificationProvider";
import { useUploadIntent } from "@/components/UploadIntentDialog";
import { DropOverlay, cn } from "@/design-system";
import {
  bundleFilesToZip,
  bundleName,
  hasFolderStructure,
  isArchiveFile,
  topFolderName,
} from "@/lib/bundleUpload";

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
 * Drop-target wrapper for the library grid. A drop anywhere inside hands the
 * files to the notification centre's upload queue, which owns the transfer so
 * it survives navigation. Click-to-open is explicit — children keep their own
 * click behaviour; the Upload toolbar button opens the picker via
 * `useDropZoneOpen()`.
 */
export function DropZone({ children }: { children: ReactNode }) {
  const { startUpload, notify } = useNotifications();
  const { ask, dialog } = useUploadIntent();

  const onDrop = useCallback(
    (files: File[]) => {
      if (files.length === 0) return;

      const folder = hasFolderStructure(files);
      const unambiguous = (files.length === 1 || files.every(isArchiveFile)) && !folder;
      if (unambiguous) {
        startUpload(files);
        return;
      }

      void (async () => {
        const intent = await ask({
          fileCount: files.length,
          totalBytes: files.reduce((sum, f) => sum + f.size, 0),
          folderName: topFolderName(files),
        });
        if (intent === null) return;
        if (intent === "separate") {
          startUpload(files);
          return;
        }
        try {
          const zip = await bundleFilesToZip(files, bundleName(files));
          startUpload([zip]);
        } catch (err) {
          notify({
            tone: "error",
            title: "Could not bundle the upload",
            message: err instanceof Error ? err.message : "zipping failed",
          });
        }
      })();
    },
    [ask, notify, startUpload],
  );

  const { getRootProps, getInputProps, isDragActive, open } = useDropzone({
    onDrop,
    noClick: true,
    noKeyboard: true,
  });

  return (
    <DropZoneContext.Provider value={{ open }}>
      <div
        {...getRootProps()}
        className={cn("relative flex min-h-full flex-1 flex-col outline-hidden")}
      >
        <input {...getInputProps()} />
        {children}
        <DropOverlay active={isDragActive} />
        {dialog}
      </div>
    </DropZoneContext.Provider>
  );
}
