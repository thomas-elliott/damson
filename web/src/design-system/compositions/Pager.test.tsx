import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Pager } from "@/design-system/compositions/Pager";

// The page numbers and gaps, in order, as the list shows them.
function shown(page: number, pageCount: number) {
  const { unmount } = render(<Pager page={page} pageCount={pageCount} onPageChange={() => {}} />);
  // The gaps are aria-hidden, so ask for hidden items too.
  const items = screen.getAllByRole("listitem", { hidden: true }).map((li) => li.textContent);
  unmount();
  return items.join(" ");
}

describe("Pager page numbers", () => {
  it("lists every page when there are few", () => {
    expect(shown(1, 1)).toBe("1");
    expect(shown(2, 4)).toBe("1 2 3 4");
  });

  it("keeps the ends and a window round the current page", () => {
    expect(shown(6, 12)).toBe("1 … 5 6 7 … 12");
    expect(shown(1, 12)).toBe("1 2 … 12");
    expect(shown(12, 12)).toBe("1 … 11 12");
  });

  it("shows a lone skipped page rather than a gap for it", () => {
    expect(shown(4, 12)).toBe("1 2 3 4 5 … 12");
  });
});

describe("Pager", () => {
  it("marks the current page and shows the page count", () => {
    render(<Pager page={3} pageCount={5} onPageChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Page 3" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Page 2" })).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("navigation", { name: "Pagination" })).toHaveTextContent("Page 3 of 5");
  });

  it("disables previous on the first page and next on the last", () => {
    const { rerender } = render(<Pager page={1} pageCount={2} onPageChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Next page" })).toBeEnabled();
    rerender(<Pager page={2} pageCount={2} onPageChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Previous page" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
  });

  it("reports the page the user picks", async () => {
    const onPageChange = vi.fn();
    render(<Pager page={2} pageCount={5} onPageChange={onPageChange} />);
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
    await userEvent.click(screen.getByRole("button", { name: "Page 5" }));
    expect(onPageChange.mock.calls).toEqual([[3], [1], [5]]);
  });

  it("shows the page size selector only when asked", () => {
    const { rerender } = render(<Pager page={1} pageCount={2} onPageChange={() => {}} />);
    expect(screen.queryByRole("combobox", { name: "Page size" })).toBeNull();
    rerender(
      <Pager
        page={1}
        pageCount={2}
        onPageChange={() => {}}
        pageSize={50}
        pageSizes={[25, 50]}
        onPageSizeChange={() => {}}
      />,
    );
    expect(screen.getByRole("combobox", { name: "Page size" })).toHaveTextContent("50 per page");
  });
});
