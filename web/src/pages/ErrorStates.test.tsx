import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType, ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";

import { CollectionDetailPage } from "./CollectionDetailPage";
import { CollectionsPage } from "./CollectionsPage";
import { PreviewQueuePage } from "./PreviewQueuePage";
import { ProjectDetailPage } from "./ProjectDetailPage";
import { ProjectsPage } from "./ProjectsPage";
import { QueuePage } from "./QueuePage";

vi.mock("@/lib/token", () => ({ getAccessToken: vi.fn().mockResolvedValue(null) }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
// Live updates come over EventSource, which jsdom lacks.
vi.mock("@/hooks/useImportEvents", () => ({ useImportEvents: () => {} }));
vi.mock("@/hooks/usePreviewEvents", () => ({ usePreviewEvents: () => {} }));

// The endpoint under test answers each status in `failing` in turn, then 200
// with `body`; every other endpoint answers 200 with an empty object.
let endpoint: string;
let failing: number[];
let body: unknown;

beforeEach(() => {
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
      if (path !== endpoint) return Promise.resolve(Response.json({}));
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
    "Queue",
    QueuePage,
    "/api/imports",
    "Couldn't load the import queue",
    { imports: [] },
    "Nothing waiting for review",
  ],
  [
    "Processing",
    PreviewQueuePage,
    "/api/previews",
    "Couldn't load the processing queue",
    { previews: [] },
    "No previews processing",
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

  it("shows the error with a Retry for any other failure", async () => {
    endpoint = api;
    failing = [500];
    renderAt(path, route, element);
    expect(await screen.findByText(failed)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.queryByText(notFound)).not.toBeInTheDocument();
  });
});
