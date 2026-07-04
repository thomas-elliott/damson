# Notification Centre (upload queue → persistent notifications)

## Problem

Three issues surfaced while dogfooding uploads:

1. **Uploads die on navigation.** Upload state lives in `DropZone`
   (`web/src/components/DropZone.tsx`), which is mounted inside `LibraryPage`.
   React Router renders one page element per route, so leaving the library
   unmounts `DropZone` and aborts the in-flight XHR — the upload silently dies.
2. **The upload popup can't be hidden or cleared.** The bottom-right upload
   list has no close, dismiss, or clear affordance. Finished rows just pile up
   until the page unmounts.
3. **There's no home for toasts/errors.** Damson has no global toast surface
   (confirmed: no toast/notification system exists client- or server-side).
   Mutation errors surface inline or not at all.

We want a single **Notification Centre**: a bell in the nav rail above
Settings (with an unread badge) that opens a panel pinned to the bottom-right
of the app and survives every route change. Uploads run there independently of
which page is mounted; finished uploads and app toasts become notifications
that can be read, dismissed, and archived.

## Chosen approach

**A global `NotificationProvider` mounted above the routed area.** It owns
uploads, notifications, and the panel's open/minimised state, and renders the
fixed bottom-right panel itself — so nothing in the tree unmounts on
navigation. `DropZone` remains the drop target but hands files to the provider
via `startUpload(files)` instead of running the upload locally. The rail bell
and the panel both read/call context.

This beat the alternatives:

- **Layout-route refactor** (persistent `<Outlet/>` layout, rail pulled out of
  every page) — "most correct" long-term but touches every page's `Shell`
  usage; far larger blast radius than this fix warrants. Damson's established
  pattern is per-page `<Shell>`, which we keep.
- **React Query as the store** — uploads are imperative client state (XHR
  handles, abort); forcing them through the query cache is awkward and overkill.

### Behaviour model

- **Uploads** — live browser→server transfers with a progress bar. They
  **cannot be cleared, only cancelled** (aborts the XHR). Cancelling **removes
  the upload silently** — no notification left behind. On success → a `done`
  (success) notification; on failure → an `error` notification. Uploads are
  **in-memory only**; a page reload ends any in-flight upload (XHR can't be
  resumed).
- **Notifications** — finished uploads *and* all app toasts/errors land here.
  Each can be **marked read** (moves to the Read tab, not deleted) or
  **dismissed** (deleted immediately). Persisted to `localStorage` so they
  survive reloads; **read items older than 7 days are pruned on load**.
- **Read tab** — the archive of read notifications; auto-clears after 7 days.
- **Panel** — pinned **fixed bottom-right** of the app. **Minimise** (an
  icon-only control, no "minimise" label) closes it; clicking the rail bell
  toggles it open again.
- **Uploads tab is hidden entirely when nothing is in flight** — it only
  appears while transfers are running. The panel otherwise shows
  Notifications + Read.
- **Rail badge** counts **unread notifications + in-flight uploads**, so active
  transfers are visible from any page.

## Task breakdown

Ordered stages, each an independently-committable unit.

1. **Add cancellation to `uploadFile`.** Accept an `AbortSignal` (or return an
   abort handle) and wire `xhr.abort()`. Aborted uploads must reject/settle in
   a way the caller can distinguish from a real failure (so a cancel produces
   no error notification). Isolated; no UI change.
   - `web/src/api.ts`

2. **`NotificationProvider` + context.** State model — uploads (`id`,
   `filename`, `size`, `progress`, `status`, `error?`, abort handle) and
   notifications (`id`, `tone`, `title`, `message?`, `createdAt`, `read`,
   `readAt?`, `source`). `localStorage` persistence for notifications with a
   7-day prune of read items on load; uploads are never persisted. Context API:
   `startUpload(files)`, `notify(...)`, `markRead(id)`, `dismiss(id)`,
   `clearRead()`, `toggleOpen()` / minimise, plus derived `unreadCount` and
   `inFlightCount`. `startUpload` runs the XHR uploads (progress tracking,
   query invalidation for `assets` + `imports` as `DropZone` does today), and
   on settle converts each upload into the appropriate notification. Mount the
   provider around the authed routes in `App.tsx`.
   - `web/src/components/notifications/NotificationProvider.tsx`, `web/src/App.tsx`

3. **`NotificationCentre` panel UI.** Fixed bottom-right panel rendered by the
   provider. Tabs: Uploads (only when `inFlightCount > 0`), Notifications, Read
   — using the `Tabs` primitive. Upload rows reuse `QueueItem` / `ProgressBar`
   with a Cancel action; notification rows show tone + Mark read / Dismiss;
   Read tab shows archived items with a Clear-all affordance. Icon-only
   minimise button in the header.
   - `web/src/components/notifications/NotificationCentre.tsx`

4. **Rail bell + badge.** Add a bell `NavRailItem` above Settings in
   `DamsonRail`; badge from context (`unreadCount + inFlightCount`); click
   toggles the panel.
   - `web/src/components/Shell.tsx`

5. **Rewire `DropZone` + remove the old popup.** `DropZone.onDrop` and the
   library Upload button call `startUpload`; delete the local `uploads` state
   and the bottom-right list in `DropZone`. Keep `DropZone` as the drop
   surface + picker (`useDropZoneOpen`). Verify uploads keep running across a
   route change.
   - `web/src/components/DropZone.tsx`, `web/src/pages/LibraryPage.tsx`

6. **Route mutation errors into `notify()`.** Wire mutation/query error
   handling so failures push error notifications, delivering the "all toasts
   show here" goal.
   - `web/src/hooks/useMutations.ts`, `web/src/lib/queryClient.ts`

## Reuse

- **Design-system primitives (all barrel-exported):** `Tabs` / `TabsList` /
  `TabsTrigger` / `TabsContent` / `TabCount`, `Badge`, `ProgressBar`,
  `QueueItem`, `IconButton`, `cn`, `formatBytes`, plus `NavRailItem`'s existing
  `badge` prop and lucide icons (`Bell`, `X`, etc.).
- **Existing upload logic** in `DropZone.tsx` (XHR progress + query
  invalidation for `queryKeys.assets()` and `queryKeys.imports`) — moved into
  the provider, not rewritten.
- **`localStorage` persistence pattern** already used in `AppShell.tsx`
  (sidebar width) and `lib/theme.ts`.
- **Reuse search that came up empty:** grep across `web/src` and
  `internal/` found **no existing toast/notification system** (client or
  server). The `useSavedFlash` hook is a similar transient-indicator pattern
  but is inline/per-component, not a global surface — the centre is built new.
- The import queue (`/api/imports`, SSE, `/queue` page, its rail badge) is a
  **separate** server-side pipeline concept and is left untouched; this spec
  concerns the client-side upload transfer + notifications only.

## Resolved questions

- **Panel position:** fixed bottom-right of the app (matches today's popup;
  clear of the left rail and detail panels).
- **Cancelled upload:** removed silently — no notification.
- **Rail badge:** counts unread notifications **+ in-flight uploads**.
- **Stage 6 (toasts/errors → notifications):** in scope now — build the
  `notify()` API and route mutation errors into the centre.
- **Minimise control:** icon only, no "minimise" text label.
- **Uploads tab:** hidden entirely when no upload is in flight.
