# Trialling `@telliott/shared-ui` as a package

damson is the trial consumer for `shared-design`'s **npm package** path, which supersedes copy-in as
the default distribution. Run it on a branch against the prerelease channel; if it holds,
`shared-design` cuts its first release and damson adopts the version.

`shared-design/docs/consuming.md` is the install reference. This file is only what is
damson-specific, and what the trial exists to find out.

## Why a prerelease and not a release

`shared-design` publishes `0.0.0-main.<sha>` under the **`main`** dist-tag on every push to main.
That is a prerelease below every released version, so no `^` or `~` range reaches it and nothing
adopts it by accident.

```bash
pnpm add @telliott/shared-ui@main   # or pin the exact 0.0.0-main.<sha>
```

Pin the exact version once the branch is working, so the dependency stops moving under you.

**Nothing has been released yet** `[2026-08-21]`. The registry holds only prereleases, so the
package's contract — the `exports` shape, what the CSS entry carries, whose `cn` runs inside a
component — is still cheap to change. Anything this trial finds gets fixed in `shared-design` before
the first tag rather than in a `0.1.x` afterwards. That is the whole reason the trial comes first.

## What is already proven, and where

Verified 2026-08-21 against the published `0.0.0-main.ccc99a8`, in a synthetic consumer with no
pre-existing palette:

- resolution from the Forgejo registry, anonymously — the packument is world-readable, so a token is
  needed to publish and not to install
- `exports`, and declarations under **both** `moduleResolution: bundler` and `node16`/`nodenext`
- `dist/styles.css` producing real utilities, and the component classes actually reaching the
  emitted CSS
- the `@source` gate, including its negative control: drop the line and `bg-destructive` vanishes
  from the CSS while the build still exits 0
- SSR render of `Button` and `Chip`, and `cn` merging the role scale

None of that involved a competing palette, a competing `cn`, pnpm, a token, or Docker. **Those are
what damson is for.**

## One-time setup

**1. `web/.npmrc`**, committed:

```
@telliott:registry=https://git.smol.cloud/api/packages/telliott/npm/
//git.smol.cloud/api/packages/telliott/npm/:_authToken=${FORGEJO_TOKEN}
```

The auth line omits the `https:` scheme deliberately — Forgejo's docs call that out. Without the
scope mapping the package resolves against npmjs.com, where `@telliott` is unclaimed: a 404 today
and someone else's code the day anyone registers it.

**2. `Dockerfile:5` will break, by inspection.** It reads:

```dockerfile
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
```

The `.npmrc` is not in that COPY, and `COPY web/ ./` does not land until line 7 — after the install.
Add it to line 5, and give the build stage `FORGEJO_TOKEN`. This is the failure
`shared-design/docs/consuming.md` predicts for the layer-cache idiom; damson is the case that
confirms it.

**3. `pnpm-workspace.yaml`** — `web/pnpm-workspace.yaml` exists; add:

```yaml
minimumReleaseAgeExclude:
  - "@telliott/shared-ui"
```

pnpm 11 quarantines new versions for 1440 minutes by default. A prerelease you are actively testing
is minutes old, so without this it is uninstallable for a day. **damson is the first repo in the
estate that will exercise the cooldown.** Report whether the exclusion is sufficient on its own.

**4. Keep `components.json`'s `@shared` registry entry.** The copy-in path stays available for a
fork or a one-off; it is no longer the default.

## The three unknowns this trial closes

Each has a concrete predicted failure. Confirm or refute each one explicitly — a refutation is worth
as much as a confirmation and neither is currently known.

### 1. Two `cn`, and a silently dropped font size

**The sharpest of the three, and the one most likely to need a change in `shared-design`.**

`web/src/design-system/utils/cn.ts` composes `typeScale`'s 8 contract roles with **19 damson roles**
(`text-mono`, `text-caption`, `text-num`, `text-table-row`, the `title-*`/`heading-*`/`subhead-*`
families). A package component does **not** use that `cn` — it uses the one bundled in
`dist/lib/utils.js`, which extends `typeScale` alone.

So predict: `<Button className="text-mono">` loses `text-mono`. The package's `cn` does not recognise
it as a font size, tailwind-merge falls through to the colour group, and the button's own
`text-label` wins. No error, no warning — the class is simply gone from the DOM.

This is the same failure mode `cn.ts`'s own comment describes for the contract roles, one layer out.
Test it directly: render a package `Button` with each of the 19 roles and read the resulting
`class` attribute.

If it reproduces, the options are damson's to raise and `shared-design`'s to decide — teach the
package's `cn` to accept an injected config, have package components accept a merged `className`
resolved by the caller, or rule that damson roles never cross the boundary. **Do not fix it by
widening damson's `cn`**; it is not the one running.

### 2. Two `tokens.css` in one stylesheet

`web/src/styles/globals.css:8` imports damson's own `tokens.css` as `layer(base)`, and damson
overrides every contract value.

`@telliott/shared-ui/styles.css` carries `tokens.css` + `theme.css` + `motion.css`, and both token
files must stay **unlayered** to beat Tailwind's own `@layer theme` copy of the same names. Import it
alongside damson's layered one and the precedence question is live: an unlayered declaration beats a
layered one, so the package's *reference palette* can win over damson's overrides.

Also worth settling: whether damson needs the package's CSS entry at all. `globals.css:23` already
carries a CLI-generated `@theme inline` block mapping the same names, written by
`shadcn add @shared/tokens`. If that block is complete, damson may need `motion.css` and nothing
else — but `theme.css` is **not independently importable** (31 of its 71 entries are self-references
whose targets `tokens.css` declares), so "import only the half I need" is not available as stated.

Expect this to end in a recommendation back to `shared-design` about what the package should export.

### 3. Authenticated install, in CI and in the image

Item 2 of the setup above is a prediction from reading `Dockerfile:5`, not an observation. Confirm it
fails, then confirm the fix works in the real image build — not just locally, where a developer's
ambient token can mask a missing one.

## What to report back

Into `shared-design/docs/roadmap.md`'s evidence bank, one line each, and anything needing a code
change before its first tag flagged as blocking:

- each of the three unknowns: reproduced, or refuted
- whether `minimumReleaseAgeExclude` alone is enough on pnpm 11
- whether any of the four currently-installed copy-in files
  (`web/src/components/ui/{button,dialog,empty-state,icon-button}.tsx`) behave differently as package
  imports
- anything `consuming.md` claims that turned out not to be true — it has never been followed by a
  real repo

## Docs this will make wrong

- `docs/design-system.md` — wherever it says primitives arrive by `shadcn add`.
- `docs/shared-ui-package-trial.md` — this file. It is a trial brief; once the outcome is known it
  should be replaced by whatever the adoption actually was, or deleted.
- `docs/shared-design-adoption.md` is already marked superseded and describes the copy-in path. Leave
  it alone.
