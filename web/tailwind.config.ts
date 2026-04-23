import type { Config } from "tailwindcss";
import typography from "@tailwindcss/typography";

// Visual direction per docs/ui-design.md §2: Zed-like, monochrome-leaning,
// narrow luminance range across three surfaces, single yellow accent.
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        panel: "hsl(var(--panel))",
        elevated: "hsl(var(--elevated))",
        border: "hsl(var(--border))",
        muted: "hsl(var(--muted))",
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        licence: {
          compliant: "hsl(142, 55%, 45%)",
          review: "hsl(40, 90%, 55%)",
          violation: "hsl(0, 72%, 55%)",
          unknown: "hsl(0, 0%, 50%)",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
    },
  },
  plugins: [typography],
} satisfies Config;
