/**
 * Scoped a11y exceptions for known contrast debt. Each one narrows a single
 * axe rule so it skips the elements that carry the faulty token, and nothing
 * else: the story keeps `a11y.test = "error"` for every other rule and
 * element. See "Stories and tests" in docs/design-system.md.
 */

/** axe's own selector for each rule we narrow; ours replaces it. */
const RULE_SELECTORS = {
  "color-contrast": "*",
  "link-in-text-block": "a[href], [role=link]",
} as const;

/** The debt, each with the task that tracks it. */
const DEBTS = {
  /** `faint-foreground` on `card` (4.34:1) or `muted` (3.96:1). SMO-3, audit finding 5. */
  "faint-foreground": { rule: "color-contrast", selector: ".text-faint-foreground" },
  /** `destructive` on `destructive-soft`, 4.42:1. SMO-22. */
  "destructive-soft": { rule: "color-contrast", selector: ".bg-destructive-soft" },
  /**
   * Markdown links told apart from body text by colour alone, 1.13:1. SMO-23.
   * Skips every link in the story, so use it only where Markdown is the sole
   * source of links.
   */
  "markdown-link": { rule: "link-in-text-block", selector: "a" },
} as const;

export type ContrastDebt = keyof typeof DEBTS;

type Rule = keyof typeof RULE_SELECTORS;

/** `parameters.a11y` for a story that shows the given debt. */
export function contrastDebt(...debts: ContrastDebt[]) {
  const excluded = new Map<Rule, string[]>();
  for (const debt of debts) {
    const { rule, selector } = DEBTS[debt];
    excluded.set(rule, [...(excluded.get(rule) ?? []), selector]);
  }
  return {
    config: {
      rules: [...excluded].map(([id, selectors]) => {
        const not = selectors.map((s) => `:not(${s})`).join("");
        const selector = RULE_SELECTORS[id]
          .split(", ")
          .map((part) => part + not)
          .join(", ");
        return { id, selector };
      }),
    },
  };
}
