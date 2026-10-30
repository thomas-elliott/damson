import { forwardRef } from "react";

import { Pill, type BadgeProps } from "@/design-system/primitives";
import type { LicenceStatus } from "@/design-system/tokens";

export interface LicencePillProps extends Omit<BadgeProps, "tone" | "shape" | "mono"> {
  status: LicenceStatus;
  /** SPDX identifier or free-form label. */
  spdx: string;
}

/**
 * Canonical licence badge — tone derived from status, always mono (SPDX
 * identifiers read cleaner as mono).
 */
export const LicencePill = forwardRef<HTMLSpanElement, LicencePillProps>(function LicencePill(
  { status, spdx, children, ...props },
  ref,
) {
  return (
    <Pill ref={ref} tone={status} mono {...props}>
      {spdx}
      {children}
    </Pill>
  );
});
