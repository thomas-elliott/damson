import { QueryClient } from "@tanstack/react-query";

import { MINUTE } from "@/lib/cachePolicy";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * MINUTE,
      gcTime: 30 * MINUTE,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
