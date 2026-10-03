import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AssetDetailView } from "@/hooks/useDataQueries";

const patchMutate = vi.fn();
let patchError: Error | null = null;

vi.mock("@/hooks/useMutations", () => ({
  usePatchAsset: () => ({
    mutate: (vars: unknown, opts?: { onSuccess?: () => void }) => {
      patchMutate(vars);
      if (!patchError) opts?.onSuccess?.();
    },
    isPending: false,
    error: patchError,
  }),
}));

const { LocationSection } = await import("./LocationSection");

function reference(over: Partial<AssetDetailView> = {}): AssetDetailView {
  return {
    id: 7,
    name: "POLYGON_Fantasy_Kingdom",
    description: "",
    source_url: "https://assetstore.unity.com/packages/k-1",
    location_path: "/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage",
    needs_review: false,
    is_reference: true,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    tags: [],
    versions: [],
    files: [],
    images: [],
    projects: [],
    collections: [],
    ...over,
  };
}

describe("LocationSection", () => {
  beforeEach(() => {
    patchMutate.mockClear();
    patchError = null;
  });

  it("shows the path with Copy and the store link with Open", () => {
    render(<LocationSection asset={reference()} />);
    expect(
      screen.getByText("/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy path" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open store link" })).toHaveAttribute(
      "href",
      "https://assetstore.unity.com/packages/k-1",
    );
  });

  it("copies the path to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<LocationSection asset={reference()} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy path" }));
    expect(writeText).toHaveBeenCalledWith("/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage");
    expect(await screen.findByText("Copied")).toBeInTheDocument();
  });

  it("saves an edited path and store link", () => {
    render(<LocationSection asset={reference()} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit location" }));
    fireEvent.change(screen.getByLabelText("Path"), { target: { value: " /mnt/nfs/Other " } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(patchMutate).toHaveBeenCalledWith({
      id: 7,
      location_path: "/mnt/nfs/Other",
      source_url: "https://assetstore.unity.com/packages/k-1",
    });
  });

  it("won't save with both the path and the store link cleared", () => {
    render(<LocationSection asset={reference()} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit location" }));
    fireEvent.change(screen.getByLabelText("Path"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("Store link"), { target: { value: " " } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText(/keeps a path or a store link/)).toBeInTheDocument();
  });

  it("shows the server's refusal of a path already recorded", () => {
    patchError = new Error('that path is already recorded on "Forest" (asset 3)');
    render(<LocationSection asset={reference()} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit location" }));
    expect(screen.getByText(/already recorded on "Forest"/)).toBeInTheDocument();
  });
});
