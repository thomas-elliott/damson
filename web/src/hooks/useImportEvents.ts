import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { fetchEventSource } from "@microsoft/fetch-event-source";

import { getAccessToken } from "@/lib/token";
import { queryKeys } from "@/lib/queryKeys";

/**
 * Subscribes to /api/imports/events and refreshes the triage queue as imports
 * move through their stages. The queue is asset-shaped and an event carries an
 * import, so rather than fold one into the other the hook refetches; TanStack
 * collapses a burst of events into the one request already in flight.
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
            void queryClient.invalidateQueries({ queryKey: queryKeys.triage() });
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
