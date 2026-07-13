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

type PreviewCache = { previews: PreviewRow[]; finished?: PreviewRow[]; total: number } | undefined;

// FINISHED_CAP mirrors the backend's retained-tail limit so the live fold
// doesn't grow the finished section past what a refetch would return.
const FINISHED_CAP = 50;

// foldPreviewEvent upserts a rollup into the preview-queue cache, keyed by
// version_id. A rollup with queued/processing files stays in flight; one that
// has finished (nothing pending, something rendered) moves to the retained
// finished tail. Pure (exported) so the fold logic is unit-testable without a
// live query client.
export function foldPreviewEvent(cached: PreviewCache, ev: PreviewRow): PreviewCache {
  if (!cached) return cached;
  const previews = cached.previews.filter((row) => row.version_id !== ev.version_id);
  const finished = (cached.finished ?? []).filter((row) => row.version_id !== ev.version_id);
  const inFlight = ev.queued > 0 || ev.processing > 0;
  if (inFlight) previews.unshift(ev);
  else if (ev.done + ev.failed > 0) finished.unshift(ev);
  return { previews, finished: finished.slice(0, FINISHED_CAP), total: previews.length };
}

function applyPreviewEvent(queryClient: ReturnType<typeof useQueryClient>, ev: PreviewRow) {
  queryClient.setQueryData<PreviewCache>(queryKeys.previews, (cached) =>
    foldPreviewEvent(cached, ev),
  );
}
