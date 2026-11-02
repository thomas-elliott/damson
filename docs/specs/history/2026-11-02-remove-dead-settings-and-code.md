# Remove dead settings, stale redirects and unused code

Covers no board Task.

## Why are we doing this

The repo is going public. Settings that do nothing, copy that promises a missing feature, a one-off
boot repair, old-address redirects and dead code should not be the first thing a reader finds.

## Steering

- "I am wanting to make it public on my github" — repository, 2026-10-21, session 477946a3-d94e-482d-a814-e822cc4041f0
- "Have licence compliance be a future feature - we should track it" — agreeing to: what happens to a project's Target distribution, whose copy promises licence compliance checks? — licence compliance, 2026-11-02, session 402f89ce-20d9-46ff-b3d1-1c7b2799a9e6

## Design

none — operator asked for the six removals from the pre-public audit: "create a plan for 1-6"

## Decisions

### What happens to "Dedupe enabled" and "Auto-update licence knowledge base"?
Answer: Remove both completely: the UI toggles and read-only rows, the API fields, the GORM fields and the docs rows. Drop `library_settings.dedupe_enabled` and `server_settings.licence_kb_auto_update` in a new migration.
By: recommended, operator chose "Drop the DB columns too" (2026-11-02)

### What happens to a project's Target distribution?
Answer: Keep the field, its column and its API. Licence compliance checks are a future feature. Until then the UI makes no promise of checks: the create form copy stops saying projects "track licence compliance" and that the field is "Used later for licence compliance checks", and the project page shows the value as a readable label, not the raw string. Rename the ProjectCard prop `engine` to a name that matches what it holds.
By: operator — "Have licence compliance be a future feature - we should track it", then chose "Keep it, honest copy" (2026-11-02)

### Is `useCreateLibrary` kept for a possible "New library" button?
Answer: No. Remove it now.
By: operator — chose "Remove it now" (2026-11-02)

### What goes with the gallery repair?
Answer: Remove `enrich.RepairGallery`, its tests and its boot call. The server no longer lists the `thumbs/` prefix at startup.
By: operator — "create a plan for 1-6", agreeing to the audit's verdict to remove it (2026-11-02)

### What goes with the old-address redirects?
Answer: Remove the `/processing` and `/library/:assetId/browse` routes, the `BrowseRedirect` component, the e2e assertions that visit those addresses, and the mentions of them in `docs/e2e-flows.md`.
By: operator — "create a plan for 1-6", agreeing to the audit's verdict to remove them (2026-11-02)

### What other dead code goes?
Answer: The `Slider` primitive, its export, its use in the Radix atoms story and the `@radix-ui/react-slider` dependency; and `queryKeys.assetLicence`.
By: operator — "create a plan for 1-6", agreeing to the audit's verdict to remove them (2026-11-02)

## Acceptance

- Library settings show no "Dedupe enabled" toggle or read-only row, and Settings › Server shows no "Auto-update licence knowledge base".
- `GET /api/settings` has no `dedupe_enabled` and `GET /api/admin/settings` has no `licence_kb_auto_update`.
- An existing database starts on the new build and loses both columns; a fresh database never has them.
- `docs/configuration.md` and `docs/how-to/separate-libraries.md` no longer mention either setting.
- The new-project form and the projects page say nothing about licence compliance checks, and a project page shows its target distribution as a readable label.
- Creating a project with a target distribution still stores and shows it.
- Server startup no longer runs the gallery repair.
- Visiting `/processing` or `/library/<id>/browse` no longer redirects, and `docs/e2e-flows.md` no longer lists those addresses as covered.
- `Slider`, `@radix-ui/react-slider`, `queryKeys.assetLicence` and `useCreateLibrary` are gone from `web/`.
- `just check` passes.

## Out of scope

- `ingest.BackfillCovers` and `preview.QueueNewlySupported`.
- Building licence compliance checks.
- Keeping or removing multi-user and multi-library support, including a "New library" button.
- Every other item from the pre-public audit.

## Start here

- `docs/CONVENTIONS.md` (Database Migrations)
- `internal/handlers/api/settings.go`
- `internal/handlers/api/admin.go`
- `web/src/pages/ProjectsPage.tsx`
- `docs/e2e-flows.md`

## Deferred follow-ups

None

## Implementation report

Mode: committing — branch `chore/remove-dead-settings-and-code`, one commit per stage (repo rule), no push, no PR.

### Stages

1. Remove "Dedupe enabled" and "Auto-update licence knowledge base": UI, API, GORM, docs, and a migration dropping both columns.
2. Target distribution: honest copy on the create form and projects page, readable label on the project page, rename the ProjectCard `engine` prop.
3. Remove `enrich.RepairGallery`, its tests and its boot call.
4. Remove the `/processing` and `/library/:assetId/browse` redirects, `BrowseRedirect`, their e2e assertions and their `docs/e2e-flows.md` mentions.
5. Remove `Slider` (primitive, export, story use, `@radix-ui/react-slider`), `queryKeys.assetLicence` and `useCreateLibrary`.

### Choices made

Written at close, not per stage.

- `web/src/lib/targetDistribution.ts`: value → label map mirroring `lib/stores.ts`; unknown values render raw.
- `web/src/design-system/domain/ProjectCard.tsx`: the prop is `distribution` (a display label); story fixtures now hold distribution labels instead of engine names.
- `web/src/pages/ProjectsPage.tsx`: new copy, "Projects pin specific asset versions for a game or build." and "How you plan to release the project."
- `database/migrations/034_drop_dead_settings.sql`: one migration drops both columns, mirroring `028_drop_theme_default.sql`.

### Review — stages 1…5

#### Verdict
No blockers

#### Checked
repo rules (AGENTS.md, docs/CONVENTIONS.md Database Migrations, frontend API discipline, query keys in `lib/queryKeys.ts`), documented hazards (schema change in both migration and GORM struct: both columns dropped in `034_drop_dead_settings.sql` and `models/settings.go`; migration wrapped in `BEGIN; … COMMIT;` and idempotent with `IF EXISTS`; `default:` tag hazard n/a for dropped fields; library_id scoping untouched; doc-update rule: `configuration.md`, `how-to/separate-libraries.md`, `e2e-flows.md` updated), mechanical shape conformance (`serverSettingsView`, `librarySettingsView`, patch requests, TS `ServerSettings`/`LibrarySettings`/`LibrarySettingsPatch`/`SettingsView` all lose the field on both sides), cross-boundary contract (API JSON ↔ TS types agree; `TARGET_DISTRIBUTIONS` values match `allowedDistributions` in `internal/handlers/api/projects.go:122`), leftover-reference sweep (no `dedupe_enabled`, `licence_kb_auto_update`, `RepairGallery`, `BrowseRedirect`, `/processing`, `Slider`, `react-slider`, `assetLicence`, `useCreateLibrary`, `engine=` outside migrations 005/034 and specs), logic (label fallback to raw value for unknown distributions; card shows label only when set; e2e `header` locator matches `<header>` at `ProjectDetailPage.tsx:128`), error handling, silent failure (old clients sending the dropped fields are ignored by JSON binding, not rejected — correct for removed optional fields), security (admin and library settings routes keep their guards; no auth change), state/idempotency (migration re-runnable), data (old audit diffs keep the removed keys as history; nothing renders them by name), tests (preflight audit test moved to `retain_original_packages`; new unit and e2e assertions cover the readable label and a stored distribution), smell baseline · issues in: none · n/a: change-type blocks (quick depth), risk-and-omissions sweep (quick depth), performance (removals only, cold paths), credentials (none touched) · validation: in-context, 0 candidates, 0 confirmed, 0 refuted

### Review — cleanup

#### Verdict
LOW · introduced: the project page now shows "Commercial" as a plain badge, but the screenshot used by both how-tos still shows the old mono `commercial` badge, and the how-tos give the target as raw values (`docs/images/project-page.png`, used at `docs/how-to/pin-versions.md:124` and `docs/how-to/track-licences.md:112`; values at `docs/how-to/pin-versions.md:84`, `docs/how-to/track-licences.md:114`, `docs/how-to/track-licences.md:116`). Fix: retake `project-page.png` and give the labels (Commercial, Open source, Personal, Unspecified) in place of the stored values. → fixed (the labels, in both how-tos); `project-page.png` → left for cleanup: retaking it needs the 9-pack sample dataset from `docs/images/README.md` (licences, labels, a pin behind latest), which `just seed` does not build, so it is larger than a local edit → accepted

#### Checked
repo rules (AGENTS.md doc-update rule, frontend API discipline, query keys only in `lib/queryKeys.ts`, schema change in both migration and GORM struct), documented hazards (`docs/CONVENTIONS.md` Database Migrations: `034_drop_dead_settings.sql` follows the `NNN_` naming, is wrapped in `BEGIN; … COMMIT;`, uses `IF EXISTS` so a re-run after a crash is safe, follows the `028_drop_theme_default.sql` pattern, and both GORM fields are gone; the `default:` tag hazard does not apply to dropped fields; library_id scoping is untouched), mechanical (shape and coverage: `serverSettingsView`, `librarySettingsView`, both patch requests, `loadOrCreateServerSettings`, `library.Settings` defaults and the TS `ServerSettings`, `LibrarySettings`, `LibrarySettingsPatch`, `SettingsView` and `usePatchServerSettings` input all lose the fields; cross-boundary: `TARGET_DISTRIBUTIONS` values match `allowedDistributions` at `internal/handlers/api/projects.go:122`, kept as a mirrored list like `lib/stores.ts` because Go and TS cannot share a constant here; completeness and shared-library checks do not apply), logic (the label falls back to the raw value; every project created with no distribution stores `unspecified`, so every card shows "Unspecified", which is the old behaviour with a capital letter), error handling, silent failure (old clients sending dropped fields get them ignored by JSON binding rather than rejected, which is right for removed optional fields), input validation (unchanged server-side), idempotency (migration), security (admin and library settings routes keep their guards; no auth change), resource management (the removed boot goroutine took a context that is still cancelled on shutdown; `BackfillCovers` is unchanged), logging (the "gallery repair" log lines went with the function; nothing else emits them), performance (boot no longer lists `thumbs/`), change-type blocks: db-migration (one-way: README.md:66 says "there is no downgrade path", so rolling back the binary after 034 is unsupported, as stated; `DROP COLUMN` takes a brief ACCESS EXCLUSIVE lock on two small tables; no backfill; committed with the GORM change and a test), web-endpoint (`GET`/`PATCH /api/settings` and `/api/admin/settings` change only in shape), risk and omissions (cross-stage leftover sweep with `git grep` over the whole repo, excluding the lockfile and specs, covering Go, `cmd/dmsn`, `tools/`, `web/` src/e2e/stories, docs, README, AGENTS.md and the migrations: no `dedupe_enabled`/`DedupeEnabled`, `licence_kb`/`LicenceKB`, "knowledge base" UI copy, `RepairGallery`/"gallery repair", `BrowseRedirect`, `/processing` route, `/library/:id/browse` route, `@radix-ui/react-slider`, `Slider` primitive (only lucide `Sliders`/`SlidersHorizontal` icons), `assetLicence`, `useCreateLibrary` or `engine=` prop remain outside migration 005; dmsn writes no skill or template that names these; `docs/architecture.md` never described the repair; `docs/e2e-flows.md` matches the retired assertions and the new projects assertion does not make it untrue; the `anyAsset` comment is updated; compliance copy is gone from ProjectsPage and the create dialog, and the only remaining "licence compliance" text is the `LicenceBar` doc comment, which describes the bar, not a promise of checks; across stages, the ProjectCard rename reaches its only two callers, `ProjectsPage` and `Project.stories`, and the stage-1 settings removal and stage-5 dependency removal do not touch each other), backward compatibility (audit rows keep the old keys as history; nothing renders them by name), deployment (forward-only migration runs at startup), dependencies (one removal; lockfile change is the `pnpm remove` output), tests (preflight audit test moved to `retain_original_packages`; unit and e2e cover the readable label and that a chosen distribution is stored; the removals have no remaining behaviour to test), structural risk (the `targetDistribution.ts` placement mirrors `lib/stores.ts`), smell baseline, operator intent (no `docs/intent/`; the one UI change is a tweak covered by the steering quoted in the spec) · issues in: risk and omissions (docs co-change) · n/a: concurrency (no shared state), external contracts (no new calls), credentials (none touched) · overlap with `### Review — stages 1…5`: the leftover sweep and the shape checks repeat here across the whole repo and still find nothing · human approver: schema change (two columns dropped, one-way), public API shape change (two response fields removed) · validation: none needed, 0 candidates, 0 confirmed, 0 refuted

### Results

| Item | Status | Notes |
| --- | --- | --- |
| No Dedupe toggle/row; no licence-KB auto-update in Settings › Server | passed | Playwright against `just dev`: 0 matches for /Dedupe/ and /knowledge base/ on /settings; Server settings section reads only "Max upload bytes". No test added: a deletion has no behaviour left to protect. |
| `GET /api/settings` / `GET /api/admin/settings` lack the fields | passed | curl against `just dev`: neither key present. Preflight audit test now flips `retain_original_packages`. |
| Existing DB loses both columns; fresh DB never has them | passed | Booted `main`'s build against the empty dev DB (both columns created), then the branch: `information_schema` count 0. `damson_test` after the e2e run: 0. |
| `configuration.md`, `separate-libraries.md` no longer mention them | passed | rows and the "dedupe" mention removed; `docs/images/settings-page.png` retaken (light, 1440×900, dev admin so avatar reads "DE"). |
| No licence-compliance copy; project page shows a readable label | passed | Playwright: dialog and projects page have 0 matches for /compliance\|compatib/; header shows "Commercial". Unit `ProjectDetailPage header` fails when the label is replaced by the raw value (confirmed). |
| Creating a project with a target distribution stores and shows it | passed | e2e `projects.spec.ts` now picks "Open source" and asserts it in the header after create, against the real API and Postgres; fails if the value is dropped or shown raw. Also curl POST returned `commercial`. |
| Startup no longer runs the gallery repair | passed | `RepairGallery` and its boot call removed; `just build backend` succeeds. No test: removal. `RepairGallery` was silent when nothing was missing, so its absence is not visible in boot logs. |
| `/processing` and `/library/<id>/browse` no longer redirect; e2e-flows updated | passed | Playwright: both URLs stay put. Two e2e assertions retired (triage, package page). |
| `Slider`, `@radix-ui/react-slider`, `queryKeys.assetLicence`, `useCreateLibrary` gone | passed | grep over `web/` finds none; `pnpm remove` updated the lockfile. |
| `just check` | passed | second run exit 0 (Vitest 570, story tests 155, e2e 37, Go 24 packages ok). First run failed on Prettier in `App.tsx` and `ProjectsPage.tsx`; fixed with `just format`. |
| review: stages 1…5 | passed | `### Review — stages 1…5`, no blockers. |


### Cleanup

- Acceptance: all 10 lines met (evidence in Results above). No undeclared departures.
- Review coverage: `### Review — stages 1…5` covers every built stage; `### Review — cleanup` ran the thorough pass over the branch. Its one LOW: the how-to text is fixed, and the operator accepted the stale `docs/images/project-page.png`, which still shows the old raw mono `commercial` badge.
- Simplify: `ProjectsPage.tsx` passes `targetDistributionLabel(...)` straight through (an empty value still renders nothing), and the migration comment loses its note on the rejected alternative.
- Test gate: no cuts. The `ProjectDetailPage header` unit test guards the value→label rule. The `projects.spec.ts` assertion guards the stored value's round trip through the API and Postgres. The preflight and ServerAdminSection changes swap fixtures only.
- Verify: `just check` exit 0 after the last edit.
- Open at retirement: none.
