import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const bulkMutate = vi.fn();
vi.mock("@/hooks/useMutations", () => ({
  useBulkAssets: () => ({ mutate: bulkMutate, isPending: false }),
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useCollections: () => ({ data: [{ id: 1, name: "Faves" }], loading: false }),
}));

const notify = vi.fn();
vi.mock("@/components/notifications/NotificationProvider", () => ({
  useNotifications: () => ({ notify }),
}));

import { BulkAssetBar } from "./BulkAssetBar";

describe("BulkAssetBar", () => {
  beforeEach(() => {
    bulkMutate.mockClear();
    notify.mockClear();
  });

  it("dispatches a review action for the selected ids", async () => {
    render(<BulkAssetBar assetIds={[1, 2, 3]} onClear={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "Mark reviewed" }));

    expect(bulkMutate).toHaveBeenCalledTimes(1);
    expect(bulkMutate.mock.calls[0][0]).toEqual({
      asset_ids: [1, 2, 3],
      action: "review",
    });
  });

  it("adds a tag with the typed path, and won't dispatch an empty tag", async () => {
    render(<BulkAssetBar assetIds={[7]} onClear={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "Tag" }));

    // Empty path → the add button is disabled, so nothing is dispatched.
    const addButton = screen.getByRole("button", { name: "Add tag" });
    expect(addButton).toBeDisabled();

    await userEvent.type(screen.getByLabelText("Tag path"), "environment/rocks");
    await userEvent.click(screen.getByRole("button", { name: "Add tag" }));

    expect(bulkMutate).toHaveBeenCalledTimes(1);
    expect(bulkMutate.mock.calls[0][0]).toEqual({
      asset_ids: [7],
      action: "add_tag",
      path: "environment/rocks",
    });
  });
});
