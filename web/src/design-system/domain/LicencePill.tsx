import { forwardRef } from "react";

import { Pill, type BadgeProps } from "@/design-system/primitives";
import type { LicenceStatus } from "@/design-system/tokens";

export interface LicencePillProps extends Omit<BadgeProps, "tone" | "shape" | "mono"> {
  status: LicenceStatus;
  /** SPDX identifier or free-form label. */
  spdx: string;
}

// `warn` is a licence with terms of its own to read (a store EULA, say), not
// a problem: orange means needs review, so it shows as a plain label. Green
// and red keep their dots, and unknown keeps its grey one.
const TONE = { ok: "ok", warn: "neutral", err: "err", unk: "unk" } as const;

/**
 * Canonical licence badge — tone derived from status, always mono (SPDX
 * identifiers read cleaner as mono).
 */
export const LicencePill = forwardRef<HTMLSpanElement, LicencePillProps>(function LicencePill(
  { status, spdx, children, ...props },
  ref,
) {
  return (
    <Pill ref={ref} tone={TONE[status]} mono {...props}>
      {spdx}
      {children}
    </Pill>
  );
});
