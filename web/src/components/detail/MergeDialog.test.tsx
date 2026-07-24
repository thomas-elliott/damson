import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { MergeResponse } from "@/hooks/useMutations";

const mergeMutate = vi.fn();
const patchMutate = vi.fn();
let assetsData: { id: number; name: string }[] = [];
let mergeResult: MergeResponse;

vi.mock("@/hooks/useDataQueries", () => ({
  useAssets: () => ({
    data: assetsData,
    total: assetsData.length,
    loading: false,
    error: null,
    reload: vi.fn(),
  }),
}));

vi.mock("@/hooks/useMutations", () => ({
  useMergeAssets: () => ({
    mutate: (
      vars: { targetId: number; source_asset_id: number },
      opts?: { onSuccess?: (r: MergeResponse) => void },
    ) => {
      mergeMutate(vars);
      opts?.onSuccess?.(mergeResult);
    },
    reset: vi.fn(),
    isPending: false,
    error: null,
  }),
  usePatchAsset: () => ({
    mutate: (vars: unknown, opts?: { onSuccess?: () => void }) => {
      patchMutate(vars);
      opts?.onSuccess?.();
    },
    reset: vi.fn(),
    isPending: false,
  }),
}));

import { MergeDialog } from "./MergeDialog";

describe("MergeDialog", () => {
  beforeEach(() => {
    mergeMutate.mockReset();
    patchMutate.mockReset();
    assetsData = [
      { id: 1, name: "Alpha" },
      { id: 2, name: "Target" },
      { id: 3, name: "Bravo" },
    ];
    mergeResult = {
      target_asset_id: 2,
      merged_versions: 1,
      adoptable: { name: "Bravo Deluxe" },
    };
  });

  it("excludes the target and merges the picked source", () => {
    render(<MergeDialog targetId={2} targetName="Target" open onOpenChange={vi.fn()} />);

    // The target is never a merge candidate.
    expect(screen.queryByRole("button", { name: "Target" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Bravo" }));
    fireEvent.click(screen.getByRole("button", { name: "Merge" }));

    expect(mergeMutate).toHaveBeenCalledWith({ targetId: 2, source_asset_id: 3 });
  });

  it("keeps Merge disabled until a source is chosen", () => {
    render(<MergeDialog targetId={2} targetName="Target" open onOpenChange={vi.fn()} />);

    const mergeButton = screen.getByRole("button", { name: "Merge" });
    expect(mergeButton).toBeDisabled();
    fireEvent.click(mergeButton);
    expect(mergeMutate).not.toHaveBeenCalled();
  });

  it("adopts the merged asset's name on Done", () => {
    render(<MergeDialog targetId={2} targetName="Target" open onOpenChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Bravo" }));
    fireEvent.click(screen.getByRole("button", { name: "Merge" }));
    // The adopt panel appears after a successful merge.
    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    expect(patchMutate).toHaveBeenCalledWith({ id: 2, name: "Bravo Deluxe" });
  });
});
