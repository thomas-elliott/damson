import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AssetDetailView } from "@/hooks/useDataQueries";

const patchMutate = vi.fn();

vi.mock("@/hooks/useMutations", () => ({
  usePatchAsset: () => ({
    mutate: (vars: unknown, opts?: { onSuccess?: () => void }) => {
      patchMutate(vars);
      opts?.onSuccess?.();
    },
    isPending: false,
    error: null,
  }),
}));

const { DetailsSection } = await import("./DetailsSection");

function baseAsset(over: Partial<AssetDetailView> = {}): AssetDetailView {
  return {
    id: 1,
    name: "Nature Pack",
    description: "",
    source_url: "",
    needs_review: false,
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

// The section starts collapsed, so every read assertion opens it first.
function renderOpen(asset: AssetDetailView) {
  render(<DetailsSection asset={asset} />);
  // Exact: "Edit details" would also match a loose pattern.
  fireEvent.click(screen.getByRole("button", { name: "Details" }));
}

describe("DetailsSection", () => {
  beforeEach(() => patchMutate.mockClear());

  it("renders the storefront fields it has and omits the ones it does not", () => {
    renderOpen(
      baseAsset({
        publisher: "JustCreate",
        store: "unity",
        released_at: "2020-02-11T09:47:13Z",
        rating: 4.85,
        rating_count: 126,
      }),
    );

    expect(screen.getByText("JustCreate")).toBeInTheDocument();
    expect(screen.getByText("Unity Asset Store")).toBeInTheDocument();
    expect(screen.getByText("4.85 / 5 (126)")).toBeInTheDocument();
    // No engine or price was set, so those rows must not appear at all rather
    // than rendering as blanks.
    expect(screen.queryByText("Engine")).not.toBeInTheDocument();
    expect(screen.queryByText("Price")).not.toBeInTheDocument();
  });

  it("shows a free asset as Free rather than 0", () => {
    renderOpen(baseAsset({ price: "0", price_currency: "USD" }));
    expect(screen.getByText("Free")).toBeInTheDocument();
    expect(screen.queryByText("0 USD")).not.toBeInTheDocument();
  });

  it("prompts rather than showing an empty list when nothing is set", () => {
    renderOpen(baseAsset());
    expect(screen.getByText(/fetch storefront metadata/i)).toBeInTheDocument();
  });

  it("links the publisher out when a publisher URL is known", () => {
    renderOpen(
      baseAsset({ publisher: "JustCreate", publisher_url: "https://justcreate3d.studio/" }),
    );
    const link = screen.getByRole("link", { name: /JustCreate/ });
    expect(link).toHaveAttribute("href", "https://justcreate3d.studio/");
    // Opening a third-party link must not leak the referrer or window handle.
    expect(link).toHaveAttribute("rel", expect.stringContaining("noopener"));
  });

  it("renders the store's own attributes in the order the store listed them", () => {
    renderOpen(
      baseAsset({
        store: "unity",
        source_metadata: {
          matched: true,
          metadata: {
            attributes: [
              { key: "download_size", label: "Download size", value: "1.9 MB" },
              { key: "asset_count", label: "Assets", value: "57" },
              { key: "package_type", label: "Package type", value: "unitypackage" },
            ],
          },
        },
      }),
    );
    expect(screen.getByText("From the store listing")).toBeInTheDocument();
    const labels = screen.getAllByRole("term").map((el) => el.textContent);
    const sheet = labels.slice(labels.indexOf("Download size"));
    expect(sheet).toEqual(["Download size", "Assets", "Package type"]);
  });

  it("sends every field on save, with a blank string for the ones being cleared", () => {
    render(<DetailsSection asset={baseAsset({ publisher: "Wrong Studio", engine: "unity" })} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));

    const publisher = screen.getByDisplayValue("Wrong Studio");
    fireEvent.change(publisher, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    expect(patchMutate).toHaveBeenCalledTimes(1);
    const payload = patchMutate.mock.calls[0][0] as Record<string, unknown>;
    expect(payload.id).toBe(1);
    // Blank clears the column; the untouched engine is sent unchanged.
    expect(payload.publisher).toBe("");
    expect(payload.engine).toBe("unity");
    // A field the asset never had is sent blank, which is a no-op server-side.
    expect(payload.price).toBe("");
  });

  it("flags an available store update in the section header", () => {
    render(
      <DetailsSection
        asset={baseAsset({ update_available: true, latest_store_version: "1.24" })}
      />,
    );
    expect(screen.getByText(/update: 1\.24/)).toBeInTheDocument();
  });

  it("shows no update pill when the asset is current", () => {
    render(<DetailsSection asset={baseAsset({ latest_store_version: "1.24" })} />);
    expect(screen.queryByText(/update:/)).not.toBeInTheDocument();
  });

  it("seeds the date inputs as plain dates, not timestamps", () => {
    render(<DetailsSection asset={baseAsset({ released_at: "2020-02-11T09:47:13Z" })} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    expect(screen.getByDisplayValue("2020-02-11")).toBeInTheDocument();
  });
});
