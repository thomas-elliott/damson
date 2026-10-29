# Keep two teams' assets apart

A library is Damson's access boundary. Assets, tags, projects, collections, settings and the audit log each belong to exactly one library, and you see only libraries you are a member of. One instance can host a studio's shared assets, a client's licensed material and a scratch pile without any seeing the others.

Use separate libraries when two teams must not see each other's packs, when a client's terms require it, or when you want a sandbox that does not pollute the shared catalogue. Most small teams need only one library. The separation is real, and so is the extra work.

## What a library separates

Everything library-scoped, which is nearly everything:

- assets, and through them versions, files, imports, images and licences;
- tags, projects and collections;
- the library's settings row and audit log;
- in-flight upload sessions.

Global to the instance, and so not separated:

- Users: one account, however many libraries the person belongs to.
- Stored objects: one shared, content-addressed object store. Two libraries importing the same texture store it once. Access always goes through a library-scoped row, so nothing leaks, but the bytes are shared and disk accounting is instance-wide.
- Server settings: the upload ceiling and storage cleanup belong to the sysadmin.

## Two role hierarchies

| | Values | Scope |
|---|---|---|
| `library_memberships.role` | `admin`, `member` | One library |
| `users.role` | `user`, `sysadmin` | The whole instance |

They are independent. A sysadmin is not automatically a member of anything.

A library admin can edit library settings, rename and delete the library, and invite, re-role and remove members. A member can do everything else (upload, tag, pin, curate) and can read the settings screen without changing it.

A sysadmin can administer any library (invite, re-role, rename, delete) without being a member, and gets the **Server settings**, **Users**, **Storage cleanup** and server-wide audit screens.

A sysadmin cannot read a library's contents without joining it. Library-scoped routes resolve your current library through your own membership, with no bypass.

## Create one

The web UI can rename, delete and switch libraries but cannot create one. Creating one is an API call that any authenticated user may make:

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

The creator becomes its admin. It does not become your current library; switching is a separate step.

No instance-wide setting restricts creation, so any authenticated user can create libraries, including one who belongs to none yet. To watch for this, check the audit log for `library.create`.

## Switch between them

The **Switch library** button in the header lists every library you belong to, with your role in each. Picking one changes your current library for every screen and API call.

> _[Screenshot: the Switch library dropdown open, showing two libraries with their role badges and the current one marked.]_

Over the API:

```sh
curl -sS -X POST localhost:8080/api/me/current-library \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"library_id":2}'
```

Switching to a library you are not a member of returns 403.

Your current library is stored on your user row, not in the browser, so it follows you between devices and sessions.

A personal access token is pinned permanently to the library that was current when you minted it. Switching libraries in the SPA does not move a token. To automate against two libraries, mint two tokens, each while the right library is selected, and label them.

## Add people

Go to **Settings → Members**, then **Invite**. You must be a library admin or a sysadmin. The dialog takes an email, an optional display name, a role and, in local auth mode, an **Initial password**.

> _[Screenshot: the Invite a member dialog with email, display name, role and initial password fields.]_

The result depends on whether the person has an account:

- They have one: leave the password blank. They are added to this library with the chosen role.
- They have none, in local auth mode: supply a password of at least 8 characters, and Damson creates the account and membership together.
- They have none, in OIDC mode: the invite fails with "user not found — they must sign in once before being invited". Your IdP holds identities, so Damson cannot create accounts. The person signs in first, sees "No library access yet", and you then invite them.

Inviting an existing member returns 409.

### Change a role, or remove someone

Both are on the **Members** list. Damson refuses to demote or remove a library's last admin. The UI disables those controls with an explanatory tooltip, and the API returns 409.

Removing a member:

1. deletes the membership;
2. invalidates every token and session that user holds, instance-wide, so an open tab cannot keep reading from cache;
3. clears their current library if it was this one.

Their next request fails and the SPA sends them to sign in. Removing someone from one library therefore signs them out of all of them.

## Settings are per library

**Settings** configures your current library and names it under the heading. A library admin gets the form. A member sees the same values read-only, with "You have read-only access — ask a library admin to edit."

The sections are **Appearance** (grid density default), **Storage** (retain original packages, dedupe), **Thumbnails** (raster, audio waveform, font sample, Unity preview harvest), **Metadata** (auto-enrich on import, preferred store) and **Audit** (retention in days).

> _[Screenshot: the Settings page for a library, showing the Appearance, Storage and Thumbnails sections with their toggles.]_

Every change is audited. [`docs/configuration.md`](../configuration.md) has the full table of settings and the one per-library setting with no control surface.

Two settings are worth choosing per library:

- Retain original packages. A client library that must be reproducible wants it on; a scratch library of 8 GiB packs probably does not. It is read at import time only (see [Import Unity packages](import-unity-packages.md)).
- Auto-enrich on import. A library of material that is on no storefront gains nothing from lookups, and they cost time and outbound requests.

## Rename or delete one

**Settings → Library profile** renames it. The **Danger zone** in the same section deletes it, after you type the library's name (case-sensitive).

Deletion is total and immediate. Every asset, version, file row, tag, project, collection and audit entry in the library goes with it through database cascades. Every member's sessions are invalidated, and anyone whose current library it was has it cleared.

The stored objects are not deleted. Their referencing rows are gone, so they become orphans, and a sysadmin reclaims them deliberately through **Settings → Storage cleanup**. Nothing on a normal path deletes bytes.

The deletion is recorded in the server-wide audit feed, not the library's own, because the library's log went with it. The entry carries the library's name and the counts of memberships and assets removed.

There is no export, archive or undo. If you might want the contents again, back up Postgres and the object store first; see the Backups note in [`docs/getting-started.md`](../getting-started.md).

## Things that will catch you out

- A 403 is about you and a 404 is about the resource. If your current library is gone (deleted, or your membership removed), every scoped route returns 403 before looking at the request. Asking for an asset in a library you are not in returns 404, because scoped queries match the id and the current library. Neither response reveals whether something exists elsewhere.
- A user with no membership can sign in and has nowhere to go. They see "No library access yet" with their email. There is no "everyone can see everything" mode and no default library.
- Nothing moves between libraries. There is no transfer, copy or cross-library search. Re-import, or record the pack as a reference asset in the second library.
- The object store is shared, so a library is an access boundary, not a storage boundary. For separate storage, run separate Damson instances.
- Audit is per library (**Audit log** in the rail), plus a server-wide feed for sysadmins. Retention is a per-library setting; unset keeps entries forever.

## Where to go next

- [`docs/configuration.md`](../configuration.md): every per-library and server-wide setting, and the OIDC setup.
- [`docs/observability.md`](../observability.md): the log shape, to see library scoping from outside.
- [`docs/architecture.md`](../architecture.md), Authentication and authorisation: the rules behind all of this.
