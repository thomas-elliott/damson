import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

const pinMutate = vi.fn();
const unpinMutate = vi.fn();
const reloadDetail = vi.fn();

const project = {
  id: 7,
  name: "Ashfall",
  target_distribution: "open-source",
  asset_count: 1,
  updates_available: 0,
  pins: [
    {
      pin_id: 70,
      asset_id: 3,
      asset_name: "Castle Kit",
      pinned_version_id: 31,
      version_label: "v1",
      has_update: false,
      licence_status: "ok",
      size_bytes: 10,
      pinned_at: "2026-10-01T00:00:00Z",
    },
    {
      pin_id: 71,
      asset_id: 8,
      asset_name: "Map Link",
      pinned_version_id: null,
      version_label: "",
      is_reference: true,
      has_update: false,
      licence_status: "unk",
      size_bytes: 0,
      pinned_at: "2026-10-01T00:00:00Z",
    },
  ],
};

const library = [
  { id: 1, name: "Forest Pack" },
  { id: 2, name: "Desert Pack" },
  { id: 5, name: "Store Link", is_reference: true },
  { id: 3, name: "Castle Kit" },
  { id: 4, name: "Empty Pack" },
  { id: 6, name: "Broken Pack" },
  { id: 8, name: "Map Link", is_reference: true },
];

// Each asset with versions: its versions, and which one is current.
const details: Record<number, { current_version_id: number | null; versions: number[] }> = {
  1: { current_version_id: 12, versions: [11, 12] },
  2: { current_version_id: 22, versions: [21, 22] },
  3: { current_version_id: 32, versions: [31, 32] },
  4: { current_version_id: null, versions: [] },
};
// An asset whose details fail to load.
const BROKEN = 6;

vi.mock("@/hooks/useDataQueries", () => ({
  useProject: () => ({ data: project, loading: false, failed: false, error: null }),
  useProjectAudit: () => ({ data: [], loading: false }),
  useAssets: ({ q }: { q?: string }) => {
    const data = library.filter((a) => !q || a.name.toLowerCase().includes(q.toLowerCase()));
    return { data, total: data.length, loading: false, placeholder: false };
  },
  useAssetDetail: (id: number | null) => {
    if (id === BROKEN) return { data: null, loading: false, failed: true, reload: reloadDetail };
    const d = id == null ? undefined : details[id];
    return {
      data: d && {
        id,
        current_version_id: d.current_version_id,
        versions: d.versions.map((v) => ({
          id: v,
          version_label: `v${v}`,
          store_version: "",
          is_current: v === d.current_version_id,
        })),
      },
      loading: false,
      failed: false,
      reload: reloadDetail,
    };
  },
}));

vi.mock("@/hooks/useMutations", () => ({
  useDeleteProject: () => ({ mutate: vi.fn() }),
  usePinAssetToProject: () => ({
    mutate: pinMutate,
    reset: vi.fn(),
    isPending: false,
    error: null,
  }),
  useUnpinAssetFromProject: () => ({ mutate: unpinMutate }),
}));

vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));

import { ProjectDetailPage } from "./ProjectDetailPage";

function Location() {
  return <output aria-label="location">{useLocation().pathname}</output>;
}

function mount() {
  return render(
    <MemoryRouter initialEntries={["/projects/7"]}>
      <Routes>
        <Route path="/projects/:id" element={<ProjectDetailPage />} />
        <Route path="/library/:assetId" element={null} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
}

async function openPinDialog() {
  await userEvent.click(screen.getAllByRole("button", { name: "Pin asset" })[0]);
  return screen.getByRole("dialog");
}

beforeEach(() => {
  pinMutate.mockReset();
  unpinMutate.mockReset();
  reloadDetail.mockReset();
});

describe("ProjectDetailPage header", () => {
  it("names the target distribution rather than showing its stored value", () => {
    mount();
    expect(screen.getByText("Open source")).toBeInTheDocument();
    expect(screen.queryByText("open-source")).toBeNull();
  });
});

describe("ProjectDetailPage pin dialog", () => {
  it("offers the picked asset's versions with the current one preselected", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Forest Pack" }));

    expect(within(dialog).getByRole("combobox", { name: "Version" })).toHaveTextContent(
      "v12 (current)",
    );
    await userEvent.click(within(dialog).getByRole("button", { name: "Pin version" }));
    expect(pinMutate).toHaveBeenCalledWith(
      { projectId: 7, asset_id: 1, version_id: 12 },
      expect.anything(),
    );
  });

  it("pins the newly picked asset's version after switching assets", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Forest Pack" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Desert Pack" }));

    await userEvent.click(within(dialog).getByRole("button", { name: "Pin version" }));
    expect(pinMutate).toHaveBeenCalledWith(
      { projectId: 7, asset_id: 2, version_id: 22 },
      expect.anything(),
    );
  });

  it("pins a reference asset as the asset, with no version", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Store Link" }));

    expect(within(dialog).queryByRole("combobox", { name: "Version" })).toBeNull();
    await userEvent.click(within(dialog).getByRole("button", { name: "Pin asset" }));
    expect(pinMutate).toHaveBeenCalledWith(
      { projectId: 7, asset_id: 5, version_id: undefined },
      expect.anything(),
    );
  });
  it("still names the picked asset once a new search hides it", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Forest Pack" }));
    await userEvent.type(within(dialog).getByRole("searchbox", { name: "Asset" }), "desert");

    await waitFor(() =>
      expect(within(dialog).queryByRole("button", { name: "Forest Pack" })).toBeNull(),
    );
    expect(within(dialog).getByText(/Pinning/)).toHaveTextContent("Pinning Forest Pack");
  });

  it("shows why it cannot pin when the asset's versions fail to load", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Broken Pack" }));

    expect(within(dialog).getByText("Couldn't load this asset's versions")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: /Retry/ }));
    expect(reloadDetail).toHaveBeenCalled();
    expect(within(dialog).getByRole("button", { name: "Pin version" })).toBeDisabled();
  });

  it("says when an asset has no versions to pin", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Empty Pack" }));

    expect(within(dialog).getByText("This asset has no versions to pin")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Pin version" })).toBeDisabled();
  });

  it("marks an already pinned asset and offers to change its version", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Castle Kit · Pinned · v1" }));

    expect(within(dialog).getByText(/is pinned at/)).toHaveTextContent(
      "Castle Kit is pinned at v1",
    );
    expect(within(dialog).getByRole("combobox", { name: "Version" })).toHaveTextContent("v31");
    // Already at the chosen version: changing to it would do nothing.
    expect(within(dialog).getByRole("button", { name: "Change pinned version" })).toBeDisabled();
  });

  it("does not offer to pin a pinned reference asset again", async () => {
    mount();
    const dialog = await openPinDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "Map Link · Pinned" }));

    expect(within(dialog).getByRole("button", { name: "Already pinned" })).toBeDisabled();
  });
});

describe("ProjectDetailPage pin table", () => {
  it("opens an asset from its name", async () => {
    mount();
    await userEvent.click(screen.getByRole("link", { name: "Castle Kit" }));
    expect(screen.getByLabelText("location")).toHaveTextContent("/library/3");
  });

  it("asks before unpinning", async () => {
    mount();
    await userEvent.click(screen.getAllByRole("button", { name: "Unpin" })[0]);
    expect(unpinMutate).not.toHaveBeenCalled();

    const confirm = screen.getByRole("dialog", { name: 'Unpin "Castle Kit"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Unpin" }));
    await waitFor(() => expect(unpinMutate).toHaveBeenCalledWith({ projectId: 7, pinId: 70 }));
  });
});

describe("ProjectDetailPage header", () => {
  it("names Projects once, with a chevron back button", () => {
    mount();
    expect(screen.getByRole("button", { name: "Back to projects" })).toHaveTextContent("");
    expect(screen.getAllByText("Projects")).toHaveLength(1);
  });

  it("shows the licence mix for a project with pins", () => {
    mount();
    expect(screen.getByText("Licence mix")).toBeInTheDocument();
  });

  it("hides the licence mix for a project with no pins", () => {
    const { pins, asset_count } = project;
    Object.assign(project, { pins: [], asset_count: 0 });
    try {
      mount();
      expect(screen.queryByText("Licence mix")).toBeNull();
      expect(screen.getByText("No assets pinned")).toBeInTheDocument();
    } finally {
      Object.assign(project, { pins, asset_count });
    }
  });
});
