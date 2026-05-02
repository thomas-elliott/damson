import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { fetchEventSource } from "@microsoft/fetch-event-source";

import { getAccessToken } from "@/lib/token";
import { queryKeys } from "@/lib/queryKeys";
import type { ImportRow } from "@/hooks/useDataQueries";

/**
 * Subscribes to /api/imports/events and keeps the in-flight import cache
 * live. The hook owns no state of its own — every event is folded into
 * the existing `useImports("in_flight")` query data via setQueryData, so
 * components rendering off that hook update without a refetch.
 *
 * Auth: fetch-event-source preserves the standard Bearer header (native
 * EventSource cannot), so the SSE route uses the exact same auth chain as
 * every other authenticated endpoint.
 */
export function useImportEvents(libraryId: number | null | undefined) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!libraryId) return;
    const ctrl = new AbortController();

    void (async () => {
      try {
        await fetchEventSource("/api/imports/events", {
          signal: ctrl.signal,
          headers: await sseHeaders(),
          // openWhenHidden: keep streaming when the tab is in the
          // background. fetch-event-source pauses by default, which would
          // strand stage transitions until the user came back.
          openWhenHidden: true,
          onmessage: (msg) => {
            if (!msg.data) return;
            try {
              const ev = JSON.parse(msg.data) as ImportRow;
              applyImportEvent(queryClient, ev);
            } catch {
              // ignore unparsable frames; keep the stream alive
            }
          },
          // No onerror handler: fetch-event-source's default behaviour is
          // to reconnect with exponential backoff, which is what we want.
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

// applyImportEvent upserts an event into every cached useImports query.
// The "in_flight" query is the live one QueuePage renders; the others are
// updated for symmetry (e.g. a "failed" filter open in another tab). For
// terminal `done` events the row is removed from the in_flight slice
// because the snapshot endpoint excludes done rows.
function applyImportEvent(queryClient: ReturnType<typeof useQueryClient>, ev: ImportRow) {
  type CachedShape = { imports: ImportRow[]; total: number } | undefined;
  const variants: Array<"in_flight" | "all" | "done" | "failed"> = [
    "in_flight",
    "all",
    "done",
    "failed",
  ];
  for (const variant of variants) {
    queryClient.setQueryData<CachedShape>([...queryKeys.imports, variant], (cached) => {
      if (!cached) return cached;
      const inFlightHidesDone = variant === "in_flight";
      const stageMatchesFilter =
        variant === "all" || variant === "in_flight" || variant === ev.stage;
      const next = cached.imports.filter((row) => row.id !== ev.id);
      const shouldKeep = stageMatchesFilter && !(inFlightHidesDone && ev.stage === "done");
      if (shouldKeep) next.unshift(ev);
      return { imports: next, total: next.length };
    });
  }
}
