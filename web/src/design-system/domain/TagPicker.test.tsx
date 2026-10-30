import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { TagPicker } from "./TagPicker";

const TAGS = [
  { path: "3d", count: 10 },
  { path: "3d/character", count: 6 },
  { path: "3d/character/humanoid/human", count: 4 },
  { path: "audio/sound-fx", count: 3 },
];

describe("TagPicker", () => {
  it("fills in levels that exist only as part of a deeper path", async () => {
    render(<TagPicker tags={TAGS} selected={[]} onSelectedChange={() => {}} />);
    const top = screen.getByRole("list", { name: "Top-level tags" });
    // "audio" is never a tag itself, only the start of audio/sound-fx.
    expect(within(top).getByRole("button", { name: /audio/ })).toBeInTheDocument();

    await userEvent.click(within(top).getByRole("button", { name: /3d/ }));
    await userEvent.click(screen.getByRole("button", { name: /character/ }));
    const humanoid = screen.getByRole("list", { name: "Tags in 3d/character" });
    expect(within(humanoid).getByRole("button", { name: /humanoid/ })).toBeInTheDocument();
  });

  it("offers no tick box on the top level", () => {
    render(<TagPicker tags={TAGS} selected={[]} onSelectedChange={() => {}} />);
    const top = screen.getByRole("list", { name: "Top-level tags" });
    expect(within(top).queryByRole("checkbox")).toBeNull();
  });

  it("ticks and unticks a tag", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <TagPicker tags={TAGS} selected={[]} onSelectedChange={onChange} />,
    );
    await userEvent.click(screen.getByRole("button", { name: /3d/ }));
    await userEvent.click(screen.getByRole("checkbox", { name: "Select 3d/character" }));
    expect(onChange).toHaveBeenLastCalledWith(["3d/character"]);

    rerender(<TagPicker tags={TAGS} selected={["3d/character"]} onSelectedChange={onChange} />);
    await userEvent.click(screen.getByRole("checkbox", { name: "Select 3d/character" }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it("opens on the column of the first ticked tag", () => {
    render(
      <TagPicker
        tags={TAGS}
        selected={["3d/character/humanoid/human"]}
        onSelectedChange={() => {}}
      />,
    );
    expect(
      screen.getByRole("checkbox", { name: "Select 3d/character/humanoid/human" }),
    ).toBeChecked();
  });

  it("lists matching paths when filtering", async () => {
    render(<TagPicker tags={TAGS} selected={[]} onSelectedChange={() => {}} />);
    await userEvent.type(screen.getByRole("searchbox", { name: "Filter tags" }), "human");
    const results = screen.getByRole("list", { name: "Matching tags" });
    // humanoid is implicit, so it has no count; human's count is 4.
    expect(
      within(results)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["3d / character / humanoid", "3d / character / humanoid / human4"]);
  });

  it("says when the matches are cut short", async () => {
    const many = Array.from({ length: 60 }, (_, i) => ({ path: `3d/props/item-${i}`, count: 1 }));
    render(<TagPicker tags={many} selected={[]} onSelectedChange={() => {}} />);
    await userEvent.type(screen.getByRole("searchbox", { name: "Filter tags" }), "item");
    expect(screen.getByText(/Showing 50 of 60 matches/)).toBeInTheDocument();
  });

  it("steps up a level from a column's heading", async () => {
    render(
      <TagPicker
        tags={TAGS}
        selected={["3d/character/humanoid/human"]}
        onSelectedChange={() => {}}
      />,
    );
    expect(screen.getByRole("list", { name: "Tags in 3d/character/humanoid" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Up a level from\s*humanoid/ }));
    expect(
      screen.queryByRole("list", { name: "Tags in 3d/character/humanoid" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Tags in 3d/character" })).toBeInTheDocument();
  });
});
