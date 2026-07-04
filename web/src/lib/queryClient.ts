import { MutationCache, QueryClient } from "@tanstack/react-query";

import { isAuthRedirectError } from "@/api";
import { MINUTE } from "@/lib/cachePolicy";
import { emitNotification } from "@/lib/notificationBridge";

export const queryClient = new QueryClient({
  // Mutation errors only — routing query errors here would be noisy on every background refetch.
  mutationCache: new MutationCache({
    onError: (error: unknown) => {
      if (isAuthRedirectError(error)) return;
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
