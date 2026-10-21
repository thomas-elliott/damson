# Archived documents

Historical inputs, kept verbatim. Everything here was the source of truth at
some point; the work it drove has since landed and the live state of the
project is documented elsewhere.

Nothing in this directory is maintained against the code. Section numbers,
file paths and API shapes have drifted and will keep drifting. Read these for
the *why* and the trade-offs considered — never as a description of what the
system currently does.

## Original design documents

These drove the M0–M7 MVP build.

| Doc | What it is | Replaced by |
|---|---|---|
| `product-design-doc.md` | Original product brief: problem statement, goals, domain model, MVP scope, open questions | [`docs/architecture.md`](../architecture.md) §1 + the live code |
| `technical-design.md` | Original technical design: stack, schema, storage layout, auth, pipelines, API surface, milestones | [`docs/architecture.md`](../architecture.md) for current state; [`docs/specs/history/`](../specs/history/README.md) for what has landed |
| `ui-design.md` | Original frontend design brief: information architecture, palette, interaction patterns | [`docs/design-system.md`](../design-system.md) for the implementation contract; the brief's IA still applies |

## Point-in-time research

Dated research deliverables and scoping briefs. Each fed a spec in
[`docs/specs/`](../specs/), which is the better record of what was actually
agreed and built.

| Doc | What it is | Fed into |
|---|---|---|
| `preview-research.md` | Survey of per-file preview approaches across the file taxonomy | `specs/2026-06-18-file-previews.md` |
| `preview-feasibility-brief.md` | Scoping brief narrowing that survey to a feasible slice | `specs/2026-06-18-file-previews.md` |
| `preview-feasibility-research-prompt.md` | The research prompt that produced `preview-research.md` | — |
| `preview-processing-queue-brief.md` | Brief on making preview work observable and retryable | `specs/2026-06-18-preview-processing-queue.md` |
| `dependency-audit.md` | Pre-alpha major-version upgrade audit, audited 2026-04-26 | The pre-alpha bump pass. Stale — it predates the SQLite removal and the React Router / Vitest / TypeScript majors |

For the **current** state of the project see:

- [`README.md`](../../README.md) — running and developing Damson
- [`docs/architecture.md`](../architecture.md) — architecture, data model, API
- [`docs/CONVENTIONS.md`](../CONVENTIONS.md) — coding standards
- [`docs/design-system.md`](../design-system.md) — frontend design system
- [`docs/progress.md`](../progress.md) — known gaps, and where the code diverges from these drafts
- [`docs/specs/`](../specs/) — signed-off specs driving post-MVP work
- [`AGENTS.md`](../../AGENTS.md) — quick-start guidance for agentic coding
