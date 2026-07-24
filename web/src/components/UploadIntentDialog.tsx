import { useCallback, useRef, useState } from "react";

import { Button, formatBytes } from "@/design-system";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/design-system/primitives";
import { LARGE_BUNDLE_BYTES, LARGE_BUNDLE_COUNT } from "@/lib/bundleUpload";

export type UploadIntent = "bundle" | "separate";

interface IntentInfo {
  fileCount: number;
  totalBytes: number;
  folderName?: string;
}

export function useUploadIntent() {
  const [open, setOpen] = useState(false);
  const [info, setInfo] = useState<IntentInfo | null>(null);
  const resolverRef = useRef<((value: UploadIntent | null) => void) | null>(null);

  const settle = useCallback((value: UploadIntent | null) => {
    const resolve = resolverRef.current;
    resolverRef.current = null;
    resolve?.(value);
  }, []);

  const ask = useCallback((next: IntentInfo) => {
    resolverRef.current?.(null);
    return new Promise<UploadIntent | null>((resolve) => {
      resolverRef.current = resolve;
      setInfo(next);
      setOpen(true);
    });
  }, []);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      setOpen(next);
      if (!next) settle(null);
    },
    [settle],
  );

  const choose = useCallback(
    (value: UploadIntent) => {
      setOpen(false);
      settle(value);
    },
    [settle],
  );

  const large =
    !!info && (info.totalBytes > LARGE_BUNDLE_BYTES || info.fileCount > LARGE_BUNDLE_COUNT);

  const dialog = info ? (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>How should we handle this upload?</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <p className="text-ui text-text-2">
            You dropped {info.fileCount} files
            {info.folderName ? ` from “${info.folderName}”` : ""} ({formatBytes(info.totalBytes)}).
            Bundle them into a single asset, or upload each file as its own asset?
          </p>
          {large && (
            <p className="mt-2 text-small text-warn">
              This is a large bundle — zipping it in the browser may take a while and use a lot of
              memory. You can still proceed.
            </p>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="ghost" onClick={() => choose("separate")}>
            Upload separately
          </Button>
          <Button variant="primary" onClick={() => choose("bundle")}>
            Bundle into one asset
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ) : null;

  return { ask, dialog };
}
