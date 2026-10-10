// Ported from dialogue-db/web/scripts/check-tokens.mjs. Fix bugs there too.
//
// Fails the build on a token name this project has retired, on a raw colour
// outside tokens.css, on a typing surface below the 16px floor, or on any text
// below the 12px floor. The vocabulary is the shared contract's
// (design.smol.cloud/r/tokens.json).
// Colour values live in tokens.css; the scale literals live in globals.css's
// second @theme block, which is the only thing `shadcn add` will not regenerate.

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

/** Size, radius, motion and elevation names, so they cannot ride the
 * COLOUR_PREFIXES regex above and need their own list. */
const RETIRED_SCALE = [
  [/(?<![\w-])text-small(?![\w-])/, "text-small was retired — use text-label (12px)"],
  [/(?<![\w-])duration-std(?![\w-])/, "duration-std was retired — use duration-fast (120ms)"],
  [
    /(?<![\w-])shadow-elevated(?![\w-])/,
    "shadow-elevated was retired — use shadow-lg for overlays, shadow-md for resting surfaces",
  ],
  [/--elevation(?=[),;\s]|$)/, "--elevation was retired — use --shadow-md / --shadow-lg"],
  [/(?<![\w-])rounded-\[3px\](?![\w-])/, "rounded-[3px] was retired — use rounded-xs (4px)"],
];

/** Scoped to a preceding transition utility, not matched as a bare word: `ease`
 * is ordinary English, and a real CSS keyword in the animation shorthand that
 * globals.css still uses. Every retired call site had this shape. */
const BARE_EASE = /(?<![\w-])(?:transition|duration|delay)-[\w[\].-]+\s+ease(?![\w-])/;

/** The 16px floor: below it, iOS Safari zooms on focus and does not zoom back.
 * An allowlist rather than general typing-surface detection. The last two are
 * not installed (the field primitives are damson's), so the rule is already in
 * place if that is ever revisited. */
const TYPING_SURFACES = new Set([
  "design-system/primitives/Input.tsx",
  "design-system/primitives/Textarea.tsx",
  "design-system/primitives/SearchInput.tsx",
  "components/ui/input.tsx",
  "components/ui/textarea.tsx",
]);
const BELOW_FIELD = /(?<![\w-])text-(ui|label|small|xs|sm|base|\[[^\]]+\])(?![\w-])/;

/** The 12px floor: no text renders smaller, badges included. Catches an
 * arbitrary Tailwind size, a stylesheet font-size and an inline fontSize, in
 * px or rem (16px to the rem). An inline fontSize is a bare number, which
 * React reads as px, or a quoted string; other units are not checked. */
const TEXT_FLOOR_PX = 12;
const TEXT_SIZES = [
  /(?<![\w-])text-\[(\d*\.?\d+)(px|rem)\]/g,
  /font-size:\s*(\d*\.?\d+)(px|rem)/g,
  /fontSize:\s*(\d*\.?\d+)(?![\w.%])/g,
  /fontSize:\s*["'`](\d*\.?\d+)(px|rem)["'`]/g,
];
function belowTextFloor(line) {
  for (const re of TEXT_SIZES) {
    for (const [match, value, unit] of line.matchAll(re)) {
      const px = unit === "rem" ? Number(value) * 16 : Number(value);
      if (px < TEXT_FLOOR_PX) return match;
    }
  }
  return null;
}

for (const file of files) {
  const rel = file.slice(ROOT.length + 1);
  const isStylesheet = file.endsWith(".css") && file !== TOKENS_CSS;
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

      for (const [re, message] of RETIRED_SCALE) {
        if (re.test(line)) problems.push(`${at}: ${message}`);
      }

      if (BARE_EASE.test(line))
        problems.push(`${at}: bare \`ease\` emits nothing — use ease-standard`);

      if (TYPING_SURFACES.has(rel)) {
        const small = line.match(BELOW_FIELD);
        if (small)
          problems.push(
            `${at}: ${small[0]} on a typing surface — must be text-field (16px), or ` +
              `iOS Safari zooms on focus and does not zoom back`,
          );
      }

      // Everything above this line applies to stories and tests too: a retired
      // name is retired in a fixture as well. Only the value checks below are
      // relaxed, because a story may demonstrate a raw value as documentation.
      if (isFixture || /^\s*(\/\/|\/\*|\*)/.test(line)) return;

      const hex = isStylesheet && line.match(RAW_HEX);
      if (hex) problems.push(`${at}: hex colour ${hex[0]} — use a token from tokens.css`);

      const tiny = belowTextFloor(line);
      if (tiny)
        problems.push(`${at}: ${tiny} is below the 12px text floor — use text-label (12px)`);

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
