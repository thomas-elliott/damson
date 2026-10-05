package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

const (
	skillDirName  = "damson"
	skillFileName = "SKILL.md"
	hookFileName  = "damson-session-start.sh"
)

func init() {
	var project, noHook bool
	register(&command{
		name:    "setup",
		summary: "install the Claude Code skill for this CLI",
		usage: "usage: dmsn setup [--project] [--no-hook]\n\n" +
			"Writes a skill describing dmsn to ~/.claude/skills/damson/, or to\n" +
			"./.claude/skills/damson/ with --project, the scan instructions to\n" +
			"damson-scan/ beside it, plus a SessionStart hook\n" +
			"script that prints the home view. Idempotent: rerunning overwrites\n" +
			"the same paths and prints every one it touched.\n\n" +
			"The hook script is written but not registered — registering it\n" +
			"means editing your settings.json, which is yours to edit.",
		flags: func(fs *flag.FlagSet) {
			fs.BoolVar(&project, "project", false, "install into ./.claude instead of ~/.claude")
			fs.BoolVar(&noHook, "no-hook", false, "skip the SessionStart hook script")
		},
		run: func(ctx context.Context, e *Env) error {
			return runSetup(ctx, e, project, noHook)
		},
	})
}

func runSetup(_ context.Context, e *Env, project, noHook bool) error {
	root, err := claudeRoot(project)
	if err != nil {
		return err
	}

	written := []map[string]any{}
	skillPath := filepath.Join(root, "skills", skillDirName, skillFileName)
	if err := writeFileAt(skillPath, skillDocument(), 0o644); err != nil {
		return err
	}
	written = append(written, map[string]any{"path": displayPath(skillPath), "kind": "skill"})

	scanPath := filepath.Join(root, "skills", scanSkillDirName, skillFileName)
	if err := writeFileAt(scanPath, scanSkillDocument(), 0o644); err != nil {
		return err
	}
	written = append(written, map[string]any{"path": displayPath(scanPath), "kind": "skill"})

	purchasesPath := filepath.Join(root, "skills", purchasesSkillDirName, skillFileName)
	if err := writeFileAt(purchasesPath, purchasesSkillDocument(), 0o644); err != nil {
		return err
	}
	written = append(written, map[string]any{"path": displayPath(purchasesPath), "kind": "skill"})

	if !noHook {
		hookPath := filepath.Join(root, "hooks", hookFileName)
		if err := writeFileAt(hookPath, hookScript(), 0o755); err != nil {
			return err
		}
		written = append(written, map[string]any{"path": displayPath(hookPath), "kind": "hook"})
	}

	table := axi.Table{
		Key: "written", Columns: []string{"path", "kind"},
		Rows: written, Total: -1, EmptyNote: "nothing written",
	}
	summary := Block{Key: "setup", Fields: []toon.KV{
		{Key: "scope", Value: scopeName(project)},
		{Key: "root", Value: displayPath(root)},
		{Key: "files", Value: len(written)},
	}}

	help := []string{"dmsn", "dmsn help"}
	if !noHook {
		help = append(help,
			fmt.Sprintf("add %q to hooks.SessionStart in your Claude Code settings to run it automatically",
				displayPath(filepath.Join(root, "hooks", hookFileName))))
	}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options())
}

func scopeName(project bool) string {
	if project {
		return "project"
	}
	return "user"
}

func claudeRoot(project bool) (string, error) {
	if project {
		wd, err := os.Getwd()
		if err != nil {
			return "", axi.Wrap(axi.CodeIO, err, "cannot resolve the working directory")
		}
		return filepath.Join(wd, ".claude"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "cannot resolve your home directory")
	}
	return filepath.Join(home, ".claude"), nil
}

func writeFileAt(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot create %s", filepath.Dir(path))
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot write %s", path)
	}
	return nil
}

func hookScript() string {
	return `#!/bin/sh
# Prints the damson library at a glance, for Claude Code's SessionStart hook.
# Written by ` + binaryName + ` setup. Exits 0 even when damson is unreachable,
# so a missing server never blocks a session from starting.
` + binaryName + ` --no-help 2>/dev/null || true
`
}

// skillDocument is the ambient context an agent gets: what the tool is, the
// output contract, and the handful of commands worth knowing up front.
func skillDocument() string {
	return `---
name: damson
description: Manage a damson asset library from the command line — search and inspect assets, upload files and folders, tag, collect and curate. Use whenever the user asks about their asset library, game assets, packages, or damson.
---

# damson (` + binaryName + `)

` + binaryName + ` is a CLI for a damson asset library. Run it with no arguments
to see the current library: counts, recent imports, and what needs attention.

## Output contract

Output is TOON, not JSON — a compact tabular format:

    assets[2]{id,name,publisher,needs_review}:
      511,RPG Builder,Blink,false
      512,DOTween Pro,Demigiant,true
    total: 412
    shown: 2

- ` + "`key[N]{cols}:`" + ` is a table with N rows; each following line is one row.
- ` + "`key[N]: a,b,c`" + ` is a list. ` + "`key[]: <reason>`" + ` is empty, and always says why.
- ` + "`total`" + ` is the full match count; ` + "`shown`" + ` is what this page returned.
- A ` + "`help[N]:`" + ` block suggests concrete next commands.
- Add ` + "`--json`" + ` for JSON, ` + "`--quiet`" + ` for data only, ` + "`--no-help`" + ` to drop the help block.

## Errors and exit codes

Errors print to **stdout** as a block, not to stderr:

    error:
      code: usage
      message: "..."

Exit codes: ` + "`0`" + ` success, ` + "`1`" + ` failure, ` + "`2`" + ` bad usage. Codes include
` + "`usage`, `auth`, `forbidden`, `not_found`, `conflict`, `too_large`, `no_space`, `network`, `server`" + `.
` + binaryName + ` never prompts — an ambiguous command fails with the exact
alternatives in its help block.

## Commands

    ` + binaryName + `                              the library at a glance
    ` + binaryName + ` assets [filters]             list assets
    ` + binaryName + ` asset <id>                   one asset in full
    ` + binaryName + ` tags | collections | imports browse
    ` + binaryName + ` upload <path>... [--as ...]  import files or folders
    ` + binaryName + ` sync scan|push               import the Unity Store cache
    ` + binaryName + ` asset edit|review|tag|rm     modify one asset
    ` + binaryName + ` asset record --location-path record an asset by where it lives
    ` + binaryName + ` scan known|match|record      catalogue a folder of packs (damson-scan skill)
    ` + binaryName + ` purchases read|plan|record   catalogue Unity Asset Store purchases (damson-purchases skill)
    ` + binaryName + ` bulk tag|review|rm --assets  modify many at once
    ` + binaryName + ` collection new|add|remove    curate

Every subcommand takes ` + "`--help`" + `, which is one screen. Filters on ` + "`assets`" + `
include ` + "`--q --tag --publisher --needs-review --updates-available --sort`" + `.

## Conventions worth knowing

- **Fields.** Lists show 3–4 columns by default. Widen with ` + "`--fields a,b,c`" + `;
  an unknown name exits 2 and lists the valid ones.
- **Directories are ambiguous.** ` + "`upload <dir>`" + ` requires ` + "`--as asset`" + ` (zip the
  folder into one asset) or ` + "`--as assets`" + ` (one asset per child).
- **The Unity cache syncs, not uploads.** ` + "`sync scan`" + ` lists the local Asset
  Store cache against the library; ` + "`sync push`" + ` uploads only what is missing or
  changed, and is safe to re-run. A package the library already holds by name
  but not by cache path is skipped until ` + "`--adopt-versions`" + `.
- **Reference assets hold no files.** ` + "`asset record`" + ` adds one by its path
  and/or store link (` + "`--source-url`" + `), importing nothing. Recording a known path
  updates that asset; ` + "`--needs-review`" + ` flags it for review.
- **Destructive verbs need ` + "`--yes`" + `.** ` + "`asset rm`, `tag rm`, `collection rm`, `bulk rm`" + `.
- **Mutations are idempotent** where the API allows, and report ` + "`changed`" + ` or
  ` + "`affected`" + ` so you can tell a no-op from a real change.
- **Tokens are scoped.** A read-only token fails every mutation with
  ` + "`code: forbidden`" + `. Check with ` + binaryName + ` ` + "`whoami`" + `.
- **Long text is truncated** to 400 characters with a note; ` + "`--full`" + ` disables that.

## Setup

If ` + binaryName + ` reports ` + "`connected: false`" + `, the user needs to authenticate:

    ` + binaryName + ` login --server <url> --token <token>

Tokens are created in the damson web UI under Settings → Access tokens.
Prefer ` + "`--token -`" + ` (reads from stdin) or the ` + "`DAMSON_TOKEN`" + ` environment
variable over passing a secret on the command line, where it lands in shell
history and the process list.
`
}
