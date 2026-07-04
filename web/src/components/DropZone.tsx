import { createContext, useCallback, useContext, type ReactNode } from "react";
import { useDropzone } from "react-dropzone";

import { useNotifications } from "@/components/notifications/NotificationProvider";
import { DropOverlay, cn } from "@/design-system";

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
  const { startUpload } = useNotifications();

  const onDrop = useCallback(
    (files: File[]) => {
      if (files.length === 0) return;
      startUpload(files);
    },
    [startUpload],
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
      </div>
    </DropZoneContext.Provider>
  );
}
