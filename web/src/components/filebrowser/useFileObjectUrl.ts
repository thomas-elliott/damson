import { useEffect, useState } from "react";

import { fetchBlob } from "@/api";
import { isImageType, type FileNode } from "@/design-system";

// Raster image extensions we will fetch + render inline. SVG is deliberately
// excluded — rendering attacker-controlled SVG (even via an object URL) is a
// risk the server's nosniff/attachment policy exists to avoid.
const PREVIEW_MIME: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
};

/**
 * Resolve a previewable image file to an object URL by fetching the blob with
 * auth and re-typing it (see api.ts `fetchBlob`). Returns undefined while
 * loading, on error, or for non-previewable files. The URL is revoked when the
 * file changes or the component unmounts.
 */
export function useFileObjectUrl(node: FileNode | null | undefined): string | undefined {
  const [url, setUrl] = useState<string>();

  const objectKey = node?.objectKey;
  const ext = node?.ext ?? "";
  const mime = PREVIEW_MIME[ext];
  const canPreview = !!node && isImageType(node.ftype) && !!objectKey && !!mime;

  useEffect(() => {
    // Not previewable: the previous run's cleanup has already reset the url
    // (and revoked any object URL), so there's nothing to do synchronously.
    if (!canPreview || !objectKey) return;
    let cancelled = false;
    let created: string | undefined;
    fetchBlob(`/api/files/${objectKey}`, mime)
      .then((blob) => {
        if (cancelled) return;
        created = URL.createObjectURL(blob);
        setUrl(created);
      })
      .catch(() => {
        if (!cancelled) setUrl(undefined);
      });
    return () => {
      cancelled = true;
      if (created) URL.revokeObjectURL(created);
      setUrl(undefined);
    };
  }, [canPreview, objectKey, mime]);

  return url;
}

/**
 * Download a file by fetching the blob with auth and triggering a save. The
 * endpoint is bearer-authenticated, so a plain anchor href can't be used.
 */
export async function downloadFile(objectKey: string, filename: string): Promise<void> {
  const blob = await fetchBlob(`/api/files/${objectKey}`);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
