package main

import "strings"

// purchasesSkillDirName is where `dmsn setup` installs the purchase-run
// instructions, beside the damson and scan skills.
const purchasesSkillDirName = "damson-purchases"

// purchasesSkillDocument walks an agent through cataloguing the Unity Asset
// Store purchase list. dmsn reads the list and drafts the matches; the agent's
// part is judging the unsure rows, and the operator's is approval.
func purchasesSkillDocument() string {
	return `---
name: damson-purchases
description: Catalogue the user's Unity Asset Store purchases into damson as reference assets, downloaded or not, so they can be found by search. Use when the user asks to catalogue, import or record their purchases or their Unity Asset Store assets.
---

# Catalogue Unity Asset Store purchases into damson

Each purchase becomes a damson **reference asset** linked to its Asset Store
listing, or is matched to an asset damson already records. Nothing is
downloaded. Use the damson skill for ` + "`" + binaryName + "`" + `'s output contract.

**Nothing is recorded until the user approves the plan.** Steps 1–4 only read
and write local files.

## 1. Read the purchase list

dmsn reads the list from the Unity Editor's own HTTP cache. It makes no network
call to Unity and needs no Unity sign-in, but the cache holds only what Package
Manager has loaded. Ask the user to open Package Manager in the Unity Editor,
go to **My Assets**, and choose **Load all** at the bottom of the list. Then:

    ` + binaryName + ` purchases read --out damson-purchases.tsv

It reports ` + "`found`" + ` against the list's ` + "`total`" + `. If it reports a
` + "`shortfall`" + `, tell the user how many are missing, ask them to run Load all
again, and read again. **Plan only from a complete list**, unless the user
says to go ahead with a partial one. If the cache isn't in its usual place
(Windows, Linux), pass ` + "`--cache <path to CurlRequestCache.db>`" + `.

## 2. Draft the plan

    ` + binaryName + ` purchases plan damson-purchases.tsv --out damson-purchases-plan.tsv

matches every purchase against every asset damson records, stored or
reference, and writes one row per purchase with this header:

    ` + strings.Join(purchasePlanColumns, "\t") + `

- ` + "`matched`" + `: the asset carries the same Asset Store listing ID. Leave it.
- ` + "`unsure`" + `: only the name matches. The purchase list has no publisher,
  so check what you can (the asset's name, link and publisher against the
  purchase) and change the row to ` + "`matched`" + ` or ` + "`new`" + `. Name it in
  chat either way.
- ` + "`new`" + `: nothing matches; it will be recorded as a new reference asset.

## 3. Conflicts

A row whose ` + "`asset_link`" + ` is a different link is a conflict. ` + "`matched`" + `
keeps the asset's own link; ` + "`swap`" + ` replaces it with the purchase's. Leave
it ` + "`matched`" + ` unless the user says to swap that row.

## 4. Show the plan and wait

Tell the user where the plan file is, and in chat give:

- the counts: purchases, new, matched, dropped;
- every row you were unsure of, and how you decided it;
- every conflict, with both links.

The user may drop rows (` + "`dropped`" + `), turn a match into ` + "`new`" + `, or swap a
link. Change the plan file, show the changed counts and rows again, and wait.
**Record nothing until the user explicitly approves.**

## 5. Record

    ` + binaryName + ` purchases record damson-purchases-plan.tsv

refuses a plan with any ` + "`unsure`" + ` row. It records new purchases, links
matched assets that have no link, swaps where the row says so, and marks
everything it changes Needs review. Store lookups are spaced about 2 seconds
apart, so tell the user how long a large plan will take, and run it in the
background.

## 6. Report

Tell the user what was added, linked, swapped and kept, any row that failed
and why, and the purchases linked without store data because the store lookup
failed (` + "`no_store_data`" + `). Everything a run changes is in the review queue
(` + binaryName + ` assets --needs-review). Purchases that were never downloaded are
under **Purchased, not downloaded** in the library's filters.
`
}
