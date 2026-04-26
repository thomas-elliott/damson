import type { Config } from "tailwindcss";
import typography from "@tailwindcss/typography";

// Tokens live in src/design-system/tokens/tokens.css as CSS custom properties
// (OKLCH). This config maps them to Tailwind utilities so components consume
// them as class names (e.g. bg-panel, text-text-2, border-line).
// Dark is the default theme; light opts in via html[data-theme="light"].
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  // Theme switching is done by CSS custom-property overrides keyed on
  // html[data-theme="light"] — tokens swap automatically. We rarely need
  // conditional utilities, so darkMode stays on "class" (the default) and
  // `.dark` is never applied. Light-only utilities, if needed, can use the
  // arbitrary `data-[theme=light]:` variant.
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        bg: "var(--ds-bg)",
        panel: "var(--ds-panel)",
        surface: "var(--ds-surface)",
        "surface-2": "var(--ds-surface-2)",
        line: "var(--ds-line)",
        "line-2": "var(--ds-line-2)",

        text: {
          DEFAULT: "var(--ds-text)",
          2: "var(--ds-text-2)",
          3: "var(--ds-text-3)",
          4: "var(--ds-text-4)",
        },

        accent: {
          DEFAULT: "var(--ds-accent)",
          2: "var(--ds-accent-2)",
          tint: "var(--ds-accent-tint)",
          ring: "var(--ds-accent-ring)",
          fg: "var(--ds-accent-fg)",
        },

        ok: {
          DEFAULT: "var(--ds-ok)",
          tint: "var(--ds-ok-tint)",
        },
        warn: {
          DEFAULT: "var(--ds-warn)",
          tint: "var(--ds-warn-tint)",
        },
        err: {
          DEFAULT: "var(--ds-err)",
          tint: "var(--ds-err-tint)",
        },
        unk: {
          DEFAULT: "var(--ds-unk)",
          tint: "var(--ds-unk-tint)",
        },
      },

      fontFamily: {
        sans: ["var(--ds-font-sans)"],
        mono: ["var(--ds-font-mono)"],
      },

      fontSize: {
        body: ["15px", "1.5"],
        ui: ["14px", "1.4"],
        small: ["12px", "1.4"],
      },

      borderRadius: {
        sm: "var(--ds-r-sm)",
        md: "var(--ds-r-md)",
        lg: "var(--ds-r-lg)",
        pill: "var(--ds-r-pill)",
      },

      boxShadow: {
        elevated: "var(--ds-shadow)",
      },

      transitionDuration: {
        fast: "80ms",
        std: "100ms",
        slow: "120ms",
      },

      transitionTimingFunction: {
        ease: "cubic-bezier(0.4, 0, 0.2, 1)",
        bounce: "cubic-bezier(0.34, 1.56, 0.64, 1)",
      },

      zIndex: {
        resizer: "5",
        detail: "10",
        drop: "20",
        selbar: "30",
        tooltip: "100",
        dialog: "500",
        popover: "600",
        tweaks: "1000",
      },

      width: {
        sidebar: "var(--ds-sidebar-w)",
        rail: "var(--ds-rail-w)",
        detail: "var(--ds-detail-w)",
      },

      keyframes: {
        "slide-in-right": {
          from: { transform: "translateX(100%)" },
          to: { transform: "translateX(0)" },
        },
        "fade-in": {
          from: { opacity: "0" },
          to: { opacity: "1" },
        },
        "pop-up": {
          from: { transform: "translateY(8px) scale(0.98)", opacity: "0" },
          to: { transform: "translateY(0) scale(1)", opacity: "1" },
        },
      },

      animation: {
        "slide-in-right": "slide-in-right 160ms cubic-bezier(0.4, 0, 0.2, 1)",
        "fade-in": "fade-in 100ms ease",
        "pop-up": "pop-up 140ms cubic-bezier(0.34, 1.56, 0.64, 1)",
      },
    },
  },
  plugins: [typography],
} satisfies Config;
