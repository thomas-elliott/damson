import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LicencePill } from "./LicencePill";

describe("LicencePill", () => {
  it("shows a licence with its own terms as a plain label, not a warning", () => {
    render(<LicencePill status="warn" spdx="Unity-EULA" />);
    expect(screen.getByText("Unity-EULA").querySelector("[aria-hidden]")).toBeNull();
  });

  it("keeps the dot for the statuses that carry one", () => {
    for (const status of ["ok", "err", "unk"] as const) {
      const { unmount } = render(<LicencePill status={status} spdx={status} />);
      expect(screen.getByText(status).querySelector("[aria-hidden]")).not.toBeNull();
      unmount();
    }
  });
});
