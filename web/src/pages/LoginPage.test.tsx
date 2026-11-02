import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { AuthRedirectError } from "@/api";
import { queryKeys } from "@/lib/queryKeys";

import { LoginPage } from "./LoginPage";

vi.mock("@/hooks/useDataQueries", () => ({
  useAuthMode: () => ({ mode: "local", needsBootstrap: false, loading: false }),
}));
vi.mock("@/lib/localAuth", () => ({ localLogin: vi.fn().mockResolvedValue({}) }));

function Landed() {
  const { pathname, search, hash } = useLocation();
  return <p>Landed on {pathname + search + hash}</p>;
}

// `from` is the router state AuthGuard sets when it sends a visitor to /login.
async function signIn(from?: unknown, queryClient = new QueryClient()) {
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[{ pathname: "/login", state: from ? { from } : null }]}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="*" element={<Landed />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await userEvent.type(screen.getByLabelText("Email"), "dev@example.com");
  await userEvent.type(screen.getByLabelText("Password"), "password1");
  await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("LoginPage", () => {
  it("returns to the page that was asked for, with its query and hash", async () => {
    await signIn({ pathname: "/library/42", search: "?q=x", hash: "#files" });
    expect(await screen.findByText("Landed on /library/42?q=x#files")).toBeInTheDocument();
  });

  it("goes home when no page was asked for", async () => {
    await signIn();
    expect(await screen.findByText("Landed on /")).toBeInTheDocument();
  });

  it("goes home instead of to a destination outside the app", async () => {
    await signIn("https://evil.example/");
    expect(await screen.findByText("Landed on /")).toBeInTheDocument();
  });

  // A revoked session reaches sign-in without a reload, with the old account
  // failed and the old library still cached. Kept past sign-in, the account
  // sends the user back to sign in and the library shows them what they lost.
  it("forgets the previous session's data", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(queryKeys.me, { id: 1 });
    queryClient.setQueryData(["tags"], [{ id: 1 }]);
    queryClient.setQueryData(queryKeys.authMode, { mode: "local" });
    await queryClient
      .fetchQuery({
        queryKey: queryKeys.me,
        queryFn: () => Promise.reject(new AuthRedirectError()),
        retry: false,
        staleTime: 0,
      })
      .catch(() => {});
    await signIn(undefined, queryClient);
    await screen.findByText("Landed on /");
    expect(queryClient.getQueryState(queryKeys.me)).toBeUndefined();
    expect(queryClient.getQueryData(["tags"])).toBeUndefined();
    expect(queryClient.getQueryData(queryKeys.authMode)).toEqual({ mode: "local" });
  });
});
