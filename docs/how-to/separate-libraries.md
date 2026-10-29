# Keep two teams' assets apart

A **library** is Damson's wall. Assets, tags, projects, collections, settings
and the audit log each belong to exactly one, and you only ever see a library
you are a member of. One Damson instance can host a studio's shared shelf, a
client's licensed material and somebody's scratch pile without any of them
knowing about each other.

Reach for this when two teams must not see each other's packs, when a client's
assets come with terms that say so, or when you want a sandbox that does not
pollute the catalogue everyone else searches.

Most small teams never need a second library. One is the right answer until
something forces otherwise — the separation is real, and so is the friction.

## What the wall actually separates

Everything library-scoped, which is nearly everything:

- assets, and through them versions, files, imports, images and licences;
- tags, projects, collections;
- the library's settings row and its audit log;
- in-flight upload sessions.

What is *not* separated, because it is global to the instance:

- **users** — one person, one account, however many libraries they belong to;
- **stored objects** — one content-addressed object store, shared. Two
  libraries that import the same texture store it once. Nothing leaks, because
  access always goes through a library-scoped row, but the bytes are shared and
  the disk accounting is instance-wide;
- **server settings** — the upload ceiling and storage cleanup are the
  sysadmin's, not a library's.

## Two role hierarchies, kept apart

| | Values | Scope |
|---|---|---|
| `library_memberships.role` | `admin`, `member` | One library |
| `users.role` | `user`, `sysadmin` | The whole instance |

They are independent. A sysadmin is not automatically a member of anything.

A **library admin** can edit library settings, rename and delete the library,
and invite, re-role and remove its members. A **member** can do everything
else — upload, tag, pin, curate, and read the settings screen without being
able to change it.

A **sysadmin** can administer any library — invite, re-role, rename, delete —
without being a member of it, and gets the **Server settings**, **Users**,
**Storage cleanup** and server-wide audit screens.

But a sysadmin **cannot read a library's contents without joining it.** The
library-scoped routes resolve your current library through your own
membership, with no bypass. To look at a library's assets, a sysadmin has to be
a member of it like anyone else.

## Create one

**The web UI cannot create a library.** It can rename, delete and switch
between them. Creating one is an API call that any authenticated user may make:

```sh
TOKEN=$(curl -sS -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"your-password"}' \
  | jq -r .token)

curl -sS -X POST localhost:8080/api/libraries \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Hollow Client Work","description":"Licensed material, do not reuse"}'
# {"id":2,"name":"Hollow Client Work","description":"Licensed material, do not reuse"}
```

Whoever creates it becomes its admin. **It does not become your current
library** — switching is a separate, deliberate step.

Note that there is no instance-wide gate on this: any authenticated user can
create more, including one who is not a member of a single library yet. If that
matters to you, it is a thing to watch in the audit log (`library.create`), not
a thing to configure.

## Switch between them

The header carries a **Switch library** button listing every library you belong
to, with your role in each. Picking one changes your current library for every
screen and every subsequent API call.

> _[Screenshot: the Switch library dropdown open, showing two libraries with
> their role badges and the current one marked.]_

Over the API:

```sh
curl -sS -X POST localhost:8080/api/me/current-library \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"library_id":2}'
```

Switching to a library you are not a member of is a 403.

Your current library is stored on your user row, not in the browser, so it
follows you between devices and between sessions.

**A personal access token is different: it is pinned to the library that was
current when you minted it, permanently.** Switching libraries in the SPA does
not move a token. Automating against two libraries means two tokens. Mint each
one while the right library is selected, and label it accordingly.

## Add people

**Settings → Members**, then **Invite**. You need to be a library admin (or a
sysadmin).

The dialog takes an email, an optional display name, a role, and — in local
auth mode — an **Initial password**.

> _[Screenshot: the Invite a member dialog with email, display name, role and
> initial password fields.]_

What happens depends on whether the person already has an account:

- **They have one.** Leave the password blank; they are added to this library
  with the role you chose.
- **They do not, and you are in local auth mode.** Supply a password and
  Damson creates the account and the membership together. The password must be
  at least 8 characters.
- **They do not, and you are in OIDC mode.** This fails with "user not found —
  they must sign in once before being invited". Identity lives in your IdP, so
  there is no admin-driven account creation path. Have them sign in first;
  they will land on a "No library access yet" screen, and then you can invite
  them.

Inviting someone who is already a member is a 409.

### Change a role, or remove someone

Both are on the same **Members** list. Damson refuses to demote or remove the
**last admin** of a library — a library cannot end up with nobody who can
administer it. The UI disables those controls with a tooltip saying why; the
API answers with a 409.

Removing a member does three things, all at once:

1. deletes the membership;
2. invalidates every token and session that user holds, instance-wide, so an
   open tab cannot keep reading from cache;
3. clears their current library if it was this one.

Their next request fails, the SPA bounces them to sign in, and they land
wherever their remaining memberships put them. Note the second point: removing
someone from one library signs them out of all of them. It is abrupt on
purpose.

## Settings are per library

**Settings** configures the library you are currently in, and says so under the
heading. A library admin gets the form; a member gets the same values
read-only, with "You have read-only access — ask a library admin to edit."

The sections are **Appearance** (grid density default), **Storage** (retain
original packages, dedupe), **Thumbnails** (raster, audio waveform, font
sample, Unity preview harvest), **Metadata** (auto-enrich on import, preferred
store) and **Audit** (retention in days).

> _[Screenshot: the Settings page for a library, showing the Appearance,
> Storage and Thumbnails sections with their toggles.]_

Every change is audited. The full table of what each setting does, and the one
per-library setting with no control surface, is in
[`docs/configuration.md`](../configuration.md).

Two of these are worth setting deliberately per library rather than leaving at
the default:

- **Retain original packages.** A client library that must be reproducible
  wants this on; a scratch library full of 8 GiB packs probably does not. It
  is read at import time only — see
  [Import Unity packages](import-unity-packages.md).
- **Auto-enrich on import.** A library of material that is not on any
  storefront gains nothing from the lookups, and they cost time and outbound
  requests.

## Rename or delete one

**Settings → Library profile** renames it. The same section's **Danger zone**
deletes it, behind a dialog that makes you type the library's name
(case-sensitive) to confirm.

Deleting is total and immediate. Every asset, version, file row, tag, project,
collection and audit entry scoped to that library goes with it, through the
database's cascades. Every member's sessions are invalidated, so nobody is
left holding a dead library id. Anyone whose current library it was gets it
cleared.

**The stored objects are not deleted.** The rows that referenced them are gone,
which makes those objects orphans, and a sysadmin reclaims them deliberately
through **Settings → Storage cleanup**. That is the design: nothing on a normal
path ever deletes bytes.

The deletion is recorded in the *server-wide* audit feed rather than the
library's own — the library's log went with the library — and carries the name
and the counts of memberships and assets that went with it.

There is no export, no archive and no undo. If you might want the contents
again, back up Postgres and the object store first; see the Backups note in
[`docs/getting-started.md`](../getting-started.md).

## Things that will catch you out

- **A 403 is about you, a 404 is about the resource.** If your current library
  is gone — deleted, or your membership removed — every scoped route answers
  403 before it looks at what you asked for. Asking for an asset that belongs
  to a library you are not in is a plain 404 instead: scoped queries match on
  the id *and* the current library, so the row simply is not there. Neither
  response tells you whether something exists elsewhere.
- **A user with no membership can sign in and then has nowhere to go.** They
  see "No library access yet" with their own email on it. There is no
  "everyone can see everything" mode and no default library.
- **Nothing moves between libraries.** There is no transfer, no copy and no
  cross-library search. Re-import, or record the pack as a reference asset in
  the second library.
- **The object store is shared.** A library is an access boundary, not a
  storage boundary. If you need genuinely separate storage, you need separate
  Damson instances.
- **Audit is per library** (**Audit log** in the rail), plus a server-wide feed
  for sysadmins. Retention is a per-library setting, unset — keep forever — by
  default.

## Where to go next

- [`docs/configuration.md`](../configuration.md) — every per-library and
  server-wide setting, and the OIDC setup.
- [`docs/observability.md`](../observability.md) — the log shape, if you want
  to see library scoping from the outside.
- [`docs/architecture.md`](../architecture.md), Authentication and
  authorisation — the rules behind all of the above.
