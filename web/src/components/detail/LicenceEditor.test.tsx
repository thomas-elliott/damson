import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { LicenceView } from "@/hooks/useDataQueries";

const fetchJson = vi.fn();

vi.mock("@/api", async (importActual) => ({
  ...(await importActual<typeof import("@/api")>()),
  fetchJson: (url: string, init?: RequestInit) => fetchJson(url, init),
}));

const { LicenceEditor } = await import("./LicenceEditor");

const CUSTOM: LicenceView = {
  id: 7,
  asset_id: 1,
  detected_type: "custom",
  detection_confidence: 0,
  original_text: "Use in games only.",
  attributes: {
    commercial_use: null,
    modification: null,
    redistribution: null,
    attribution_required: null,
    share_alike: null,
    open_source_required: null,
    custom_terms: "games only",
  },
  status: "warn",
};

const MIT_DETECTION = {
  detected_type: "MIT",
  detection_confidence: 0.98,
  attributes: {
    commercial_use: true,
    modification: true,
    redistribution: true,
    attribution_required: true,
    share_alike: false,
    open_source_required: false,
  },
  status: "ok",
};

const MIT_WITH_TERMS: LicenceView = {
  ...CUSTOM,
  detected_type: "MIT",
  detection_confidence: 0.98,
  original_text: "MIT License ...",
  attributes: { ...MIT_DETECTION.attributes, custom_terms: "no resale" },
  status: "ok",
};

const UNKNOWN_DETECTION = {
  detected_type: "unknown",
  detection_confidence: 0,
  attributes: { ...CUSTOM.attributes, custom_terms: undefined },
  status: "unk",
};

function renderEditor(existing: LicenceView) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <LicenceEditor assetId={1} existing={existing} />
    </QueryClientProvider>,
  );
}

function openEdit() {
  fireEvent.click(screen.getByRole("button", { name: "Edit licence" }));
}

function setAttr(label: string, value: "Yes" | "No" | "Unknown") {
  fireEvent.click(within(screen.getByRole("radiogroup", { name: label })).getByText(value));
}

function changeText(text: string) {
  fireEvent.change(screen.getByRole("textbox"), { target: { value: text } });
}

function putBody() {
  const call = fetchJson.mock.calls.find(([, init]) => init?.method === "PUT");
  return call ? JSON.parse(call[1].body as string) : undefined;
}

function respondToDetect(result: unknown) {
  fetchJson.mockImplementation((url: string) =>
    url.endsWith("/detect") ? Promise.resolve(result) : Promise.resolve({}),
  );
}

describe("LicenceEditor editing a set licence", () => {
  beforeEach(() => {
    fetchJson.mockReset();
    fetchJson.mockResolvedValue({});
  });

  it("opens with the licence's text and attributes, and Cancel changes nothing", () => {
    renderEditor({ ...CUSTOM, attributes: { ...CUSTOM.attributes, commercial_use: false } });
    openEdit();

    expect(screen.getByRole("textbox")).toHaveValue("Use in games only.");
    const commercial = screen.getByRole("radiogroup", { name: "Commercial use" });
    expect(within(commercial).getByRole("radio", { name: "No" })).toHaveAttribute(
      "aria-checked",
      "true",
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel licence edit" }));
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(fetchJson).not.toHaveBeenCalled();
  });

  it("saves a changed attribute with the licence's type, keeping custom terms", async () => {
    renderEditor(CUSTOM);
    openEdit();
    setAttr("Commercial use", "No");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    await waitFor(() => expect(putBody()).toBeDefined());
    expect(putBody()).toEqual({
      original_text: "Use in games only.",
      detected_type: "custom",
      attributes: { ...CUSTOM.attributes, commercial_use: false },
    });
    await waitFor(() => expect(screen.queryByRole("textbox")).not.toBeInTheDocument());
  });

  it("saves an attribute edit of an unknown licence as custom", async () => {
    renderEditor({ ...CUSTOM, detected_type: "unknown", status: "unk" });
    openEdit();
    setAttr("Commercial use", "No");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    await waitFor(() => expect(putBody()?.detected_type).toBe("custom"));
  });

  it("needs Detect on changed text, then saves the detected licence", async () => {
    respondToDetect(MIT_DETECTION);
    renderEditor(CUSTOM);
    openEdit();
    changeText("MIT License ...");
    expect(screen.getByRole("button", { name: "Save licence" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Detect" }));
    await screen.findByText("98% match");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    await waitFor(() => expect(putBody()).toBeDefined());
    expect(putBody()).toEqual({
      original_text: "MIT License ...",
      detected_type: "MIT",
      attributes: MIT_DETECTION.attributes,
    });
  });

  it("keeps custom terms when Detect finds the licence's own type", async () => {
    respondToDetect(MIT_DETECTION);
    renderEditor(MIT_WITH_TERMS);
    openEdit();
    changeText("MIT License, 2025 ...");
    fireEvent.click(screen.getByRole("button", { name: "Detect" }));
    await screen.findByText("98% match");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    await waitFor(() => expect(putBody()).toBeDefined());
    expect(putBody().attributes).toEqual(MIT_WITH_TERMS.attributes);
  });

  it("needs Detect again when the text changes back after a Detect", async () => {
    respondToDetect(MIT_DETECTION);
    renderEditor(CUSTOM);
    openEdit();
    changeText("MIT License ...");
    fireEvent.click(screen.getByRole("button", { name: "Detect" }));
    await screen.findByText("98% match");

    changeText(CUSTOM.original_text);
    expect(screen.getByRole("button", { name: "Save licence" })).toBeDisabled();
  });

  it("keeps a custom licence custom when its edited text matches nothing", async () => {
    respondToDetect(UNKNOWN_DETECTION);
    renderEditor(CUSTOM);
    openEdit();
    setAttr("Redistribution", "No");
    changeText("Use in games and films only.");
    fireEvent.click(screen.getByRole("button", { name: "Detect" }));
    await screen.findByText("No match: will save as custom");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    await waitFor(() => expect(putBody()).toBeDefined());
    expect(putBody()).toEqual({
      original_text: "Use in games and films only.",
      detected_type: "custom",
      attributes: { ...CUSTOM.attributes, redistribution: false },
    });
  });

  it.each([
    ["a custom", CUSTOM],
    ["an unknown", { ...CUSTOM, detected_type: "unknown", status: "unk" } as LicenceView],
  ])("keeps Save disabled until %s licence is changed", (_, licence) => {
    renderEditor(licence);
    openEdit();
    const save = screen.getByRole("button", { name: "Save licence" });
    expect(save).toBeDisabled();

    setAttr("Commercial use", "No");
    expect(save).toBeEnabled();
    setAttr("Commercial use", "Unknown");
    expect(save).toBeDisabled();
  });

  it("clears a failed save's error when Detect starts", async () => {
    let finishDetect: (result: unknown) => void = () => {};
    fetchJson.mockImplementation((url: string) =>
      url.endsWith("/detect")
        ? new Promise((resolve) => (finishDetect = resolve))
        : Promise.reject(new Error("licence save failed")),
    );
    renderEditor(CUSTOM);
    openEdit();
    setAttr("Commercial use", "No");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));
    await screen.findByText("licence save failed");

    changeText("MIT License ...");
    fireEvent.click(screen.getByRole("button", { name: "Detect" }));
    await screen.findByText("Detecting…");
    expect(screen.queryByText("licence save failed")).not.toBeInTheDocument();

    finishDetect(MIT_DETECTION);
    await screen.findByText("98% match");
    expect(screen.queryByText("licence save failed")).not.toBeInTheDocument();
  });

  it("shows a failed Detect and leaves the draft as it was", async () => {
    fetchJson.mockImplementation((url: string) =>
      url.endsWith("/detect") ? Promise.reject(new Error("detect failed")) : Promise.resolve({}),
    );
    renderEditor(CUSTOM);
    openEdit();
    setAttr("Commercial use", "No");
    fireEvent.click(screen.getByRole("button", { name: "Detect" }));

    expect(await screen.findByText("detect failed")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveValue(CUSTOM.original_text);
    const save = screen.getByRole("button", { name: "Save licence" });
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() => expect(putBody()).toBeDefined());
    expect(putBody()).toEqual({
      original_text: CUSTOM.original_text,
      detected_type: "custom",
      attributes: { ...CUSTOM.attributes, commercial_use: false },
    });
  });

  it("keeps the form open with the error when the save fails", async () => {
    fetchJson.mockRejectedValue(new Error("licence save failed"));
    renderEditor(CUSTOM);
    openEdit();
    setAttr("Commercial use", "No");
    fireEvent.click(screen.getByRole("button", { name: "Save licence" }));

    expect(await screen.findByText("licence save failed")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeInTheDocument();
  });
});
