import React from "react";
import type { Decorator, Preview } from "@storybook/react";

import "../src/styles/globals.css";

// The theme-applying effect lives in a real component so the hook isn't called
// from the bare decorator function (react-hooks/rules-of-hooks).
function ThemeFrame({ theme, children }: { theme: "dark" | "light"; children: React.ReactNode }) {
  React.useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [theme]);
  return (
    <div
      className={`${theme === "dark" ? "dark " : ""}bg-bg text-text`}
      style={{
        padding: 24,
        minWidth: 320,
        fontFamily: "var(--ds-font-sans)",
      }}
    >
      {children}
    </div>
  );
}

const withTheme: Decorator = (Story, context) => (
  <ThemeFrame theme={context.globals.theme as "dark" | "light"}>
    <Story />
  </ThemeFrame>
);

const preview: Preview = {
  parameters: {
    backgrounds: { disable: true },
    layout: "centered",
    controls: {
      expanded: true,
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/,
      },
    },
    // Fail test runs on any detected a11y violation. Use parameter overrides
    // at the story level to soften this for specific intentional exceptions.
    a11y: {
      test: "error",
    },
  },
  globalTypes: {
    theme: {
      name: "Theme",
      description: "Damson theme (dark default, light opt-in)",
      defaultValue: "dark",
      toolbar: {
        icon: "circlehollow",
        items: [
          { value: "dark", title: "Dark", icon: "circle" },
          { value: "light", title: "Light", icon: "sun" },
        ],
        dynamicTitle: true,
      },
    },
  },
  decorators: [withTheme],
};

export default preview;
