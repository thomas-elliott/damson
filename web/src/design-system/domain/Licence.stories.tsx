import type { Meta, StoryObj } from "@storybook/react";

import { LicenceBar } from "./LicenceBar";
import { LicencePill } from "./LicencePill";

const meta: Meta = {
  title: "Domain/Licence",
};
export default meta;
type Story = StoryObj;

export const Pills: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <LicencePill status="ok" spdx="MIT" />
      <LicencePill status="ok" spdx="CC-BY-4.0" />
      <LicencePill status="ok" spdx="CC0-1.0" />
      <LicencePill status="warn" spdx="Unity-EULA" />
      <LicencePill status="err" spdx="CC-BY-NC-4.0" />
      <LicencePill status="unk" spdx="Unknown" />
    </div>
  ),
};

export const Bar: Story = {
  render: () => (
    <div className="flex min-w-[360px] flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <div className="text-caption">Balanced library</div>
        <LicenceBar counts={{ ok: 120, warn: 18, err: 3, unk: 42 }} />
      </div>
      <div className="flex flex-col gap-1.5">
        <div className="text-caption">Mostly compliant</div>
        <LicenceBar counts={{ ok: 248, warn: 4, err: 0, unk: 12 }} size="md" />
      </div>
      <div className="flex flex-col gap-1.5">
        <div className="text-caption">Unknown-heavy</div>
        <LicenceBar counts={{ ok: 10, warn: 2, err: 0, unk: 188 }} />
      </div>
      <div className="flex flex-col gap-1.5">
        <div className="text-caption">Empty</div>
        <LicenceBar counts={{ ok: 0, warn: 0, err: 0, unk: 0 }} />
      </div>
    </div>
  ),
};
