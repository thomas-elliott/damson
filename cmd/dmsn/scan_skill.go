package main

import "strings"

// scanSkillDirName is where `dmsn setup` installs the scan instructions, beside
// the damson skill, so "scan my asset library" is enough to start a scan.
const scanSkillDirName = "damson-scan"

// scanSkillDocument walks an agent through a scan of the asset store. The
// mechanical steps are dmsn commands that read and write files; the agent's
// part is judgement — what is one pack, what is a publisher, which listing is
// a confident match — and the operator's is approval.
func scanSkillDocument() string {
	return `---
name: damson-scan
description: Catalogue an asset library on disk (an NFS store or any folder of packs) into damson as reference assets, without importing any data. Use when the user asks to scan their asset library, catalogue the NFS store, or add the packs in a folder to damson.
---

# Scan an asset library into damson

A scan records each pack on disk as a damson **reference asset**: its path,
name, folder tags, publisher, and a store link where one is a confident match.
Nothing is imported. Use the damson skill for ` + "`" + binaryName + "`" + `'s output contract.

**Nothing is recorded until the user approves the plan.** Steps 1–5 only
read and write local files.

## 1. Walk

Ask which folder to scan if the user hasn't said (for example ` + "`/mnt/nfs`" + `).
That folder is the **root**; use its absolute path, so every pack path is
absolute too. List what is under it with ` + "`find`" + ` or ` + "`ls`" + `.

One item is one **pack**, never one asset per file:

- a ` + "`.unitypackage`" + ` file;
- an archive (` + "`.zip`, `.7z`, `.rar`, `.tar`, `.tar.gz`, `.tgz`" + `);
- a folder that is one product: it holds the product's own files (models,
  textures, a readme, a ` + "`Assets/`" + ` tree) rather than more packs.

You decide which folders are one product. Once a folder is a pack, never look
inside it for more items. Note each decision you were unsure of; the user sees
them in step 5.

For each pack:

- **Publisher**: the folder holding the pack, when it names a publisher or
  studio (` + "`Synty`, `Kenney`, `Quaternius`" + `). Otherwise leave it blank.
- **Tags**: one tag per folder between the root and the pack, not counting
  the root or the publisher folder.
- **Name**: the product's name, readable: ` + "`POLYGON_Fantasy_Kingdom.unitypackage`" + `
  becomes ` + "`POLYGON Fantasy Kingdom`" + `.

Scanned from ` + "`/mnt/nfs`" + `, the pack
` + "`/mnt/nfs/3D/Environments/Synty/POLYGON_Fantasy_Kingdom.unitypackage`" + ` has
publisher ` + "`Synty`" + ` and tags ` + "`3D`" + ` and ` + "`Environments`" + `.

## 2. Known paths

    ` + binaryName + ` scan known --under <root> --json > known.json

lists every path damson already records under the root, with ` + "`present`" + `
saying whether it still exists. Sort the packs:

- **new**: found on disk, not recorded. Only these go in the plan.
- **known**: found and already recorded. Leave them alone; a rerun never
  changes their name, tags or link.
- **gone**: recorded but ` + "`present: false`" + ` (` + binaryName + ` scan known --under <root> --gone).
  Report them; never delete them. A pack that moved folder shows as one new
  pack and one gone path.

## 3. Plan file

Write the new packs to a plan file the user can open, for example
` + "`damson-scan-plan.tsv`" + `: tab-separated, this header first, one row per pack.

    ` + strings.Join(planColumns, "\t") + `

` + "`tags`" + ` is comma-separated (` + "`3D,Environments`" + `), so write a folder
whose name holds a comma with the comma left out. Leave ` + "`link`" + ` blank for now.

## 4. Store matches

Do **not** search the stores pack by pack in conversation. Run one paced
search for the whole plan instead:

    ` + binaryName + ` scan match damson-scan-plan.tsv --out damson-scan-matches.tsv

It searches the Unity Asset Store, Sketchfab and itch.io for each row with a
blank link, about 2 seconds apart (roughly 30 packs a minute, so tell the user
how long a large plan will take, and run it in the background). Each row of the
matches file is a pack's ` + "`candidate`" + ` listing, or ` + "`none`" + `, or ` + "`store_failed`" + `
(that store errored or throttled) or ` + "`search_failed`" + `.

For each pack, choose a link only when you are confident: the title names the
same product and, where the path shows a publisher, the publisher agrees.
Prefer the Unity Asset Store for a ` + "`.unitypackage`" + `. Otherwise write ` + "`no match`" + `.
Fill the ` + "`link`" + ` column with the listing URL or ` + "`no match`" + `.

## 5. Show the plan and wait

Tell the user where the plan file is, and in chat give:

- the counts: new, known, gone;
- the packs you were unsure about (folder-is-one-product calls, names,
  links you nearly chose);
- the packs whose search failed, which will be recorded without a link.

The user may drop rows, fix names, tags or links, or say a folder is (or
isn't) one product. Change the plan file, show the changed counts and rows
again, and wait. **Record nothing until the user explicitly approves.**

## 6. Record

    ` + binaryName + ` scan record damson-scan-plan.tsv

records every row as a reference asset marked Needs review, in one run. It
refuses a plan with a blank link and skips any path already recorded.

## 7. Report

Tell the user what was added, any row that failed and why, the packs recorded
without a link because their search failed, the packs linked without store data
because the store lookup failed (` + "`no_store_data`" + `), and the gone paths. Everything from
the scan is in the review queue (` + binaryName + ` assets --needs-review) until
they mark it reviewed.
`
}
