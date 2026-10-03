import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/api";

const fetchJson = vi.fn();

vi.mock("@/api", async (importActual) => ({
  ...(await importActual<typeof import("@/api")>()),
  fetchJson: (url: string, init?: RequestInit) => fetchJson(url, init),
}));

// The cover goes through the authenticated blob loader, which this test has no
// need to exercise.
vi.mock("@/components/filebrowser/useFileObjectUrl", () => ({
  useBlobUrl: () => undefined,
}));

const { AddByLocationDialog } = await import("./AddByLocationDialog");

type Route = (init?: RequestInit) => unknown;
let routes: Record<string, Route>;

function renderDialog(onOpenAsset = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <AddByLocationDialog onOpenChange={() => {}} onOpenAsset={onOpenAsset} />
    </QueryClientProvider>,
  );
  return onOpenAsset;
}

function postedBodies(): unknown[] {
  return fetchJson.mock.calls
    .filter(([url]) => url === "/api/assets/references")
    .map(([, init]) => JSON.parse((init as RequestInit).body as string));
}

describe("AddByLocationDialog", () => {
  beforeEach(() => {
    fetchJson.mockReset();
    routes = {};
    fetchJson.mockImplementation(async (url: string, init?: RequestInit) => {
      const key = Object.keys(routes).find((k) => url.startsWith(k));
      if (!key) throw new Error(`unrouted ${url}`);
      return routes[key]!(init);
    });
  });

  it("shows the store match before adding", async () => {
    routes["/api/metadata/preview"] = () => ({
      title: "POLYGON Fantasy Kingdom",
      publisher: "Synty Studios",
      cover_url: "https://cdn.example.com/c.png",
    });
    renderDialog();
    fireEvent.change(screen.getByLabelText("Store link"), {
      target: { value: "https://assetstore.unity.com/packages/k-1" },
    });
    expect(await screen.findByText("POLYGON Fantasy Kingdom")).toBeInTheDocument();
    expect(screen.getByText("Synty Studios")).toBeInTheDocument();
    expect(postedBodies()).toHaveLength(0);
  });

  it("says a link it can't read will still be recorded", async () => {
    routes["/api/metadata/preview"] = () => {
      throw new ApiError(502, {}, "could not fetch listing");
    };
    renderDialog();
    fireEvent.change(screen.getByLabelText("Store link"), {
      target: { value: "https://example.com/broken" },
    });
    expect(await screen.findByText(/It will still be recorded/)).toBeInTheDocument();
  });

  it("records a path and offers to open the new asset", async () => {
    routes["/api/assets/references"] = () => ({
      outcome: "created",
      asset_id: 41,
      name: "POLYGON_Fantasy_Kingdom",
    });
    const onOpenAsset = renderDialog();
    fireEvent.change(screen.getByLabelText("Path"), {
      target: { value: " /mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    expect(await screen.findByText("Added to your library")).toBeInTheDocument();
    expect(postedBodies()).toEqual([
      { location_path: "/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage" },
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Open asset" }));
    expect(onOpenAsset).toHaveBeenCalledWith(41);
  });

  it("says a known path was updated, not added", async () => {
    routes["/api/assets/references"] = () => ({
      outcome: "updated",
      asset_id: 41,
      name: "Kingdom",
    });
    renderDialog();
    fireEvent.change(screen.getByLabelText("Path"), { target: { value: "/mnt/nfs/Kingdom" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByText(/^Updated/)).toBeInTheDocument();
    expect(screen.queryByText("Added to your library")).not.toBeInTheDocument();
  });

  it("shows the existing asset when the store link is already in the library", async () => {
    routes["/api/metadata/preview"] = () => ({});
    routes["/api/assets/references"] = () => ({
      outcome: "exists",
      asset_id: 9,
      name: "Imported Kingdom",
    });
    const onOpenAsset = renderDialog();
    fireEvent.change(screen.getByLabelText("Store link"), {
      target: { value: "https://assetstore.unity.com/packages/k-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByText("Already in your library")).toBeInTheDocument();
    expect(screen.getByText("Imported Kingdom")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open asset" }));
    expect(onOpenAsset).toHaveBeenCalledWith(9);
  });

  it("asks for a name only when the server can't derive one", async () => {
    routes["/api/metadata/preview"] = () => {
      throw new ApiError(502, {}, "could not fetch listing");
    };
    let calls = 0;
    routes["/api/assets/references"] = () => {
      calls++;
      if (calls === 1) throw new ApiError(422, {}, "name is required");
      return { outcome: "created", asset_id: 5, name: "Hand Named" };
    };
    renderDialog();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Store link"), {
      target: { value: "https://example.com/broken" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    const nameField = await screen.findByLabelText("Name");
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    fireEvent.change(nameField, { target: { value: "Hand Named" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(screen.getByText("Added to your library")).toBeInTheDocument());
    expect(postedBodies()[1]).toEqual({
      source_url: "https://example.com/broken",
      name: "Hand Named",
    });
  });
});
