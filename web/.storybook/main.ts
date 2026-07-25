import type { StorybookConfig } from "@storybook/react-vite";

// Storybook 10: addon-essentials is folded into core (controls, actions,
// viewport, backgrounds, measure/outline, toolbars all ship by default).
// We explicitly load:
//   - addon-a11y        axe-core checks + fail-on-error for tests
//   - addon-vitest      run stories as Vitest tests (real Chromium)
//   - pseudo-states     toolbar toggles for :hover/:focus/:active/:disabled
const config: StorybookConfig = {
  stories: [
    "../src/design-system/**/*.mdx",
    "../src/design-system/**/*.stories.@(ts|tsx)",
    // App-level stories (wired components with their real hooks, backed by a
    // stubbed fetch) live next to the component under src/components.
    "../src/components/**/*.stories.@(ts|tsx)",
  ],
  addons: ["@storybook/addon-a11y", "@storybook/addon-vitest", "storybook-addon-pseudo-states"],
  framework: {
    name: "@storybook/react-vite",
    options: {},
  },
  typescript: {
    check: false,
    reactDocgen: "react-docgen-typescript",
  },
  core: {
    disableTelemetry: true,
  },
};

export default config;
