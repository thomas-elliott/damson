# Known gaps and divergences

## Known gaps

**Decisions to not build something, with the reason.** This is the rejection log, so the same idea
does not get re-proposed, re-analysed and re-rejected — the analysis is the expensive step. Work
that is merely *not done yet* lives in [`roadmap.md`](roadmap.md), not here.

- **No `useInfiniteQuery`** — Lists are `limit`/`offset` server-side and the grid virtualises a single page. Add paging when a list actually needs it, not before.
- **No e2e for chunked uploads** — Triggering it needs a 256 MiB file; generating one per run blows the suite's runtime budget. Covered by unit and handler tests instead — see [`e2e-flows.md`](e2e-flows.md).
- **Chunk size is a Go constant** — Advertised per session so it can change without a client release, but changing it still needs a server release. `server_settings` is the natural home *if* it ever needs runtime tuning.
- **Terminal upload sessions are never purged** — `completed` / `aborted` / `expired` rows accumulate. Small, and useful as history; add retention only if it ever matters.
- **No CLI distribution** — `just build cli` only — no cross-compile matrix, no release artefacts. `dmsn setup` does the discovery work with no install story behind it. Deliberate; revisit if the CLI gets external users.
- **Heavyweight preview formats** — `.blend` rendering, Unity scene/prefab visual reconstruction, AIFF transcoding and rtf/doc conversion each need an external binary (Blender, ffmpeg, LibreOffice). Download-only is the documented default. Re-open only if a library turns out to be dominated by one of them.

---

## Decisions in code that diverge from the archived draft docs

- **Local auth mode exists alongside OIDC.** The archived tech and product
  docs describe OIDC only. `DAMSON_AUTH_MODE=local` is a real, tested path;
  the two are mutually exclusive at startup.
- **External identities live in `user_identities`, not `users.subject`.**
  Migration 008 dropped `users.subject`. Identities are keyed
  `(issuer, subject)` and a verified email is the cross-IdP join key, so one
  user can sign in through several IdPs. No data backfill — an existing OIDC
  user is re-adopted by email on next login.
- **A version's identity is its `store_version`,** not a sequence number.
  Re-extraction rewrites in place instead of spawning a version.
- **`importResponse` reports `file_count` + `package_kind`,** not a
  single-file `file_id`. The M1 shape assumed one file per import, which M2
  made wrong. Read the handler, not the draft §16.
- **Background work is a Postgres-backed queue,** not in-process goroutines.
  The DB stays the source of truth for *state*; River tracks *work*.
