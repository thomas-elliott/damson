import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType, ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";
import { PackageSummary } from "@/components/detail/PackageSummary";
import type { AssetDetailView } from "@/hooks/useDataQueries";

import { CollectionDetailPage } from "./CollectionDetailPage";
import { CollectionsPage } from "./CollectionsPage";
import { ProjectDetailPage } from "./ProjectDetailPage";
import { ProjectsPage } from "./ProjectsPage";
import { SettingsPage } from "./SettingsPage";
import { TriagePage } from "./TriagePage";

vi.mock("@/lib/token", () => ({ getAccessToken: vi.fn().mockResolvedValue(null) }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
// Live updates come over EventSource, which jsdom lacks.
vi.mock("@/hooks/useImportEvents", () => ({ useImportEvents: () => {} }));
vi.mock("@/hooks/usePreviewEvents", () => ({ usePreviewEvents: () => {} }));

// The endpoint under test answers each status in `failing` in turn, then 200
// with `body`; every other endpoint answers 200 with its entry in `others`, or
// an empty object.
let endpoint: string;
let failing: number[];
let body: unknown;
let others: Record<string, unknown>;

beforeEach(() => {
  others = {};
  // The asset panel goes fullscreen on narrow screens; jsdom has no matchMedia.
  vi.stubGlobal("matchMedia", () => ({
    matches: false,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => {
      const path = new URL(url, "http://x").pathname;
      if (path !== endpoint) return Promise.resolve(Response.json(others[path] ?? {}));
      const status = failing.shift();
      return Promise.resolve(
        status ? Response.json({ error: "internal error" }, { status }) : Response.json(body),
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

function renderAt(path: string, route: string, element: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={route} element={element} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe.each<[string, ComponentType, string, string, unknown, string]>([
  [
    "Triage",
    TriagePage,
    "/api/triage",
    "Couldn't load the triage queue",
    { assets: [], reviewed: [], total: 0 },
    "Nothing to triage",
  ],
  [
    "Projects",
    ProjectsPage,
    "/api/projects",
    "Couldn't load projects",
    { projects: [] },
    "No projects yet",
  ],
  [
    "Collections",
    CollectionsPage,
    "/api/collections",
    "Couldn't load collections",
    { collections: [] },
    "No collections yet",
  ],
])("%s", (_name, Page, path, errorTitle, emptyBody, emptyTitle) => {
  it("shows the error with a Retry that loads the list once it succeeds", async () => {
    endpoint = path;
    failing = [500];
    body = emptyBody;
    renderAt("/", "/", <Page />);
    expect(await screen.findByText(errorTitle)).toBeInTheDocument();
    expect(screen.queryByText(emptyTitle)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(emptyTitle)).toBeInTheDocument();
    expect(screen.queryByText(errorTitle)).not.toBeInTheDocument();
  });
});

describe.each<[string, string, string, ReactNode, string, string]>([
  [
    "Project detail",
    "/projects/5",
    "/projects/:id",
    <ProjectDetailPage />,
    "Project not found",
    "Couldn't load this project",
  ],
  [
    "Collection detail",
    "/collections/5",
    "/collections/:id",
    <CollectionDetailPage />,
    "Collection not found",
    "Couldn't load this collection",
  ],
  [
    "Asset panel",
    "/",
    "/",
    <AssetDetailPanel assetId={5} onClose={() => {}} />,
    "Asset not found",
    "Couldn't load this asset",
  ],
])("%s", (_name, path, route, element, notFound, failed) => {
  const api = path === "/" ? "/api/assets/5" : `/api${path}`;

  it("says not found for a 404", async () => {
    endpoint = api;
    failing = [404];
    renderAt(path, route, element);
    expect(await screen.findByText(notFound)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it("says not found for a 400", async () => {
    endpoint = api;
    failing = [400];
    renderAt(path, route, element);
    expect(await screen.findByText(notFound)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it("shows the error with a Retry for any other failure", async () => {
    endpoint = api;
    failing = [500];
    renderAt(path, route, element);
    expect(await screen.findByText(failed)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.queryByText(notFound)).not.toBeInTheDocument();
  });
});

// The detail pages' own lists fail apart from the page itself: the page loads
// and the list shows the error where its empty state would be.
describe.each<[string, string, string, ReactNode, string, unknown, string, unknown, string]>([
  [
    "Collection detail's assets",
    "/collections/5",
    "/collections/:id",
    <CollectionDetailPage />,
    "/api/collections/5",
    { id: 5, name: "Props" },
    "/api/assets",
    { assets: [], total: 0 },
    "No assets in this collection",
  ],
  [
    "Project detail's activity",
    "/projects/5",
    "/projects/:id",
    <ProjectDetailPage />,
    "/api/projects/5",
    { id: 5, name: "Castle", pins: [], asset_count: 0, updates_available: 0 },
    "/api/projects/5/audit",
    { entries: [], total: 0 },
    "No activity yet",
  ],
])("%s", (_name, path, route, element, pageApi, pageBody, listApi, emptyBody, emptyTitle) => {
  it("shows the error with a Retry that loads the list once it succeeds", async () => {
    others = { [pageApi]: pageBody };
    endpoint = listApi;
    failing = [500];
    body = emptyBody;
    renderAt(path, route, element);
    const retry = await screen.findByRole("button", { name: "Retry" });
    expect(screen.queryByText(emptyTitle)).not.toBeInTheDocument();
    await userEvent.click(retry);
    expect(await screen.findByText(emptyTitle)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });
});

// The package page's activity sits in a collapsed section, so the test opens it.
describe("Package page's activity", () => {
  it("shows the error with a Retry that loads the list once it succeeds", async () => {
    const asset = {
      id: 5,
      name: "Castle",
      description: "",
      source_url: "",
      needs_review: false,
      is_reference: false,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      tags: [],
      versions: [],
      files: [],
      images: [],
      projects: [],
      collections: [],
    } as AssetDetailView;
    others = { "/api/assets/5": asset };
    endpoint = "/api/assets/5/audit";
    failing = [500];
    body = { entries: [], total: 0 };
    renderAt("/", "/", <PackageSummary asset={asset} onDeleted={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Audit" }));
    expect(await screen.findByText("Couldn't load recent activity")).toBeInTheDocument();
    expect(screen.queryByText("No activity yet")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("No activity yet")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load recent activity")).not.toBeInTheDocument();
  });
});

// Settings waits for /api/me to know the library, then loads its settings.
describe("Settings", () => {
  it("shows the error with a Retry that loads the settings once it succeeds", async () => {
    others = {
      "/api/me": {
        id: 1,
        email: "a@example.com",
        role: "user",
        current_library: { id: 1, name: "Main", role: "member" },
      },
    };
    endpoint = "/api/settings";
    failing = [500];
    body = { theme_default: "dark", grid_density_default: "medium" };
    renderAt("/", "/", <SettingsPage />);
    expect(await screen.findByText("Couldn't load settings")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Library settings")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load settings")).not.toBeInTheDocument();
  });
});

// A project the client failed to load has nothing to pin to or delete.
describe("Project detail's toolbar", () => {
  it("disables Pin asset and Delete when the project fails to load", async () => {
    endpoint = "/api/projects/5";
    failing = [500];
    renderAt("/projects/5", "/projects/:id", <ProjectDetailPage />);
    expect(await screen.findByText("Couldn't load this project")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pin asset" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
  });
});

// Every screen names itself with one h1, for screen-reader heading navigation.
describe.each<[string, ComponentType, string, unknown]>([
  ["Triage", TriagePage, "/api/triage", { assets: [], reviewed: [], total: 0 }],
  ["Projects", ProjectsPage, "/api/projects", { projects: [] }],
  ["Collections", CollectionsPage, "/api/collections", { collections: [] }],
])("%s", (heading, Page, path, emptyBody) => {
  it("names the page with one h1", async () => {
    endpoint = path;
    failing = [];
    body = emptyBody;
    renderAt("/", "/", <Page />);
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(heading);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
  });
});
