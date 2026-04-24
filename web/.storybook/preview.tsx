import React from "react";
import type { Decorator, Preview } from "@storybook/react";

import "../src/styles/globals.css";

const withTheme: Decorator = (Story, context) => {
  const theme = context.globals.theme as "dark" | "light";
  React.useEffect(() => {
    const root = document.documentElement;
    if (theme === "light") {
      root.setAttribute("data-theme", "light");
    } else {
      root.removeAttribute("data-theme");
    }
  }, [theme]);
  return (
    <div
      className="bg-bg text-text"
      style={{
        padding: 24,
        minWidth: 320,
        fontFamily: "var(--ds-font-sans)",
      }}
    >
      <Story />
    </div>
  );
};

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
