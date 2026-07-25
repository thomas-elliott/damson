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
 * Resolve an authenticated endpoint to a `blob:` object URL by fetching the
 * bytes (see api.ts `fetchBlob`) and optionally re-typing the Blob to `mime`.
 * The general primitive every preview viewer builds on: returns undefined while
 * loading, on error, or when `url` is absent, and revokes the URL when its
 * inputs change or the component unmounts.
 */
export function useBlobUrl(url: string | undefined, mime?: string): string | undefined {
  const [objectUrl, setObjectUrl] = useState<string>();

  useEffect(() => {
    // Nothing to fetch: the previous run's cleanup already reset the url (and
    // revoked any object URL), so there's nothing to do synchronously.
    if (!url) return;
    let cancelled = false;
    let created: string | undefined;
    fetchBlob(url, mime)
      .then((blob) => {
        if (cancelled) return;
        created = URL.createObjectURL(blob);
        setObjectUrl(created);
      })
      .catch(() => {
        if (!cancelled) setObjectUrl(undefined);
      });
    return () => {
      cancelled = true;
      if (created) URL.revokeObjectURL(created);
      setObjectUrl(undefined);
    };
  }, [url, mime]);

  return objectUrl;
}

/** One authenticated endpoint to resolve, with the MIME to re-type it to. */
export interface BlobSource {
  url: string;
  mime?: string;
}

/**
 * Resolve several authenticated endpoints to `blob:` object URLs, keyed by URL.
 * Entries appear as they load rather than all at once, so a viewer can paint
 * the first image while the rest arrive; every URL is revoked when the set
 * changes or the component unmounts. Use this over a `useBlobUrl` per item when
 * the items must stay loaded together (e.g. a carousel that steps between them).
 */
export function useBlobUrls(sources: BlobSource[]): Record<string, string> {
  // Keying on the contents rather than the array identity keeps a refetch that
  // returns the same images from re-downloading them. The resolved map is
  // tagged with the key it belongs to, so a stale set is discarded on read
  // rather than cleared by a second state write.
  const key = sources.map((s) => `${s.url}|${s.mime ?? ""}`).join("\n");
  const [resolved, setResolved] = useState<{ key: string; urls: Record<string, string> }>({
    key: "",
    urls: EMPTY_URLS,
  });

  useEffect(() => {
    const entries = key
      .split("\n")
      .filter(Boolean)
      .map((line) => {
        const [url, mime] = line.split("|");
        return { url, mime: mime || undefined };
      });
    if (entries.length === 0) return;
    let cancelled = false;
    const created: string[] = [];
    for (const entry of entries) {
      fetchBlob(entry.url, entry.mime)
        .then((blob) => {
          if (cancelled) return;
          const objectUrl = URL.createObjectURL(blob);
          created.push(objectUrl);
          setResolved((prev) => ({
            key,
            urls:
              prev.key === key
                ? { ...prev.urls, [entry.url]: objectUrl }
                : { [entry.url]: objectUrl },
          }));
        })
        .catch(() => {});
    }
    return () => {
      cancelled = true;
      created.forEach(URL.revokeObjectURL);
    };
  }, [key]);

  return resolved.key === key ? resolved.urls : EMPTY_URLS;
}

const EMPTY_URLS: Record<string, string> = {};

/**
 * Resolve a file's bytes (GET /api/files/:objectKey) to an object URL, re-typed
 * to `mime`. Undefined while loading, on error, or when either is absent.
 */
export function useObjectUrl(
  objectKey: string | undefined,
  mime: string | undefined,
): string | undefined {
  return useBlobUrl(objectKey && mime ? `/api/files/${objectKey}` : undefined, mime);
}

/**
 * Resolve a previewable image file to an object URL. Returns undefined while
 * loading, on error, or for non-image / non-previewable files.
 */
export function useFileObjectUrl(node: FileNode | null | undefined): string | undefined {
  return useObjectUrl(node?.objectKey, previewImageMime(node));
}

/**
 * Resolve a file's best grid thumbnail to an object URL: a backend-rendered
 * derivative (GET /api/previews/:key, for pro/HDR images and SVG raster) when
 * one exists, otherwise the file's own bytes for natively-previewable images.
 * Undefined for everything else (the tile shows its type placeholder). Both
 * endpoints are bearer-authenticated, so the bytes go through useBlobUrl rather
 * than a bare <img src>. Only one branch fetches for any given file — derivative
 * and native are mutually exclusive — so this never double-loads.
 */
export function useFileThumbUrl(node: FileNode | null | undefined): string | undefined {
  const derivative = useBlobUrl(
    node?.previewObjectKey ? `/api/previews/${node.previewObjectKey}` : undefined,
  );
  const native = useFileObjectUrl(node);
  return derivative ?? native;
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
