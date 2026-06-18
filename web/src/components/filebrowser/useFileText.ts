import { useEffect, useState } from "react";

import { fetchBlob } from "@/api";

// Above this we read only the head and flag the preview as truncated, so a
// pathological multi-MB text file can't lock up highlighting or the DOM.
const MAX_TEXT_BYTES = 512 * 1024;

export interface FileText {
  text?: string;
  truncated: boolean;
  loading: boolean;
  error: boolean;
}

// Result is stamped with the object key it belongs to, so a result for a stale
// key reads as "still loading the current one" without a synchronous reset in
// the effect body (which would cause cascading renders).
interface Loaded {
  key: string;
  text?: string;
  truncated: boolean;
  error: boolean;
}

/**
 * Fetch a file's bytes (auth blob) and decode the head as UTF-8 text, capped at
 * {@link MAX_TEXT_BYTES}. Returns loading/error flags and a `truncated` marker.
 * No-ops (and reports not-loading) when `objectKey` is absent.
 */
export function useFileText(objectKey: string | undefined): FileText {
  const [loaded, setLoaded] = useState<Loaded>();

  useEffect(() => {
    if (!objectKey) return;
    let cancelled = false;
    fetchBlob(`/api/files/${objectKey}`)
      .then(async (blob) => {
        const truncated = blob.size > MAX_TEXT_BYTES;
        const head = truncated ? blob.slice(0, MAX_TEXT_BYTES) : blob;
        const text = await head.text();
        if (!cancelled) setLoaded({ key: objectKey, text, truncated, error: false });
      })
      .catch(() => {
        if (!cancelled) setLoaded({ key: objectKey, truncated: false, error: true });
      });
    return () => {
      cancelled = true;
    };
  }, [objectKey]);

  const ready = loaded?.key === objectKey ? loaded : undefined;
  return {
    text: ready?.text,
    truncated: ready?.truncated ?? false,
    loading: !!objectKey && !ready,
    error: ready?.error ?? false,
  };
}
