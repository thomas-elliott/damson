import { afterEach, describe, expect, it } from "vitest";

import { AuthRedirectError } from "@/api";
import { queryClient } from "@/lib/queryClient";
import { queryKeys } from "@/lib/queryKeys";

const meState = () => queryClient.getQueryState(queryKeys.me);

describe("queryClient", () => {
  afterEach(() => queryClient.clear());

  // AuthGuard sends a revoked session to sign in from the account query, so
  // a 401 on any other query has to reach it, or the page just shows nothing.
  it("rechecks the account when another query's session is revoked", async () => {
    queryClient.setQueryData(queryKeys.me, { id: 1 });
    await queryClient
      .fetchQuery({
        queryKey: ["assets", "revoked"],
        queryFn: () => Promise.reject(new AuthRedirectError()),
        retry: false,
      })
      .catch(() => {});
    expect(meState()?.isInvalidated).toBe(true);
  });

  it("leaves the account alone when a query fails for another reason", async () => {
    queryClient.setQueryData(queryKeys.me, { id: 1 });
    await queryClient
      .fetchQuery({
        queryKey: ["assets", "broken"],
        queryFn: () => Promise.reject(new Error("500")),
        retry: false,
      })
      .catch(() => {});
    expect(meState()?.isInvalidated).toBe(false);
  });
});
