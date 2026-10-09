import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system/primitives/Tooltip";
import { FileBrowserToolbar } from "./FileBrowserToolbar";
import { fileTypeMeta } from "./fileTypes";

// Several filter chips render side by side, so each remove button must say
// which filter it removes.
describe("FileBrowserToolbar filter chips", () => {
  it("names the filter each remove button removes", () => {
    render(
      <TooltipProvider>
        <FileBrowserToolbar
          assetName="Pack"
          path={[]}
          onNavigate={vi.fn()}
          count={3}
          view="grid"
          onViewChange={vi.fn()}
          activeTypes={["texture", "audio"]}
          onToggleType={vi.fn()}
        />
      </TooltipProvider>,
    );
    for (const ftype of ["texture", "audio"] as const) {
      const label = `Remove type filter: ${fileTypeMeta(ftype).label}`;
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
  });
});
