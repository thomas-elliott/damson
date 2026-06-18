import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { fetchEventSource } from "@microsoft/fetch-event-source";

import { getAccessToken } from "@/lib/token";
import { queryKeys } from "@/lib/queryKeys";
import type { PreviewRow } from "@/hooks/useDataQueries";

/**
 * Subscribes to /api/previews/events and keeps the preview-queue cache live.
 * Mirrors useImportEvents: every rollup is folded into the usePreviewQueue
 * query data via setQueryData, so components update without a refetch. A
 * version whose work is finished (no queued/processing files) drops out of the
 * in-flight list, matching the snapshot endpoint.
 */
export function usePreviewEvents(libraryId: number | null | undefined) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!libraryId) return;
    const ctrl = new AbortController();

    void (async () => {
      try {
        await fetchEventSource("/api/previews/events", {
          signal: ctrl.signal,
          headers: await sseHeaders(),
          openWhenHidden: true,
          onmessage: (msg) => {
            if (!msg.data) return;
            try {
              const ev = JSON.parse(msg.data) as PreviewRow;
              applyPreviewEvent(queryClient, ev);
            } catch {
              // ignore unparsable frames; keep the stream alive
            }
          },
        });
      } catch {
        // Aborted on unmount or library switch — nothing to do.
      }
    })();

    return () => ctrl.abort();
  }, [libraryId, queryClient]);
}

async function sseHeaders(): Promise<Record<string, string>> {
  const token = await getAccessToken();
  return {
    Accept: "text/event-stream",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

type PreviewCache = { previews: PreviewRow[]; total: number } | undefined;

// foldPreviewEvent upserts a rollup into the preview-queue cache, keyed by
// version_id. A rollup with no queued/processing files is terminal, so it's
// removed from the in-flight list rather than kept. Pure (exported) so the
// fold logic is unit-testable without a live query client.
export function foldPreviewEvent(cached: PreviewCache, ev: PreviewRow): PreviewCache {
  if (!cached) return cached;
  const next = cached.previews.filter((row) => row.version_id !== ev.version_id);
  const inFlight = ev.queued > 0 || ev.processing > 0;
  if (inFlight) next.unshift(ev);
  return { previews: next, total: next.length };
}

function applyPreviewEvent(
  queryClient: ReturnType<typeof useQueryClient>,
  ev: PreviewRow,
) {
  queryClient.setQueryData<PreviewCache>(queryKeys.previews, (cached) =>
    foldPreviewEvent(cached, ev),
  );
}
