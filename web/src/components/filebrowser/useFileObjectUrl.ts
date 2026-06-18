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
 * The MIME a file should be re-typed to for inline image preview, or undefined
 * when the file isn't an inline-previewable image. Pure (no React) so the
 * classification is unit-testable on its own. Later preview stages add their
 * own MIME resolvers for the types they handle.
 */
export function previewImageMime(node: FileNode | null | undefined): string | undefined {
  if (!node || !isImageType(node.ftype)) return undefined;
  return PREVIEW_MIME[node.ext ?? ""];
}

/**
 * Resolve an object key to a `blob:` object URL by fetching the bytes with auth
 * and re-typing the Blob to `mime` (see api.ts `fetchBlob`). The general
 * primitive every preview viewer builds on: returns undefined while loading, on
 * error, or when `objectKey`/`mime` is absent, and revokes the URL when its
 * inputs change or the component unmounts.
 */
export function useObjectUrl(
  objectKey: string | undefined,
  mime: string | undefined,
): string | undefined {
  const [url, setUrl] = useState<string>();

  useEffect(() => {
    // Nothing to fetch: the previous run's cleanup already reset the url (and
    // revoked any object URL), so there's nothing to do synchronously.
    if (!objectKey || !mime) return;
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
  }, [objectKey, mime]);

  return url;
}

/**
 * Resolve a previewable image file to an object URL. Returns undefined while
 * loading, on error, or for non-image / non-previewable files.
 */
export function useFileObjectUrl(node: FileNode | null | undefined): string | undefined {
  return useObjectUrl(node?.objectKey, previewImageMime(node));
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
