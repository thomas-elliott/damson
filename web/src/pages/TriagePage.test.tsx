import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PreviewRow, TriageRow } from "@/hooks/useDataQueries";

import { TriagePage } from "./TriagePage";

vi.mock("@/lib/token", () => ({ getAccessToken: vi.fn().mockResolvedValue(null) }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
// Live updates come over EventSource, which jsdom lacks.
vi.mock("@/hooks/useImportEvents", () => ({ useImportEvents: () => {} }));
vi.mock("@/hooks/usePreviewEvents", () => ({ usePreviewEvents: () => {} }));
vi.mock("@/components/filebrowser/useFileObjectUrl", () => ({ useBlobUrl: () => undefined }));

const row = (over: Partial<TriageRow>): TriageRow => ({
  asset_id: 1,
  asset_name: "asset",
  needs_review: true,
  is_reference: false,
  matched: false,
  updated_at: "2026-10-30T12:00:00Z",
  ...over,
});

let triage: { assets: TriageRow[]; reviewed: TriageRow[]; total: number };
let previews: { previews: PreviewRow[]; finished: PreviewRow[]; total: number };
let posts: string[];

beforeEach(() => {
  posts = [];
  triage = { assets: [], reviewed: [], total: 0 };
  previews = { previews: [], finished: [], total: 0 };
  vi.stubGlobal("matchMedia", () => ({
    matches: false,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string, init?: RequestInit) => {
      const path = new URL(url, "http://x").pathname;
      if (init?.method === "POST") {
        posts.push(path);
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      if (path === "/api/triage") return Promise.resolve(Response.json(triage));
      if (path === "/api/previews") return Promise.resolve(Response.json(previews));
      return Promise.resolve(Response.json({}));
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/queue"]}>
        <TriagePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

async function findRow(name: string): Promise<HTMLElement> {
  const title = await screen.findByText(name);
  return title.closest(".\\@container") as HTMLElement;
}

describe("TriagePage", () => {
  it("lists a needs-review asset that never went through an import, in the panel's words", async () => {
    triage = {
      assets: [row({ asset_id: 7, asset_name: "Recorded ref", is_reference: true })],
      reviewed: [],
      total: 1,
    };
    mount();
    const card = await findRow("Recorded ref");
    expect(within(card).getByText("Needs review")).toBeInTheDocument();
    expect(within(card).getByText("No store match")).toBeInTheDocument();
    expect(within(card).getByText("No description yet.")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Find listing" })).toBeInTheDocument();

    await userEvent.click(within(card).getByRole("button", { name: "Mark reviewed" }));
    expect(posts).toEqual(["/api/assets/7/review"]);
  });

  it("offers Retry and Dismiss on a failed import, and no review actions", async () => {
    triage = {
      assets: [
        row({
          asset_id: 3,
          asset_name: "Broken pack",
          import: { id: 42, stage: "failed", error: "archive is truncated" },
        }),
      ],
      reviewed: [],
      total: 1,
    };
    mount();
    const card = await findRow("Broken pack");
    expect(within(card).getByText("Import failed")).toBeInTheDocument();
    expect(within(card).getByText("archive is truncated")).toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Mark reviewed" })).not.toBeInTheDocument();

    await userEvent.click(within(card).getByRole("button", { name: "Dismiss" }));
    expect(posts).toEqual(["/api/imports/42/dismiss"]);
  });

  it("shows a running import's stage without actions that would race it", async () => {
    triage = {
      assets: [
        row({
          asset_name: "Uploading",
          needs_review: true,
          import: { id: 9, stage: "extracting" },
        }),
      ],
      reviewed: [],
      total: 1,
    };
    mount();
    const card = await findRow("Uploading");
    expect(within(card).getByText("Importing · extracting")).toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Mark reviewed" })).not.toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it("calls a matched asset a store match, and offers Re-match", async () => {
    triage = {
      assets: [row({ asset_name: "Matched", matched: true, description: "**Bold** words" })],
      reviewed: [],
      total: 1,
    };
    mount();
    const card = await findRow("Matched");
    expect(within(card).getByText("Store match")).toBeInTheDocument();
    expect(within(card).getByText("Bold words")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Re-match" })).toBeInTheDocument();
  });

  it("folds preview processing into the page", async () => {
    previews = {
      previews: [
        {
          version_id: 5,
          asset_id: 2,
          asset_name: "Rendering pack",
          status: "processing",
          queued: 3,
          processing: 1,
          done: 2,
          failed: 0,
          total: 6,
          unsupported: 0,
          created_at: "2026-10-30T11:00:00Z",
          updated_at: "2026-10-30T12:00:00Z",
        },
      ],
      finished: [],
      total: 1,
    };
    mount();
    expect(await screen.findByRole("heading", { name: "Processing previews" })).toBeInTheDocument();
    const card = await findRow("Rendering pack");
    expect(within(card).getByText("Rendering")).toBeInTheDocument();
    expect(within(card).getByText("2/6 previews")).toBeInTheDocument();
  });

  it("lists recently reviewed assets under their own heading", async () => {
    triage = {
      assets: [],
      reviewed: [row({ asset_name: "Done with", needs_review: false })],
      total: 0,
    };
    mount();
    expect(await screen.findByRole("heading", { name: "Recently reviewed" })).toBeInTheDocument();
    const card = await findRow("Done with");
    expect(within(card).getByText("Reviewed")).toBeInTheDocument();
    expect(screen.getByText("Nothing waiting for review.")).toBeInTheDocument();
  });
});
