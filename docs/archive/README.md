# Archived design documents

These are the design documents that drove the M0–M7 build of Damson. They
were the source of truth while the system was being built; the code has
since landed and the live state of the project is documented elsewhere.

They are kept here verbatim as a historical reference — the *why* and the
trade-offs we considered. They are **not** maintained against the code, and
section numbers, file paths, and API shapes will drift over time.

| Doc | What it is | Replaced by |
|---|---|---|
| `product-design-doc.md` | Original product brief: problem statement, goals, domain model, MVP scope, open questions | [`docs/architecture.md`](../architecture.md) §1 + the live code |
| `technical-design.md` | Original technical design: stack, schema, storage layout, auth, pipelines, API surface, milestones | [`docs/architecture.md`](../architecture.md) for current state; [`docs/progress.md`](../progress.md) for milestone status |
| `ui-design.md` | Original frontend design brief: information architecture, palette, interaction patterns | [`docs/design-system.md`](../design-system.md) for the implementation contract; the brief's IA still applies |

For the **current** state of the project see:

- [`README.md`](../../README.md) — running and developing Damson
- [`docs/architecture.md`](../architecture.md) — architecture, data model, API
- [`docs/CONVENTIONS.md`](../CONVENTIONS.md) — coding standards
- [`docs/design-system.md`](../design-system.md) — frontend design system
- [`docs/progress.md`](../progress.md) — milestone-by-milestone implementation status
- [`AGENTS.md`](../../AGENTS.md) — quick-start guidance for agentic coding
