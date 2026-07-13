import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AssetDetailView, MetadataFetchResponse } from "@/hooks/useDataQueries";

const fetchMutate = vi.fn();
const applyMutate = vi.fn();
const searchMutate = vi.fn();

// The component's success handling is what we're testing, so the mocked
// mutations invoke onSuccess synchronously with a per-test canned response.
let fetchResponse: MetadataFetchResponse;

vi.mock("@/hooks/useMutations", () => ({
  useFetchMetadata: () => ({
    mutate: (
      vars: { id: number; url?: string },
      opts?: { onSuccess?: (r: MetadataFetchResponse) => void },
    ) => {
      fetchMutate(vars);
      opts?.onSuccess?.(fetchResponse);
    },
    isPending: false,
    isError: false,
  }),
  useApplyMetadata: () => ({
    mutate: (
      vars: { id: number; fields: string[] },
      opts?: { onSuccess?: (r: MetadataFetchResponse) => void },
    ) => {
      applyMutate(vars);
      opts?.onSuccess?.({
        store: "generic",
        listing_url: "",
        fetched: null,
        applied: vars.fields,
        conflicts: [],
      });
    },
    isPending: false,
  }),
  useSearchMetadata: () => ({
    mutate: searchMutate,
    isPending: false,
    isError: false,
    data: { query: "", candidates: [] },
  }),
}));

import { MetadataActions } from "./MetadataActions";

const asset = {
  id: 7,
  name: "Fantasy Pack",
  description: "old description",
  source_url: "https://assetstore.unity.com/packages/x-1",
} as AssetDetailView;

describe("MetadataActions", () => {
  beforeEach(() => {
    fetchMutate.mockClear();
    applyMutate.mockClear();
    searchMutate.mockClear();
  });

  it("opens the overwrite-confirm dialog on conflicts and applies the confirmed field", () => {
    fetchResponse = {
      store: "unity",
      listing_url: "https://assetstore.unity.com/packages/x-1",
      fetched: null,
      applied: [],
      conflicts: [
        { field: "description", current: "old description", proposed: "fetched description" },
      ],
    };

    render(<MetadataActions asset={asset} />);
    fireEvent.click(screen.getByRole("button", { name: /Fetch/i }));

    expect(fetchMutate).toHaveBeenCalledWith({ id: 7, url: undefined });
    // The conflict must surface as the confirm dialog.
    expect(screen.getByText("Confirm overwrites")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Apply selected/i }));
    expect(applyMutate).toHaveBeenCalledWith({ id: 7, fields: ["description"] });
  });

  it("does not open the confirm dialog when there are no conflicts", () => {
    fetchResponse = {
      store: "unity",
      listing_url: "https://assetstore.unity.com/packages/x-1",
      fetched: null,
      applied: ["description", "thumbnail"],
      conflicts: [],
    };

    render(<MetadataActions asset={asset} />);
    fireEvent.click(screen.getByRole("button", { name: /Fetch/i }));

    // A component that always opened the dialog would fail this assertion.
    expect(screen.queryByText("Confirm overwrites")).not.toBeInTheDocument();
    expect(applyMutate).not.toHaveBeenCalled();
  });
});
