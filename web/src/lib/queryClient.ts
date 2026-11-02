import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { isAuthRedirectError } from "@/api";
import { MINUTE } from "@/lib/cachePolicy";
import { emitNotification } from "@/lib/notificationBridge";
import { queryKeys } from "@/lib/queryKeys";

// Several queries fail at once when a session is revoked; each joins the
// account fetch already in flight instead of restarting it.
function recheckAccount() {
  void queryClient.invalidateQueries({ queryKey: queryKeys.me }, { cancelRefetch: false });
}

export const queryClient: QueryClient = new QueryClient({
  // A revoked session on any query rechecks the account, whose failure is
  // where AuthGuard sends the tab to sign in.
  queryCache: new QueryCache({
    onError: (error, query) => {
      if (!isAuthRedirectError(error) || query.queryKey[0] === queryKeys.me[0]) return;
      recheckAccount();
    },
  }),
  // Mutation errors only — routing query errors here would be noisy on every background refetch.
  mutationCache: new MutationCache({
    onError: (error: unknown) => {
      if (isAuthRedirectError(error)) {
        recheckAccount();
        return;
      }
      emitNotification({
        tone: "error",
        title: "Something went wrong",
        message: error instanceof Error ? error.message : "request failed",
      });
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 5 * MINUTE,
      gcTime: 30 * MINUTE,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
