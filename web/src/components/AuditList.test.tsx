import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { AuditList } from "./AuditList";

const base = { created_at: "2026-10-29T13:10:00Z", user_email: "a@example.com" };

describe("AuditList targets", () => {
  it("names a target, with its id beside it, and links it", () => {
    render(
      <MemoryRouter>
        <AuditList
          linkTargets
          entries={[
            {
              ...base,
              id: 1,
              action_type: "project.create",
              target_type: "project",
              target_id: 7,
              target_name: "Ashfall",
            },
          ]}
        />
      </MemoryRouter>,
    );
    const link = screen.getByRole("link", { name: "project Ashfall #7" });
    expect(link).toHaveAttribute("href", "/projects/7");
  });

  it("falls back to the id for a target that is gone", () => {
    render(
      <MemoryRouter>
        <AuditList
          entries={[
            { ...base, id: 1, action_type: "asset.delete", target_type: "asset", target_id: 297 },
          ]}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText("asset #297")).toBeInTheDocument();
  });
});
