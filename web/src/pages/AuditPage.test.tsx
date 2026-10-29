import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AdminAuditPage } from "./AdminAuditPage";
import { AuditPage } from "./AuditPage";

// Each test sets the feed's total; the hooks record what they were asked for.
const feed = vi.hoisted(() => ({
  total: 0,
  error: null as Error | null,
  failed: false,
  reload: () => {},
  requests: [] as Record<string, unknown>[],
}));

vi.mock("@/hooks/useDataQueries", () => {
  const audit = (filters: Record<string, unknown>) => {
    feed.requests.push(filters);
    return {
      data: [],
      total: feed.total,
      loading: false,
      placeholder: false,
      error: feed.error,
      failed: feed.failed,
      reload: feed.reload,
    };
  };
  return {
    useMe: () => ({ data: { id: 1, role: "sysadmin" }, loading: false }),
    useLibraryAudit: audit,
    useAdminAudit: audit,
  };
});
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));

function Location() {
  const { search } = useLocation();
  return <output aria-label="query">{search}</output>;
}

beforeEach(() => {
  feed.total = 120;
  feed.error = null;
  feed.failed = false;
  feed.reload = vi.fn();
  feed.requests = [];
});

const lastRequest = () => feed.requests[feed.requests.length - 1];
const query = () => screen.getByLabelText("query").textContent;

function mount(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/admin/audit" element={<AdminAuditPage />} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
}

describe("Audit pages' headings", () => {
  it("names the audit log with an h1", () => {
    mount("/audit");
    expect(screen.getByRole("heading", { level: 1, name: "Audit log" })).toBeInTheDocument();
  });
});

describe("AuditPage paging", () => {
  it("asks for the page in the URL", () => {
    mount("/audit?page=3&page_size=25");
    expect(lastRequest()).toMatchObject({ limit: 25, offset: 50 });
    expect(screen.getByRole("navigation", { name: "Audit log pages" })).toHaveTextContent(
      "Page 3 of 5",
    );
  });

  it("writes the next page to the URL", async () => {
    mount("/audit");
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(query()).toBe("?page=2");
    expect(lastRequest()).toMatchObject({ limit: 50, offset: 50 });
  });

  it("moves a page past the end to the last page", () => {
    mount("/audit?page=9");
    expect(query()).toBe("?page=3");
  });

  // A failed fetch has no total, which must not read as an empty feed.
  it("keeps the page when the fetch fails", () => {
    feed.total = 0;
    feed.error = new Error("network");
    mount("/audit?page=3");
    expect(query()).toBe("?page=3");
  });

  it("goes back to page 1 when a chip changes", async () => {
    mount("/audit?page=2");
    await userEvent.click(screen.getByRole("button", { name: "Merges" }));
    expect(query()).toBe("?action=asset.merge");
    expect(lastRequest()).toMatchObject({ action_type: "asset.merge", offset: 0 });
  });

  it("searches on the server from page 1", async () => {
    mount("/audit?page=2");
    await userEvent.type(screen.getByRole("searchbox", { name: /filter audit log/i }), "b");
    expect(query()).toBe("?q=b");
    expect(lastRequest()).toMatchObject({ q: "b", offset: 0 });
  });
});

describe("AdminAuditPage paging", () => {
  it("asks for the page in the URL", () => {
    mount("/admin/audit?scope=all&page=2");
    expect(lastRequest()).toMatchObject({ scope: "all", limit: 50, offset: 50 });
  });

  it("goes back to page 1 when the scope changes", async () => {
    mount("/admin/audit?page=2");
    await userEvent.click(screen.getByRole("radio", { name: "All libraries" }));
    expect(query()).toBe("?scope=all");
    expect(lastRequest()).toMatchObject({ scope: "all", offset: 0 });
  });
});

describe.each([
  ["AuditPage", "/audit"],
  ["AdminAuditPage", "/admin/audit"],
])("%s load failure", (_name, path) => {
  it("shows the error with a Retry instead of an empty log", async () => {
    feed.total = 0;
    feed.error = new Error("network");
    feed.failed = true;
    mount(path);
    expect(screen.getByText("Couldn't load the audit log")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(feed.reload).toHaveBeenCalled();
  });

  it("keeps showing the log when only a refetch failed", () => {
    feed.error = new Error("network");
    mount(path);
    expect(screen.queryByText("Couldn't load the audit log")).toBeNull();
  });
});
