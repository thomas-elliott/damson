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

// FINISHED_CAP mirrors the backend's retained-tail limit so the live fold
// doesn't grow the finished section past what a refetch would return.
const FINISHED_CAP = 50;

type ImportCache = { imports: ImportRow[]; finished?: ImportRow[]; total: number } | undefined;

// foldImportEvent applies one event to the in_flight cache: the active queue is
// the review backlog (anything not done, or done-but-still-needs-review); a
// done+reviewed row moves to the retained finished tail. `total` counts the
// active set only, matching the backend and the nav badge. Pure (exported) so
// the fold is unit-testable without a live query client.
export function foldImportEvent(cached: ImportCache, ev: ImportRow): ImportCache {
  if (!cached) return cached;
  const active = cached.imports.filter((row) => row.id !== ev.id);
  const finished = (cached.finished ?? []).filter((row) => row.id !== ev.id);
  const isActive = ev.stage !== "done" || ev.needs_review;
  if (isActive) active.unshift(ev);
  else if (ev.stage === "done") finished.unshift(ev);
  return { imports: active, finished: finished.slice(0, FINISHED_CAP), total: active.length };
}

// applyImportEvent upserts an event into every cached useImports query.
// The "in_flight" query is the live one QueuePage renders; the others are
// updated for symmetry (e.g. a "failed" filter open in another tab).
function applyImportEvent(queryClient: ReturnType<typeof useQueryClient>, ev: ImportRow) {
  queryClient.setQueryData<ImportCache>([...queryKeys.imports, "in_flight"], (cached) =>
    foldImportEvent(cached, ev),
  );
  const variants: Array<"all" | "done" | "failed"> = ["all", "done", "failed"];
  for (const variant of variants) {
    queryClient.setQueryData<ImportCache>([...queryKeys.imports, variant], (cached) => {
      if (!cached) return cached;
      const stageMatchesFilter = variant === "all" || variant === ev.stage;
      const next = cached.imports.filter((row) => row.id !== ev.id);
      if (stageMatchesFilter) next.unshift(ev);
      return { ...cached, imports: next, total: next.length };
    });
  }
}
