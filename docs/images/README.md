# Documentation screenshots

The images in this directory are used by the README, `docs/getting-started.md` and the guides in `docs/how-to/`. They show Damson 0.3.0 with sample data, not a real studio's assets.

## How they were taken

- A real instance: the built server and SPA, Postgres, local storage, signed in as a local-mode admin.
- 1440×900 viewport, device scale factor 1, light theme, headless Chrome with no browser chrome in the frame. The one exception is `dmsn-sync-scan.png`, which is real `dmsn sync scan` output set in a terminal box.
- Auto-enrich was off while importing. With it on, a pack whose name resembles a storefront listing picks up that listing's publisher, description and licence, and can be renamed.
- Sample packs were generated, not downloaded: stylised cover art, textures, fake meshes, short WAV loops, and licence files holding real licence text (Unity Asset Store EULA, MIT, Apache-2.0, CC0-1.0, GPL-3.0). Nothing in them comes from a third party's product.
- Pack versions carry a version label equal to their store version, because the project page shows labels, not store versions. See [Pin a project to a version you have tested](../how-to/pin-versions.md).

## Keeping them current

A screenshot goes stale when the UI around it changes. When you change a screen shown here, retake its image at the same size and theme, with the same sample data, so the set stays consistent. Each image is named for the screen it shows.

| Image | Used by |
|---|---|
| `first-run-bootstrap.png` | getting-started |
| `library-switcher.png` | getting-started, separate-libraries |
| `library-drop-overlay.png`, `import-queue.png` | getting-started |
| `library-overview.png`, `asset-detail-panel.png` | README |
| `settings-page.png`, `settings-invite-member.png` | separate-libraries |
| `dmsn-sync-scan.png`, `browse-files.png` | import-unity-packages |
| `asset-versions-panel.png`, `notification-new-version.png`, `library-filter-updates.png`, `project-pin-dialog.png` | pin-versions |
| `project-page.png` | pin-versions, track-licences |
| `asset-licence-panel.png`, `library-bulk-bar.png`, `library-filter-licence-unknown.png` | track-licences |
| `collections-page.png`, `library-add-to-collection.png`, `collection-detail.png` | collections |
