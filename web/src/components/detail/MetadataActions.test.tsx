import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AssetDetailView, MetadataFetchResponse, MetadataView } from "@/hooks/useDataQueries";

const fetchMutate = vi.fn();
const applyMutate = vi.fn();
const searchMutate = vi.fn();
const previewMutate = vi.fn();

// The component's success handling is what we're testing, so the mocked
// mutations invoke onSuccess synchronously with a per-test canned response.
let fetchResponse: MetadataFetchResponse;
let previewData: MetadataView | undefined;

vi.mock("@/hooks/useMutations", () => ({
  usePreviewMetadata: () => ({
    mutate: previewMutate,
    data: previewData,
    isPending: false,
    isError: false,
  }),
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

const blobUrlMock = vi.fn<(url?: string) => string | undefined>(() => "blob:fake");
vi.mock("@/components/filebrowser/useFileObjectUrl", () => ({
  useBlobUrl: (url?: string) => blobUrlMock(url),
}));

import { StoreSuggestions, MetadataActions } from "./MetadataActions";

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
    previewMutate.mockClear();
    blobUrlMock.mockClear();
    previewData = undefined;
  });

  it("renders candidate covers through the SSRF-guarded image proxy", () => {
    const withCover = {
      ...asset,
      source_metadata: {
        matched: false,
        store: "",
        candidates: [
          {
            title: "Fantasy Pack Deluxe",
            listing_url: "https://x/right",
            store: "unity",
            cover_url: "https://cdn.unity.com/a b.png?v=1",
          },
        ],
      },
    } as AssetDetailView;

    const { container } = render(<StoreSuggestions asset={withCover} />);

    // A bare <img src={cover_url}> would be blocked by the CSP; the cover must
    // go through the proxy endpoint with the external URL encoded.
    expect(blobUrlMock).toHaveBeenCalledWith(
      `/api/metadata/image?url=${encodeURIComponent("https://cdn.unity.com/a b.png?v=1")}`,
    );
    expect(container.querySelector("img")).toHaveAttribute("src", "blob:fake");
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

  it("surfaces stored candidates and re-fetches the picked one", () => {
    fetchResponse = { store: "unity", listing_url: "", fetched: null, applied: [], conflicts: [] };
    const withMeta = {
      ...asset,
      source_metadata: {
        matched: false,
        store: "",
        candidates: [
          { title: "Fantasy Pack Deluxe", listing_url: "https://x/right", store: "unity" },
        ],
      },
    } as AssetDetailView;

    render(<StoreSuggestions asset={withMeta} />);
    // The stored candidate must be pickable.
    expect(screen.getByText(/looks like a match/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Fantasy Pack Deluxe/i }));
    expect(fetchMutate).toHaveBeenCalledWith({ id: 7, url: "https://x/right" });
  });

  it("hides candidates that share nothing with the asset's name", () => {
    const unrelated = {
      ...asset,
      source_metadata: {
        matched: false,
        store: "",
        candidates: [
          { title: "Gaia Pro 2023", listing_url: "https://x/gaia", store: "unity" },
          { title: "Behavior Designer", listing_url: "https://x/bd", store: "unity" },
        ],
      },
    } as AssetDetailView;
    const { container } = render(<StoreSuggestions asset={unrelated} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows nothing for an asset that already has its listing", () => {
    const matched = {
      ...asset,
      source_metadata: {
        matched: true,
        store: "unity",
        candidates: [{ title: "Fantasy Pack Deluxe", listing_url: "https://x/d", store: "unity" }],
      },
    } as AssetDetailView;
    const { container } = render(<StoreSuggestions asset={matched} />);
    expect(container).toBeEmptyDOMElement();
  });

  const withCandidate = {
    ...asset,
    source_metadata: {
      matched: false,
      store: "",
      candidates: [
        { title: "Fantasy Pack Deluxe", listing_url: "https://x/right", store: "unity" },
      ],
    },
  } as AssetDetailView;

  it("fetches a candidate's details only when expanded", () => {
    previewData = undefined;
    render(<StoreSuggestions asset={withCandidate} />);
    // Not fetched until expanded — the eager version would fail this guard.
    expect(previewMutate).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Expand details/i }));
    expect(previewMutate).toHaveBeenCalledWith("https://x/right");
  });

  it("shows the fetched description only once expanded", () => {
    previewData = { description: "A big pack of dungeon tiles." };
    render(<StoreSuggestions asset={withCandidate} />);
    // Collapsed by default — details stay hidden until the row is expanded.
    expect(screen.queryByText(/dungeon tiles/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Expand details/i }));
    expect(screen.getByText(/dungeon tiles/i)).toBeInTheDocument();
  });
});
