import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

const pinMutate = vi.fn();
const unpinMutate = vi.fn();

const project = {
  id: 7,
  name: "Ashfall",
  target_distribution: "internal",
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
  ],
};

const library = [
  { id: 1, name: "Forest Pack" },
  { id: 2, name: "Desert Pack" },
  { id: 5, name: "Store Link", is_reference: true },
];

// Each asset with versions: its versions, and which one is current.
const details: Record<number, { current_version_id: number; versions: number[] }> = {
  1: { current_version_id: 12, versions: [11, 12] },
  2: { current_version_id: 22, versions: [21, 22] },
};

vi.mock("@/hooks/useDataQueries", () => ({
  useProject: () => ({ data: project, loading: false, failed: false, error: null }),
  useProjectAudit: () => ({ data: [], loading: false }),
  useAssets: () => ({ data: library, total: library.length, loading: false, placeholder: false }),
  useAssetDetail: (id: number | null) => {
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
});

describe("ProjectDetailPage pin table", () => {
  it("opens an asset from its name", async () => {
    mount();
    await userEvent.click(screen.getByRole("link", { name: "Castle Kit" }));
    expect(screen.getByLabelText("location")).toHaveTextContent("/library/3");
  });

  it("asks before unpinning", async () => {
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Unpin" }));
    expect(unpinMutate).not.toHaveBeenCalled();

    const confirm = screen.getByRole("dialog", { name: 'Unpin "Castle Kit"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Unpin" }));
    await waitFor(() => expect(unpinMutate).toHaveBeenCalledWith({ projectId: 7, pinId: 70 }));
  });
});
