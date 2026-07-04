import { QueryClientProvider, useMutation } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { ApiError, AuthRedirectError } from "@/api";
import { queryClient } from "@/lib/queryClient";
import { NotificationProvider, useNotifications } from "./NotificationProvider";

function Bomb({ error }: { error: unknown }) {
  const m = useMutation({
    mutationFn: async () => {
      throw error;
    },
  });
  const { notifications } = useNotifications();
  return (
    <div>
      <button onClick={() => m.mutate()}>fire</button>
      <output>count:{notifications.length}</output>
      {notifications[0]?.message && <p>msg:{notifications[0].message}</p>}
    </div>
  );
}

function mount(error: unknown) {
  return render(
    <QueryClientProvider client={queryClient}>
      <NotificationProvider>
        <Bomb error={error} />
      </NotificationProvider>
    </QueryClientProvider>,
  );
}

describe("mutation errors → notification centre", () => {
  beforeEach(() => {
    localStorage.clear();
    queryClient.getMutationCache().clear();
  });

  it("routes a failed mutation to an error notification", async () => {
    mount(new ApiError(500, null, "kaboom"));
    await userEvent.click(screen.getByText("fire"));

    await waitFor(() => expect(screen.getByText("count:1")).toBeInTheDocument());
    expect(screen.getByText("msg:kaboom")).toBeInTheDocument();
  });

  it("does not notify on an auth-redirect error", async () => {
    mount(new AuthRedirectError());
    await userEvent.click(screen.getByText("fire"));

    await waitFor(() => expect(screen.getByText("fire")).toBeInTheDocument());
    expect(screen.getByText("count:0")).toBeInTheDocument();
  });
});
