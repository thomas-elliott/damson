// Ported from dialogue-db/web/scripts/check-tokens.mjs. Fix bugs there too.
//
// Fails the build on a token name this project has retired, or on a raw colour
// outside tokens.css. The vocabulary is the shared contract's
// (design.smol.cloud/r/tokens.json); tokens.css is the only place a value lives.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const ROOT = new URL("../src", import.meta.url).pathname;
const TOKENS_CSS = join(ROOT, "design-system", "tokens", "tokens.css");
const GLOBALS_CSS = join(ROOT, "styles", "globals.css");
const SKIP_DIRS = new Set(["node_modules", "storybook-static", "dist", ".vite", "static"]);

/** Longest name first, so a message names the specific token rather than its
 * prefix. `accent` is absent on purpose: it is a live contract name meaning the
 * hover surface, and `primary` is the brand. */
const RETIRED = [
  "surface-2",
  "surface",
  "panel",
  "bg",
  "line-2",
  "line",
  "text-2",
  "text-3",
  "text-4",
  "accent-tint",
  "accent-ring",
  "accent-fg",
  "accent-2",
  "ok-tint",
  "ok",
  "warn-tint",
  "warn",
  "err-tint",
  "err",
  "unk-tint",
  "unk",
];

const COLOUR_PREFIXES = [
  "bg",
  "text",
  "border",
  "divide",
  "ring",
  "fill",
  "stroke",
  "outline",
  "from",
  "via",
  "to",
  "shadow",
  "decoration",
  "caret",
  "placeholder",
];

/** Hex only, not rgba()/oklch(): the elevation tokens are rgba by nature. */
const RAW_HEX = /#[0-9a-fA-F]{3,8}\b/;
const ARBITRARY_COLOUR =
  /\b(?:bg|text|border|ring|fill|stroke|shadow|from|via|to|divide)-\[[^\]]*(?:#|rgb|hsl|oklch)[^\]]*\]/;

const problems = [];
const files = [];

function walk(dir) {
  for (const entry of readdirSync(dir)) {
    if (SKIP_DIRS.has(entry)) continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      walk(full);
      continue;
    }
    if (/\.(css|tsx?|jsx?)$/.test(entry)) files.push(full);
  }
}
walk(ROOT);

const names = RETIRED.join("|");
const retiredVar = new RegExp(`--(${names})(?=[),;\\s])`);
const retiredUtil = new RegExp(
  `(?<![\\w-])((?:${COLOUR_PREFIXES.join("|")})-(?:${names}))(?![\\w-])`,
);
const dsPrefix = /--ds-[a-z0-9-]+/;
const retiredRadius = /(?<![\w-])rounded-pill(?![\w-])/;

for (const file of files) {
  const rel = file.slice(ROOT.length + 1);
  const isStylesheet = file.endsWith(".css") && file !== TOKENS_CSS;
  // Stories may demonstrate a raw value as documentation, not as styling.
  const isFixture = /\.stories\.tsx$/.test(file) || /\.test\.tsx?$/.test(file);

  readFileSync(file, "utf8")
    .split("\n")
    .forEach((line, i) => {
      const at = `${rel}:${i + 1}`;

      const ds = line.match(dsPrefix);
      if (ds)
        problems.push(`${at}: ${ds[0]} — the --ds- prefix was dropped; use the contract name`);

      const dead = line.match(retiredVar);
      if (dead)
        problems.push(
          `${at}: retired token --${dead[1]} — see src/design-system/tokens/tokens.css`,
        );

      const util = line.match(retiredUtil);
      if (util)
        problems.push(`${at}: retired utility ${util[1]} — the @theme block no longer emits it`);

      const pill = line.match(retiredRadius);
      if (pill) problems.push(`${at}: rounded-pill was retired — use rounded-full`);

      if (isFixture || /^\s*(\/\/|\/\*|\*)/.test(line)) return;

      const hex = isStylesheet && line.match(RAW_HEX);
      if (hex) problems.push(`${at}: hex colour ${hex[0]} — use a token from tokens.css`);

      const arb = line.match(ARBITRARY_COLOUR);
      if (arb) problems.push(`${at}: Tailwind arbitrary colour ${arb[0]} — use a token utility`);
    });
}

// `shadcn add` appends the contract's reference :root/.dark to globals.css, which
// loads after tokens.css and is unlayered, so it wins the cascade. Renders wrong;
// never fails.
const globals = readFileSync(GLOBALS_CSS, "utf8");
if (/^:root\s*\{/m.test(globals) || /^\.dark\s*\{/m.test(globals)) {
  problems.push(
    "styles/globals.css: declares a :root or .dark block. `shadcn add` appends the " +
      "contract's reference palette here and it overrides tokens.css. Delete it.",
  );
}

if (problems.length > 0) {
  console.error(`Token guardrail: ${problems.length} problem(s)\n`);
  for (const p of problems) console.error(`  ${p}`);
  process.exit(1);
}

console.log(`Token guardrail OK: ${files.length} files clean`);
