import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AuthRedirectError } from "@/api";

import { AuthGuard } from "./AuthGuard";

// The two queries that gate the app, as their hooks return them.
const auth = vi.hoisted(() => ({
  mode: { mode: null as string | null, loading: false, error: null as unknown, reload: vi.fn() },
  me: { data: null as unknown, loading: false, error: null as unknown, reload: vi.fn() },
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useAuthMode: () => ({ ...auth.mode, needsBootstrap: false }),
  useMe: () => auth.me,
}));
vi.mock("@/lib/localAuth", () => ({ readLocalSession: () => ({ token: "t" }) }));
vi.mock("@/lib/logout", () => ({ logout: vi.fn() }));
vi.mock("@/lib/oidc", () => ({
  getUserManager: () => ({ getUser: async () => ({ expired: false }), signinRedirect: vi.fn() }),
}));

const { logout } = await import("@/lib/logout");

function mount() {
  return render(
    <MemoryRouter initialEntries={["/library"]}>
      <Routes>
        <Route element={<AuthGuard />}>
          <Route path="/library" element={<div>app</div>} />
        </Route>
        <Route path="/login" element={<div>sign-in page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  auth.mode = { mode: "local", loading: false, error: null, reload: vi.fn() };
  auth.me = { data: { id: 1, needs_profile: false }, loading: false, error: null, reload: vi.fn() };
  vi.mocked(logout).mockClear();
});

describe("AuthGuard", () => {
  it("renders the app once the mode and the account load", async () => {
    mount();
    expect(await screen.findByText("app")).toBeInTheDocument();
  });

  it("shows a failed mode lookup with a Retry and a Sign out", async () => {
    auth.mode = { mode: null, loading: false, error: new Error("500"), reload: vi.fn() };
    mount();
    expect(screen.getByText("Couldn't reach Damson")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(auth.mode.reload).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Sign out" }));
    expect(logout).toHaveBeenCalled();
  });

  it("shows a failed account load with a Retry and a Sign out", async () => {
    auth.me = { data: null, loading: false, error: new Error("500"), reload: vi.fn() };
    mount();
    expect(await screen.findByText("Couldn't load your account")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(auth.me.reload).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
  });

  // A local session the server has revoked (a removed member, say) cannot be
  // renewed, and nothing else leaves the page.
  it("sends a revoked local session to sign in", async () => {
    auth.me = { data: null, loading: false, error: new AuthRedirectError(), reload: vi.fn() };
    mount();
    expect(await screen.findByText("sign-in page")).toBeInTheDocument();
  });

  it("keeps loading while an OIDC sign-in redirect leaves the page", async () => {
    auth.mode = { mode: "oidc", loading: false, error: null, reload: vi.fn() };
    auth.me = { data: null, loading: false, error: new AuthRedirectError(), reload: vi.fn() };
    mount();
    // Past AuthGuard's own session check, into the account load.
    await act(() => Promise.resolve());
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
  });
});
